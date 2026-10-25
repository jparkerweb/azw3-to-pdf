package ebook

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

// fb2Node is one element of a FictionBook document. Children holds either
// elements or, when Text is set on a child with no Name, character data.
type fb2Node struct {
	Name     string
	Attrs    []xml.Attr
	Text     string
	Children []*fb2Node
}

// attr finds an attribute by local name, ignoring its namespace: FictionBook
// links are written l:href, xlink:href or with whatever prefix the author's
// tool picked.
func (n *fb2Node) attr(local string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func (n *fb2Node) child(name string) *fb2Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// text is the node's character data with whitespace collapsed.
func (n *fb2Node) text() string {
	var words []string
	var walk func(*fb2Node)
	walk = func(m *fb2Node) {
		if m.Name == "" {
			words = append(words, strings.Fields(m.Text)...)
			return
		}
		for _, c := range m.Children {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(words, " ")
}

var xmlEncodingDecl = regexp.MustCompile(`^<\?xml[^>]*\bencoding\s*=\s*["']([^"']+)["']`)

// decodeFB2 reads a FictionBook 2 file: one XML document holding the
// metadata, one or more bodies and the images as base64 binaries.
func decodeFB2(data []byte) (*Book, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	encoding := "UTF-8"
	if !xmlEncodingDecl.Match(bytes.TrimLeft(head(data), " \t\r\n")) && !utf8.Valid(data) {
		// Undeclared and not UTF-8: in practice that is an old Russian book,
		// which is where most FictionBook files come from.
		conv, err := charmap.Windows1251.NewDecoder().Bytes(data)
		if err != nil {
			return nil, fmt.Errorf("this FictionBook file is damaged: %v", err)
		}
		data = conv
		encoding = "windows-1251"
	}

	d := xml.NewDecoder(bytes.NewReader(data))
	// Only called for a declared encoding other than UTF-8.
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, fmt.Errorf("the encoding %q is not supported", label)
		}
		if name, err := htmlindex.Name(enc); err == nil {
			encoding = name
		} else {
			encoding = label
		}
		return enc.NewDecoder().Reader(input), nil
	}

	root, binaries, err := readFB2Tree(d)
	if err != nil {
		return nil, fmt.Errorf("this FictionBook file is damaged: %v", err)
	}

	b := &Book{Format: "FictionBook 2", Encoding: encoding, Resources: map[int]*Resource{}}
	coverID := readFB2Description(b, root.child("description"))

	ids := map[string]int{}
	for _, bin := range binaries {
		raw, err := base64.StdEncoding.DecodeString(stripSpace(bin.data))
		if err != nil || sniffImage(raw) == "" {
			continue
		}
		idx := len(b.Resources) + 1
		b.Resources[idx] = &Resource{Index: idx, Kind: sniffImage(raw), Data: raw}
		if _, dup := ids[bin.id]; !dup {
			ids[bin.id] = idx
		}
	}
	if idx, ok := ids[coverID]; ok && coverID != "" {
		b.Cover = b.Resources[idx]
	}

	w := &fb2Writer{ids: ids, coverID: coverID, firstImage: true}
	first := true
	for _, body := range root.Children {
		if body.Name != "body" {
			continue
		}
		switch {
		case first:
			w.body(body, false)
			first = false
		case body.attr("name") == "notes":
			w.out.WriteString("<mbp:pagebreak/><h1>Notes</h1>")
			w.body(body, true)
		default:
			w.body(body, false)
		}
	}
	b.HTML = w.out.String()
	b.TextBytes = len(b.HTML)
	return b, nil
}

type fb2Binary struct {
	id   string
	data string
}

// readFB2Tree streams the document into a tree, setting the binaries aside
// as they arrive, since they make up most of the file and are not markup.
func readFB2Tree(d *xml.Decoder) (*fb2Node, []fb2Binary, error) {
	var root *fb2Node
	var stack []*fb2Node
	var binaries []fb2Binary
	var bin *fb2Binary
	var binData strings.Builder
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) == 1 && t.Name.Local == "binary" {
				id := ""
				for _, a := range t.Attr {
					if a.Name.Local == "id" {
						id = a.Value
					}
				}
				bin = &fb2Binary{id: id}
				binData.Reset()
			}
			n := &fb2Node{Name: t.Name.Local, Attrs: t.Attr}
			if len(stack) == 0 {
				if root != nil {
					return nil, nil, errors.New("the document has more than one root element")
				}
				root = n
			} else if bin == nil {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			if bin != nil && len(stack) == 1 {
				bin.data = binData.String()
				binaries = append(binaries, *bin)
				bin = nil
			}
		case xml.CharData:
			switch {
			case bin != nil:
				binData.Write(t)
			case len(stack) > 0:
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, &fb2Node{Text: string(t)})
			}
		}
	}
	if root == nil {
		return nil, nil, errors.New("the document is empty")
	}
	return root, binaries, nil
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// readFB2Description fills in the metadata and returns the cover's binary id.
func readFB2Description(b *Book, desc *fb2Node) string {
	if desc == nil {
		return ""
	}
	coverID := ""
	if ti := desc.child("title-info"); ti != nil {
		for _, c := range ti.Children {
			switch c.Name {
			case "book-title":
				b.Title = c.text()
			case "author":
				if name := fb2AuthorName(c); name != "" {
					b.Authors = append(b.Authors, name)
				}
			case "annotation":
				b.Description = c.text()
			case "date":
				b.Published = c.text()
				if b.Published == "" {
					b.Published = strings.TrimSpace(c.attr("value"))
				}
			case "lang":
				b.Language = c.text()
			case "genre":
				if g := c.text(); g != "" {
					b.Subjects = append(b.Subjects, g)
				}
			case "coverpage":
				if img := c.child("image"); img != nil && coverID == "" {
					coverID = strings.TrimPrefix(img.attr("href"), "#")
				}
			}
		}
	}
	if pi := desc.child("publish-info"); pi != nil {
		if p := pi.child("publisher"); p != nil {
			b.Publisher = p.text()
		}
		if i := pi.child("isbn"); i != nil {
			b.ISBN = i.text()
		}
		if y := pi.child("year"); y != nil && b.Published == "" {
			b.Published = y.text()
		}
	}
	return coverID
}

