package htmldoc

import (
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// nbsp is a non-breaking space: content rather than collapsible whitespace.
const nbsp = ' '

// Parse converts book markup into a block sequence.
func Parse(markup string) *Doc {
	return ParseWithCSS(markup, nil)
}

// ParseWithCSS converts book markup into a block sequence, consulting the
// book's own stylesheet for alignment, relative sizes and page breaks. Kindle
// books lean heavily on CSS classes, so this is what makes headings look like
// headings and recipe lists keep their hanging indents.
func ParseWithCSS(markup string, sheet *Stylesheet) *Doc {
	p := &parser{doc: &Doc{}, css: sheet}
	p.run(markup)
	return p.doc
}

type parser struct {
	doc *Doc
	css *Stylesheet

	cur     Block
	curOpen bool

	stack []frame
	skip  int // nesting depth inside an element whose content is dropped

	style Style
	link  string

	// pendingSpace records that an inline element's padding stands in for a
	// space, which is how list numbers and run-in labels are separated from
	// the text that follows them.
	pendingSpace bool
	seenHTML     bool
}

// frame records what an open element contributed, so the end tag can undo it.
type frame struct {
	name       string
	style      Style
	link       bool
	block      bool
	skip       bool
	breakEnd   bool
	spaceAfter bool
}

func (p *parser) run(markup string) {
	z := html.NewTokenizer(strings.NewReader(markup))
	for {
		switch z.Next() {
		case html.ErrorToken:
			p.flush()
			return
		case html.TextToken:
			if p.skip == 0 {
				p.text(string(z.Text()))
			}
		case html.StartTagToken:
			p.start(z.Token())
		case html.SelfClosingTagToken:
			t := z.Token()
			p.start(t)
			p.end(t.Data)
		case html.EndTagToken:
			name, _ := z.TagName()
			p.end(string(name))
		}
	}
}

// droppedElements never contribute text to the page.
var droppedElements = map[string]bool{
	"head": true, "style": true, "script": true, "title": true,
	"svg": true, "link": true, "meta": true, "guide": true,
}

func (p *parser) start(t html.Token) {
	name := strings.ToLower(t.Data)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		// Kindle markup carries namespaced tags such as <mbp:pagebreak/>.
		if name[:i] == "mbp" && name[i+1:] == "pagebreak" {
			p.emitBreak()
			return
		}
		name = name[i+1:]
	}

	if p.skip > 0 {
		if !isVoid(name) {
			p.push(frame{name: name, skip: true})
			p.skip++
		}
		return
	}
	if droppedElements[name] {
		if isVoid(name) {
			return
		}
		p.push(frame{name: name, skip: true})
		p.skip = 1
		return
	}

	attrs := attrMap(t)
	props := p.css.Lookup(name, attrs["class"])

	switch name {
	case "html":
		// Each KF8 part starts a fresh document, which is the book's own idea
		// of where a new page begins.
		if p.seenHTML {
			p.emitBreak()
		}
		p.seenHTML = true
		return
	case "br":
		p.appendSpan(Span{Text: LineBreak})
		return
	case "hr":
		p.flush()
		p.doc.Blocks = append(p.doc.Blocks, Block{Kind: KindRule})
		return
	case "img", "image":
		p.image(attrs)
		return
	}

	f := frame{name: name, breakEnd: props.BreakEnd}

	if props.BreakStart {
		p.emitBreak()
	}

	kind, level, isBlock := blockKind(name, attrs)
	if !isBlock && props.Block {
		// A span the stylesheet promotes to a block, which book designers use
		// constantly for run-in chapter numbers and subtitles.
		kind, level, isBlock = KindParagraph, 0, true
	}
	if isBlock {
		p.flush()
		align := alignOf(attrs)
		if align == AlignDefault {
			align = props.Align
		}
		p.cur = Block{
			Kind:       kind,
			Level:      level,
			Align:      align,
			Scale:      props.Scale,
			MarginLeft: props.MarginLeft,
			Indent:     props.Indent,
		}
		p.curOpen = true
		f.block = true
		if kind == KindListItem {
			p.cur.Level = p.listDepth()
		}
	}

	if !isBlock {
		// Horizontal padding on an inline element reads as a gap.
		if props.MarginLeft > 0 {
			p.pendingSpace = true
		}
		f.spaceAfter = props.PadRight > 0
	}

	if s := inlineStyle(name, attrs) | props.style(); s != 0 {
		f.style = s &^ p.style // only what this element actually adds
		p.style |= s
	}
	if name == "a" {
		if href := attrs["href"]; href != "" && !isInternalLink(href) {
			f.link = true
			p.link = href
		}
	}

	if !isVoid(name) {
		p.push(f)
	}
}

