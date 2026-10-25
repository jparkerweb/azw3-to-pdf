package ebook

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// The OPF package file, reduced to what a conversion uses. Field tags carry
// no namespace, so encoding/xml matches on the local name and books that
// write <dc:title>, <opf:item> or plain <item> all parse the same way.
type opfPackage struct {
	Version  string         `xml:"version,attr"`
	Metadata opfMetadata    `xml:"metadata"`
	Manifest []opfItem      `xml:"manifest>item"`
	Spine    opfSpine       `xml:"spine"`
	Guide    []opfReference `xml:"guide>reference"`
}

type opfMetadata struct {
	Titles       []string        `xml:"title"`
	Creators     []string        `xml:"creator"`
	Publishers   []string        `xml:"publisher"`
	Descriptions []string        `xml:"description"`
	Identifiers  []opfIdentifier `xml:"identifier"`
	Dates        []string        `xml:"date"`
	Rights       []string        `xml:"rights"`
	Languages    []string        `xml:"language"`
	Subjects     []string        `xml:"subject"`
	Metas        []opfMeta       `xml:"meta"`
}

type opfIdentifier struct {
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}

// opfMeta is both forms of <meta>: EPUB 2's name/content pair and EPUB 3's
// property with the value as text.
type opfMeta struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Property string `xml:"property,attr"`
	Value    string `xml:",chardata"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`

	path string // Href resolved to an archive name
}

type opfSpine struct {
	Toc      string       `xml:"toc,attr"`
	Itemrefs []opfItemref `xml:"itemref"`
}

type opfItemref struct {
	IDRef      string `xml:"idref,attr"`
	Linear     string `xml:"linear,attr"`
	Properties string `xml:"properties,attr"`
}

type opfReference struct {
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type epubContainer struct {
	Rootfiles []struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

// epubReader holds one EPUB while it is turned into a Book.
type epubReader struct {
	zip     *zipArchive
	opfPath string
	pkg     opfPackage
	items   map[string]*opfItem // by manifest id

	images    map[string]int // archive name -> resource index
	resources map[int]*Resource
	cover     *Resource

	paged bool
}

var errNoPackage = errors.New("this EPUB has no package file (META-INF/container.xml is missing or empty)")

// decodeEPUB reads a reflowable or fixed-layout EPUB 2 or 3.
func decodeEPUB(data []byte) (*Book, error) {
	a, err := openZip(data)
	if err != nil {
		return nil, err
	}
	if err := checkEPUBDRM(a); err != nil {
		return nil, err
	}
	e, err := readEPUBPackage(a)
	if err != nil {
		return nil, err
	}
	if err := e.numberImages(); err != nil {
		return nil, err
	}
	e.cover = e.findCover()
	e.paged = e.prePaginated()

	b := e.metadata()
	b.Resources = e.resources
	b.Cover = e.cover
	b.Paged = e.paged
	if err := e.assemble(b); err != nil {
		return nil, err
	}
	b.TextBytes = len(b.HTML)
	return b, nil
}

// readEPUBPackage finds the package file through container.xml and parses it.
func readEPUBPackage(a *zipArchive) (*epubReader, error) {
	if !a.has("META-INF/container.xml") {
		return nil, errNoPackage
	}
	raw, err := a.read("META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	var c epubContainer
	if err := decodeXML(raw, &c); err != nil {
		return nil, fmt.Errorf("META-INF/container.xml is damaged: %v", err)
	}
	opfPath := ""
	for _, r := range c.Rootfiles {
		if r.MediaType == "application/oebps-package+xml" && r.FullPath != "" {
			opfPath = r.FullPath
			break
		}
	}
	if opfPath == "" {
		for _, r := range c.Rootfiles {
			if r.FullPath != "" {
				opfPath = r.FullPath
				break
			}
		}
	}
	if opfPath == "" {
		return nil, errNoPackage
	}
	opfPath = cleanZipName(opfPath)
	raw, err = a.read(opfPath)
	if err != nil {
		return nil, fmt.Errorf("reading the package file: %w", err)
	}
	e := &epubReader{zip: a, opfPath: opfPath, items: map[string]*opfItem{}}
	if err := decodeXML(raw, &e.pkg); err != nil {
		return nil, fmt.Errorf("the package file %s is damaged: %v", opfPath, err)
	}
	for i := range e.pkg.Manifest {
		it := &e.pkg.Manifest[i]
		it.path = a.resolve(opfPath, it.Href)
		if _, dup := e.items[it.ID]; !dup {
			e.items[it.ID] = it
		}
	}
	return e, nil
}

// decodeXML parses a book's XML leniently: books declare encodings other
// than UTF-8 and use HTML entities such as &nbsp; that strict XML rejects.
// HTML's auto-closing is left off, since it would close an EPUB 3
// <meta property="...">value</meta> before its value.
func decodeXML(data []byte, v any) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charset.NewReaderLabel
	return d.Decode(v)
}

// metadata fills the bibliographic fields from the OPF.
func (e *epubReader) metadata() *Book {
	m := e.pkg.Metadata
	b := &Book{Encoding: "UTF-8"}
	b.Title = first(m.Titles)
	for _, c := range m.Creators {
		if c = strings.TrimSpace(c); c != "" {
			b.Authors = append(b.Authors, c)
		}
	}
	b.Publisher = first(m.Publishers)
	b.Description = plainText(first(m.Descriptions))
	b.Published = first(m.Dates)
	b.Rights = first(m.Rights)
	b.Language = first(m.Languages)
	for _, s := range m.Subjects {
		if s = strings.TrimSpace(s); s != "" {
			b.Subjects = append(b.Subjects, s)
		}
	}
	for _, id := range m.Identifiers {
		v := strings.TrimSpace(id.Value)
		if len(v) >= 9 && strings.EqualFold(v[:9], "urn:isbn:") {
			b.ISBN = strings.TrimSpace(v[9:])
			break
		}
		if strings.EqualFold(strings.TrimSpace(id.Scheme), "isbn") && v != "" {
			b.ISBN = v
			break
		}
	}
	b.Format = "EPUB 2"
	if strings.HasPrefix(strings.TrimSpace(e.pkg.Version), "3") {
		b.Format = "EPUB 3"
	}
	if e.paged {
		b.Format += " (fixed layout)"
	}
	return b
}

func first(values []string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// plainText strips markup from a description, which publishers often write
// as escaped HTML.
func plainText(s string) string {
	if !strings.Contains(s, "<") {
		return strings.Join(strings.Fields(s), " ")
	}
	var words []string
	z := html.NewTokenizer(strings.NewReader(s))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return strings.Join(words, " ")
		case html.TextToken:
			words = append(words, strings.Fields(string(z.Text()))...)
		}
	}
}

var epubImageTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true,
	"image/jpg":  true, // not a registered type, but common in hand-made books
	"image/webp": true, "image/bmp": true,
}

