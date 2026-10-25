package ebook

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

var errNoChapters = errors.New("this EPUB has no readable chapters")

// spineDoc is one spine document, parsed and waiting to be emitted.
type spineDoc struct {
	path  string
	root  *html.Node
	fixed bool
	first bool // the first document in reading order
}

// assemble concatenates the spine documents into b.HTML and gathers their
// CSS into b.Flows. Each document keeps its own <html> element, which the
// markup parser treats as a page break.
func (e *epubReader) assemble(b *Book) error {
	docs, err := e.spineDocs()
	if err != nil {
		return err
	}
	if len(docs) == 0 {
		return errNoChapters
	}
	toc := e.readTOC()
	coverPages := map[string]bool{}
	if p := e.guideCoverPath(); p != "" {
		coverPages[p] = true
	}
	if p := e.landmarkCoverPath(); p != "" {
		coverPages[p] = true
	}
	sizes := e.pageSizes(docs)

	var out strings.Builder
	css := &cssGatherer{e: e, seen: map[string]bool{}}
	for i, d := range docs {
		css.collect(d)
		if d.fixed {
			out.WriteString(e.fixedPage(d, sizes[i]))
			continue
		}
		e.rewriteImages(d)
		if e.cover != nil && !e.paged && (d.first || coverPages[d.path]) && e.isCoverOnly(d.root) {
			// The renderer draws its own cover page from Book.Cover, so the
			// book's cover page would otherwise show the same image twice.
			continue
		}
		markHeadings(d.root, toc[d.path])
		var buf bytes.Buffer
		if err := html.Render(&buf, d.root); err != nil {
			return fmt.Errorf("%s: %w", d.path, err)
		}
		out.Write(buf.Bytes())
	}
	b.HTML = out.String()
	b.Flows = css.flows
	return nil
}

// spineDocs parses the spine's documents in reading order: linear items
// first, then those marked linear="no". A chapter that is listed but missing
// is skipped; one that cannot be read (a size limit, a damaged entry) fails
// the book, since converting without it would quietly lose text.
func (e *epubReader) spineDocs() ([]spineDoc, error) {
	var linear, extra []opfItemref
	for _, ref := range e.pkg.Spine.Itemrefs {
		if strings.EqualFold(strings.TrimSpace(ref.Linear), "no") {
			extra = append(extra, ref)
		} else {
			linear = append(linear, ref)
		}
	}
	var docs []spineDoc
	for _, ref := range append(linear, extra...) {
		it, ok := e.items[ref.IDRef]
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(it.MediaType)) {
		case "application/xhtml+xml", "text/html":
		default:
			continue
		}
		if !e.zip.has(it.path) {
			continue
		}
		root, err := e.parseDocument(it.path)
		if err != nil {
			return nil, err
		}
		docs = append(docs, spineDoc{
			path:  it.path,
			root:  root,
			fixed: e.pageIsFixed(ref),
			first: len(docs) == 0,
		})
	}
	return docs, nil
}