func fb2AuthorName(a *fb2Node) string {
	var parts []string
	for _, field := range []string{"first-name", "middle-name", "last-name"} {
		if c := a.child(field); c != nil {
			if s := c.text(); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if len(parts) == 0 {
		if c := a.child("nickname"); c != nil {
			return c.text()
		}
	}
	return strings.Join(parts, " ")
}

// fb2Writer turns FictionBook body markup into the HTML the layout parser
// reads.
type fb2Writer struct {
	out        strings.Builder
	ids        map[string]int
	coverID    string
	firstImage bool
}

// body writes one <body>. In the notes body, the notes sit under the Notes
// heading the caller wrote, so they are one level deeper, its own title is
// dropped in favour of that heading, and they run on without a page each.
func (w *fb2Writer) body(n *fb2Node, notes bool) {
	depth := 0
	if notes {
		depth = 1
	}
	for _, c := range n.Children {
		if notes && c.Name == "title" {
			continue
		}
		if c.Name == "section" && !notes {
			w.out.WriteString("<mbp:pagebreak/>")
		}
		w.block(c, depth)
	}
}

// block writes a block-level element found at the given section depth.
func (w *fb2Writer) block(n *fb2Node, depth int) {
	switch n.Name {
	case "":
		if strings.TrimSpace(n.Text) != "" {
			w.out.WriteString(html.EscapeString(n.Text))
		}
	case "section":
		for _, c := range n.Children {
			w.block(c, depth+1)
		}
	case "title":
		level := strconv.Itoa(min(max(depth, 1), 6))
		w.out.WriteString("<h" + level + ">")
		w.joinedLines(n)
		w.out.WriteString("</h" + level + ">")
	case "p", "text-author", "date":
		w.out.WriteString("<p>")
		w.inlineChildren(n)
		w.out.WriteString("</p>")
	case "subtitle":
		w.out.WriteString("<p><strong>")
		w.inlineChildren(n)
		w.out.WriteString("</strong></p>")
	case "empty-line":
		w.out.WriteString("<br>")
	case "epigraph", "cite":
		w.out.WriteString("<blockquote>")
		for _, c := range n.Children {
			w.block(c, depth)
		}
		w.out.WriteString("</blockquote>")
	case "poem":
		w.out.WriteString("<blockquote>")
		for _, c := range n.Children {
			w.poemPart(c, depth)
		}
		w.out.WriteString("</blockquote>")
	case "stanza":
		w.out.WriteString("<blockquote>")
		w.poemPart(n, depth)
		w.out.WriteString("</blockquote>")
	case "table":
		for _, tr := range n.Children {
			if tr.Name != "tr" {
				continue
			}
			w.out.WriteString("<p>")
			cells := 0
			for _, td := range tr.Children {
				if td.Name != "td" && td.Name != "th" {
					continue
				}
				if cells > 0 {
					w.out.WriteString(" ")
				}
				w.inlineChildren(td)
				cells++
			}
			w.out.WriteString("</p>")
		}
	case "image":
		w.image(n)
	default:
		// Unknown containers are transparent, so their text is never lost.
		for _, c := range n.Children {
			w.block(c, depth)
		}
	}
}

// poemPart writes one child of a poem. A stanza is one paragraph whose verse
// lines are separated by line breaks; a poem's title is not a section title,
// so it stays out of the heading outline.
func (w *fb2Writer) poemPart(n *fb2Node, depth int) {
	switch n.Name {
	case "stanza":
		w.out.WriteString("<p>")
		lines := 0
		for _, c := range n.Children {
			switch c.Name {
			case "v":
				if lines > 0 {
					w.out.WriteString("<br>")
				}
				w.inlineChildren(c)
				lines++
			case "title", "subtitle":
				w.out.WriteString("<strong>")
				w.joinedLines(c)
				w.out.WriteString("</strong><br>")
			}
		}
		w.out.WriteString("</p>")
	case "title":
		w.out.WriteString("<p><strong>")
		w.joinedLines(n)
		w.out.WriteString("</strong></p>")
	default:
		w.block(n, depth)
	}
}

// joinedLines writes a title's paragraphs as lines of one block.
func (w *fb2Writer) joinedLines(n *fb2Node) {
	lines := 0
	for _, c := range n.Children {
		switch c.Name {
		case "p":
			if lines > 0 {
				w.out.WriteString("<br>")
			}
			w.inlineChildren(c)
			lines++
		case "empty-line":
			w.out.WriteString("<br>")
		case "":
			if strings.TrimSpace(c.Text) != "" {
				w.out.WriteString(html.EscapeString(c.Text))
			}
		}
	}
}

var fb2InlineTags = map[string]string{
	"emphasis":      "em",
	"strong":        "strong",
	"strikethrough": "s",
	"sup":           "sup",
	"sub":           "sub",
	"code":          "code",
}

func (w *fb2Writer) inlineChildren(n *fb2Node) {
	for _, c := range n.Children {
		w.inline(c)
	}
}

func (w *fb2Writer) inline(n *fb2Node) {
	if n.Name == "" {
		w.out.WriteString(html.EscapeString(n.Text))
		return
	}
	if tag, ok := fb2InlineTags[n.Name]; ok {
		w.out.WriteString("<" + tag + ">")
		w.inlineChildren(n)
		w.out.WriteString("</" + tag + ">")
		return
	}
	switch n.Name {
	case "a":
		href := n.attr("href")
		if href == "" || strings.HasPrefix(href, "#") {
			w.inlineChildren(n)
			return
		}
		w.out.WriteString(`<a href="` + html.EscapeString(href) + `">`)
		w.inlineChildren(n)
		w.out.WriteString("</a>")
	case "image":
		w.image(n)
	default:
		w.inlineChildren(n)
	}
}

// image writes an <image> as a numbered <img>. The first image of the book is
// very often the cover again, which is already shown on its own page.
func (w *fb2Writer) image(n *fb2Node) {
	id := strings.TrimPrefix(n.attr("href"), "#")
	first := w.firstImage
	w.firstImage = false
	idx, ok := w.ids[id]
	if !ok {
		return
	}
	if first && id == w.coverID {
		return
	}
	w.out.WriteString(`<img recindex="` + strconv.Itoa(idx) + `">`)
}

// decodeFB2Zip reads a FictionBook file distributed inside a zip archive,
// which is how most libraries hand them out.
func decodeFB2Zip(data []byte) (*Book, error) {
	a, err := openZip(data)
	if err != nil {
		return nil, err
	}
	for _, name := range a.names() {
		if !strings.HasSuffix(strings.ToLower(name), ".fb2") {
			continue
		}
		inner, err := a.read(name)
		if err != nil {
			return nil, err
		}
		return decodeFB2(inner)
	}
	return nil, errors.New("this zip archive does not contain a .fb2 book")
}