func (p *parser) end(name string) {
	name = strings.ToLower(name)
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}

	idx := -1
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i].name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		// A stray close tag: KF8 fragments are full of them.
		return
	}

	// Unwind everything that badly nested markup left open.
	for i := len(p.stack) - 1; i >= idx; i-- {
		f := p.stack[i]
		if f.skip {
			if p.skip > 0 {
				p.skip--
			}
			continue
		}
		p.style &^= f.style
		if f.link {
			p.link = ""
		}
		if f.block {
			p.flush()
		}
		if f.spaceAfter {
			p.pendingSpace = true
		}
		if f.breakEnd {
			p.emitBreak()
		}
	}
	p.stack = p.stack[:idx]
}

func (p *parser) push(f frame) { p.stack = append(p.stack, f) }

func (p *parser) listDepth() int {
	depth := 0
	for _, f := range p.stack {
		if f.name == "ul" || f.name == "ol" {
			depth++
		}
	}
	if depth == 0 {
		depth = 1
	}
	return depth
}

func (p *parser) text(s string) {
	if s == "" {
		return
	}
	s = collapse(s)
	if s == "" {
		return
	}
	if !p.curOpen && strings.TrimSpace(s) == "" {
		return
	}
	p.appendSpan(Span{Text: s, Style: p.style, Link: p.link})
}

func (p *parser) appendSpan(sp Span) {
	if !p.curOpen {
		p.cur = Block{Kind: KindParagraph}
		p.curOpen = true
	}
	// Never start a block with whitespace, and never double it up.
	if sp.Text != LineBreak {
		if p.pendingSpace {
			p.pendingSpace = false
			if len(p.cur.Spans) > 0 && !strings.HasPrefix(sp.Text, " ") &&
				!strings.HasSuffix(lastText(p.cur.Spans), " ") {
				sp.Text = " " + sp.Text
			}
		}
		if len(p.cur.Spans) == 0 || strings.HasSuffix(lastText(p.cur.Spans), " ") {
			sp.Text = strings.TrimLeft(sp.Text, " ")
			if sp.Text == "" {
				return
			}
		}
	}
	// Merge with the previous span when the style matches.
	if n := len(p.cur.Spans); n > 0 && sp.Text != LineBreak {
		prev := &p.cur.Spans[n-1]
		if prev.Style == sp.Style && prev.Link == sp.Link && prev.Text != LineBreak {
			prev.Text += sp.Text
			return
		}
	}
	p.cur.Spans = append(p.cur.Spans, sp)
}

func (p *parser) flush() {
	if !p.curOpen {
		return
	}
	b := p.cur
	p.cur = Block{}
	p.curOpen = false
	p.pendingSpace = false

	// Trim trailing whitespace and stray breaks.
	for len(b.Spans) > 0 {
		last := b.Spans[len(b.Spans)-1]
		if last.Text == LineBreak || strings.TrimSpace(last.Text) == "" {
			b.Spans = b.Spans[:len(b.Spans)-1]
			continue
		}
		b.Spans[len(b.Spans)-1].Text = strings.TrimRight(last.Text, " ")
		break
	}
	if b.IsEmpty() {
		return
	}
	p.doc.Blocks = append(p.doc.Blocks, b)
}

func (p *parser) emitBreak() {
	p.flush()
	if n := len(p.doc.Blocks); n > 0 && p.doc.Blocks[n-1].Kind == KindPageBreak {
		return
	}
	p.doc.Blocks = append(p.doc.Blocks, Block{Kind: KindPageBreak})
}