// selfClosing matches an XML-style empty element such as <title/> or
// <a id="x"/>.
var selfClosing = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9:_.-]*)(\s[^<>]*?)?\s*/>`)

var voidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// expandSelfClosing rewrites <tag/> as <tag></tag> for every tag that is not
// void in HTML. XHTML allows the short form for any element, but the HTML
// parser ignores the slash, so <title/> opens a title that swallows the rest
// of the document and <a id="x"/> turns the following text into a link.
func expandSelfClosing(markup []byte) []byte {
	return selfClosing.ReplaceAllFunc(markup, func(m []byte) []byte {
		sub := selfClosing.FindSubmatch(m)
		name := string(sub[1])
		if voidTags[strings.ToLower(name)] {
			return m
		}
		return []byte("<" + name + string(sub[2]) + "></" + name + ">")
	})
}

// parseDocument reads and parses one XHTML document.
func (e *epubReader) parseDocument(docPath string) (*html.Node, error) {
	raw, err := e.zip.read(docPath)
	if err != nil {
		return nil, err
	}
	// html.Parse keeps a byte order mark as body text, which would make an
	// empty cover page or a picture-only fixed page look like it has text.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	root, err := html.Parse(bytes.NewReader(expandSelfClosing(raw)))
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read: %w", docPath, err)
	}
	return root, nil
}

// walk visits n and its descendants in document order. Returning false from
// visit skips a node's children.
func walk(n *html.Node, visit func(*html.Node) bool) {
	if !visit(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
}

func findElement(n *html.Node, name string) *html.Node {
	var found *html.Node
	walk(n, func(c *html.Node) bool {
		if found != nil {
			return false
		}
		if c.Type == html.ElementNode && c.Data == name {
			found = c
			return false
		}
		return true
	})
	return found
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func removeAttr(n *html.Node, key string) {
	kept := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Namespace != "" || a.Key != key {
			kept = append(kept, a)
		}
	}
	n.Attr = kept
}

// imageRef is the file an <img> or an SVG <image> points at, or "".
func imageRef(n *html.Node) string {
	if n.Type != html.ElementNode {
		return ""
	}
	switch n.Data {
	case "img":
		return attr(n, "src")
	case "image":
		// href in SVG 2, xlink:href in SVG 1.1, which is what most EPUBs use.
		for _, a := range n.Attr {
			if a.Key == "href" {
				return a.Val
			}
		}
	}
	return ""
}

// rewriteImages points every image at its resource number, unwraps SVG
// cover images, and drops scripts and images that resolve to nothing.
func (e *epubReader) rewriteImages(d spineDoc) {
	var svgs, imgs, scripts []*html.Node
	walk(d.root, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "svg":
			svgs = append(svgs, n)
			return false
		case "img":
			imgs = append(imgs, n)
		case "script":
			scripts = append(scripts, n)
			return false
		}
		return true
	})
	for _, n := range scripts {
		n.Parent.RemoveChild(n)
	}
	// The markup parser drops <svg> entirely, so an SVG that wraps an image
	// (the usual EPUB cover page) is replaced by that image.
	for _, svg := range svgs {
		img := findElement(svg, "image")
		if img == nil {
			continue
		}
		repl := &html.Node{Type: html.ElementNode, Data: "img"}
		if idx := e.images[e.zip.resolve(d.path, imageRef(img))]; idx > 0 {
			setAttr(repl, "recindex", strconv.Itoa(idx))
			svg.Parent.InsertBefore(repl, svg)
		}
		svg.Parent.RemoveChild(svg)
	}
	for _, n := range imgs {
		idx := e.images[e.zip.resolve(d.path, attr(n, "src"))]
		if idx == 0 {
			// Remote, missing or SVG: nothing the renderer could draw.
			n.Parent.RemoveChild(n)
			continue
		}
		removeAttr(n, "src")
		setAttr(n, "recindex", strconv.Itoa(idx))
	}
}

// body returns the document's <body>, which html.Parse always creates.
func body(root *html.Node) *html.Node {
	if b := findElement(root, "body"); b != nil {
		return b
	}
	return root
}

// hasText reports whether n holds any visible text.
func hasText(n *html.Node) bool {
	found := false
	walk(n, func(c *html.Node) bool {
		if found {
			return false
		}
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
			return false
		}
		if c.Type == html.TextNode && strings.IndexFunc(c.Data, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0 {
			found = true
		}
		return true
	})
	return found
}

// isCoverOnly reports whether a rewritten document is nothing but the cover
// image: no text and exactly one image, the cover's.
func (e *epubReader) isCoverOnly(root *html.Node) bool {
	b := body(root)
	if hasText(b) {
		return false
	}
	var refs []string
	walk(b, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "img" {
			refs = append(refs, attr(n, "recindex"))
		}
		return true
	})
	return len(refs) == 1 && refs[0] == strconv.Itoa(e.cover.Index)
}

// cssGatherer collects each document's stylesheets into Flows, reading every
// linked file once.
type cssGatherer struct {
	e     *epubReader
	seen  map[string]bool
	flows []string
}

// cssImport matches an @import statement. The book's other sheets are all
// in the manifest anyway, and left in place an @import would swallow the
// rule after it, since the stylesheet parser reads up to the next brace.
var cssImport = regexp.MustCompile(`@import[^;{}]*;`)

func (g *cssGatherer) collect(d spineDoc) {
	walk(d.root, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "link":
			if !hasProperty(strings.ToLower(attr(n, "rel")), "stylesheet") {
				return true
			}
			target := g.e.zip.resolve(d.path, attr(n, "href"))
			if g.seen[target] || !g.e.zip.has(target) {
				return true
			}
			g.seen[target] = true
			if data, err := g.e.zip.read(target); err == nil {
				g.add(string(data))
			}
		case "style":
			var sb strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					sb.WriteString(c.Data)
				}
			}
			g.add(sb.String())
			return false
		}
		return true
	})
}

func (g *cssGatherer) add(css string) {
	css = strings.TrimSpace(cssImport.ReplaceAllString(css, ""))
	if css != "" {
		g.flows = append(g.flows, css)
	}
}

// headingElements are the elements a table-of-contents target can mark as a
// heading directly.
var headingElements = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "div": true, "section": true, "header": true, "article": true,
	"blockquote": true, "li": true, "figure": true,
}

// markHeadings sets data-ebook-heading on each table-of-contents target in
// the document, so it becomes a heading and a bookmark whatever the book's
// CSS says. The element marked is chosen by headingFor; a target that is
// missing, or inline with no block around it below <body>, marks the first
// block of text in the document instead.
func markHeadings(root *html.Node, targets []tocTarget) {
	if len(targets) == 0 {
		return
	}
	ids := map[string]*html.Node{}
	walk(root, func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			if id := attr(n, "id"); id != "" {
				if _, dup := ids[id]; !dup {
					ids[id] = n
				}
			}
		}
		return true
	})
	depths := map[*html.Node]int{}
	var order []*html.Node
	mark := func(n *html.Node, depth int) {
		if n == nil {
			return
		}
		depth = min(max(depth, 1), 6)
		if d, ok := depths[n]; !ok {
			order = append(order, n)
			depths[n] = depth
		} else if depth < d {
			depths[n] = depth
		}
	}
	b := body(root)
	for _, t := range targets {
		if n := headingFor(ids[t.Fragment], b); t.Fragment != "" && n != nil {
			mark(n, t.Depth)
			continue
		}
		mark(firstTextBlock(b), t.Depth)
	}
	for _, n := range order {
		setAttr(n, "data-ebook-heading", strconv.Itoa(depths[n]))
	}
}

// containerElements hold other blocks. A heading set on one is flushed empty
// by the markup parser as soon as its first inner block opens.
var containerElements = map[string]bool{
	"div": true, "section": true, "article": true, "header": true,
	"blockquote": true, "li": true, "figure": true,
}

// headingFor is the element a table-of-contents target should mark: the
// target itself when it is a plain block, the first block of text inside it
// when it wraps a chapter, or the block around it when it is inline (an
// empty <a id> anchor at the start of a title). It returns nil when the
// target is missing or sits in no block below body.
func headingFor(n, body *html.Node) *html.Node {
	for n != nil && n != body && !headingElements[n.Data] {
		n = n.Parent
	}
	if n == nil || n == body {
		return nil
	}
	if containerElements[n.Data] {
		if inner := firstTextBlock(n); inner != nil {
			return inner
		}
	}
	return n
}

// firstTextBlock is the innermost heading-capable block around the first
// visible text in n, so a chapter wrapped in one big <div> marks its title
// rather than the whole chapter.
func firstTextBlock(n *html.Node) *html.Node {
	var text *html.Node
	walk(n, func(c *html.Node) bool {
		if text != nil {
			return false
		}
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "style") {
			return false
		}
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) != "" {
			text = c
		}
		return true
	})
	for p := text; p != nil && p != n; p = p.Parent {
		if p.Type == html.ElementNode && headingElements[p.Data] {
			return p
		}
	}
	return nil
}