// numberImages gives every raster image in the manifest the next 1-based
// index, which is how the markup will address it.
func (e *epubReader) numberImages() error {
	e.images = map[string]int{}
	e.resources = map[int]*Resource{}
	for i := range e.pkg.Manifest {
		it := &e.pkg.Manifest[i]
		if !epubImageTypes[strings.ToLower(strings.TrimSpace(it.MediaType))] || !e.zip.has(it.path) {
			continue
		}
		if _, seen := e.images[it.path]; seen {
			continue
		}
		data, err := e.zip.read(it.path)
		if err != nil {
			return err
		}
		// A file whose bytes are not an image is left out rather than handed
		// to the renderer on the strength of its manifest entry.
		kind := sniffImage(data)
		if kind == "" {
			continue
		}
		idx := len(e.resources) + 1
		e.resources[idx] = &Resource{Index: idx, Kind: kind, Data: data}
		e.images[it.path] = idx
	}
	return nil
}

// findCover picks the cover the way reading systems do: EPUB 3's
// cover-image property, EPUB 2's <meta name="cover">, then the guide.
func (e *epubReader) findCover() *Resource {
	for _, it := range e.pkg.Manifest {
		if hasProperty(it.Properties, "cover-image") {
			if idx, ok := e.images[it.path]; ok {
				return e.resources[idx]
			}
		}
	}
	for _, m := range e.pkg.Metadata.Metas {
		if strings.EqualFold(m.Name, "cover") {
			if it, ok := e.items[strings.TrimSpace(m.Content)]; ok {
				if idx, ok := e.images[it.path]; ok {
					return e.resources[idx]
				}
			}
		}
	}
	for _, r := range e.pkg.Guide {
		if !strings.EqualFold(r.Type, "cover") {
			continue
		}
		target := e.zip.resolve(e.opfPath, r.Href)
		if idx, ok := e.images[target]; ok {
			return e.resources[idx]
		}
		if idx := e.firstImageIn(target); idx > 0 {
			return e.resources[idx]
		}
	}
	return nil
}

// firstImageIn returns the index of the first numbered image a document
// shows, or 0.
func (e *epubReader) firstImageIn(docPath string) int {
	if !e.zip.has(docPath) {
		return 0
	}
	doc, err := e.parseDocument(docPath)
	if err != nil {
		return 0
	}
	found := 0
	walk(doc, func(n *html.Node) bool {
		if found > 0 {
			return false
		}
		if ref := imageRef(n); ref != "" {
			found = e.images[e.zip.resolve(docPath, ref)]
		}
		return true
	})
	return found
}

// guideCoverPath is the document a guide reference names as the cover, or "".
func (e *epubReader) guideCoverPath() string {
	for _, r := range e.pkg.Guide {
		if strings.EqualFold(r.Type, "cover") {
			return e.zip.resolve(e.opfPath, r.Href)
		}
	}
	return ""
}

func hasProperty(props, name string) bool {
	return slices.Contains(strings.Fields(props), name)
}