func (p *parser) image(attrs map[string]string) {
	idx := resourceIndex(attrs)
	if idx <= 0 {
		return
	}
	p.flush()
	p.doc.Blocks = append(p.doc.Blocks, Block{
		Kind:     KindImage,
		Resource: idx,
		Alt:      attrs["alt"],
		Align:    AlignCenter,
	})
}

// resourceIndex resolves an <img> to a 1-based resource number. KF8 uses
// kindle:embed:XXXX with a base-32 index; MOBI 6 uses recindex="00001".
func resourceIndex(attrs map[string]string) int {
	if v := attrs["recindex"]; v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	src := attrs["src"]
	if src == "" {
		return 0
	}
	if i := strings.Index(src, "kindle:embed:"); i >= 0 {
		rest := src[i+len("kindle:embed:"):]
		if j := strings.IndexAny(rest, "?&#"); j >= 0 {
			rest = rest[:j]
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(rest), 32, 64); err == nil {
			return int(n)
		}
	}
	return 0
}

var headingClass = regexp.MustCompile(`(?i)(^|\s)h([1-6])(\b|[-_])`)

// blockKind decides whether a tag opens a new block, and of what sort.
func blockKind(name string, attrs map[string]string) (Kind, int, bool) {
	switch name {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return KindHeading, int(name[1] - '0'), true
	case "li", "dt", "dd":
		return KindListItem, 1, true
	case "blockquote":
		return KindQuote, 0, true
	case "p", "div", "section", "article", "center", "tr", "td", "th",
		"table", "ul", "ol", "dl", "pre", "figure", "figcaption", "aside", "header", "footer":
		// Kindle books usually style their headings with a class rather than a
		// heading tag, so trust the class when it looks like one.
		if m := headingClass.FindStringSubmatch(attrs["class"]); m != nil {
			return KindHeading, int(m[2][0] - '0'), true
		}
		return KindParagraph, 0, true
	}
	return KindParagraph, 0, false
}

func inlineStyle(name string, attrs map[string]string) Style {
	var s Style
	switch name {
	case "b", "strong":
		s |= StyleBold
	case "i", "em", "cite", "var", "dfn":
		s |= StyleItalic
	case "u", "ins":
		s |= StyleUnderline
	case "code", "tt", "kbd", "samp", "pre":
		s |= StyleMono
	case "small":
		s |= StyleSmall
	case "sup":
		s |= StyleSuper
	case "sub":
		s |= StyleSub
	}
	css := strings.ToLower(strings.ReplaceAll(attrs["style"], " ", ""))
	if strings.Contains(css, "font-weight:bold") {
		s |= StyleBold
	}
	if strings.Contains(css, "font-style:italic") {
		s |= StyleItalic
	}
	return s
}

func alignOf(attrs map[string]string) Align {
	v := strings.ToLower(attrs["align"] + " " + strings.ReplaceAll(attrs["style"], " ", "") + " " + attrs["class"])
	switch {
	case strings.Contains(v, "center"):
		return AlignCenter
	case strings.Contains(v, "text-align:right"):
		return AlignRight
	}
	return AlignDefault
}

func isInternalLink(href string) bool {
	return strings.HasPrefix(href, "#") || strings.HasPrefix(href, "kindle:")
}

func isVoid(name string) bool {
	switch name {
	case "br", "hr", "img", "image", "meta", "link", "input", "area", "base",
		"col", "embed", "param", "source", "track", "wbr":
		return true
	}
	return false
}

func attrMap(t html.Token) map[string]string {
	m := make(map[string]string, len(t.Attr))
	for _, a := range t.Attr {
		m[strings.ToLower(a.Key)] = a.Val
	}
	return m
}

func lastText(spans []Span) string {
	if len(spans) == 0 {
		return ""
	}
	return spans[len(spans)-1].Text
}

// collapse squeezes runs of whitespace into single spaces, the way HTML does.
func collapse(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			space = true
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			if r == nbsp {
				r = ' '
			}
			b.WriteRune(r)
		}
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}
