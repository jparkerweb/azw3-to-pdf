package ebook

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// tocTarget is one table-of-contents entry within a document: the element id
// it points at (empty for the top of the document) and its nesting depth,
// 1 for the top level.
type tocTarget struct {
	Fragment string
	Depth    int
}

// readTOC returns the table of contents keyed by document path, from the
// EPUB 3 nav document or, failing that, the EPUB 2 NCX. A table of contents
// that is missing or broken only costs the bookmarks, so it is never an
// error.
func (e *epubReader) readTOC() map[string][]tocTarget {
	toc := map[string][]tocTarget{}
	if nav, navPath := e.navDocument(); nav != nil {
		if n := findNav(nav, "toc"); n != nil {
			var visit func(n *html.Node, depth int)
			visit = func(n *html.Node, depth int) {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type != html.ElementNode {
						continue
					}
					switch c.Data {
					case "ol":
						visit(c, depth+1)
						continue
					case "a":
						if href := attr(c, "href"); href != "" {
							e.addTarget(toc, navPath, href, depth)
						}
					}
					visit(c, depth)
				}
			}
			visit(n, 0)
			if len(toc) > 0 {
				return toc
			}
		}
	}

	ncxPath := ""
	if it, ok := e.items[e.pkg.Spine.Toc]; ok {
		ncxPath = it.path
	} else {
		for _, it := range e.pkg.Manifest {
			if it.MediaType == "application/x-dtbncx+xml" {
				ncxPath = it.path
				break
			}
		}
	}
	if ncxPath == "" || !e.zip.has(ncxPath) {
		return toc
	}
	raw, err := e.zip.read(ncxPath)
	if err != nil {
		return toc
	}
	var ncx struct {
		Points []ncxPoint `xml:"navMap>navPoint"`
	}
	if decodeXML(raw, &ncx) != nil {
		return toc
	}
	var visit func(points []ncxPoint, depth int)
	visit = func(points []ncxPoint, depth int) {
		for _, p := range points {
			if p.Content.Src != "" {
				e.addTarget(toc, ncxPath, p.Content.Src, depth)
			}
			visit(p.Points, depth+1)
		}
	}
	visit(ncx.Points, 1)
	return toc
}

type ncxPoint struct {
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	Points []ncxPoint `xml:"navPoint"`
}

func (e *epubReader) addTarget(toc map[string][]tocTarget, base, href string, depth int) {
	doc := e.zip.resolve(base, href)
	_, frag, _ := strings.Cut(href, "#")
	if u, err := url.PathUnescape(frag); err == nil {
		frag = u
	}
	toc[doc] = append(toc[doc], tocTarget{Fragment: frag, Depth: max(depth, 1)})
}

// navDocument parses the EPUB 3 navigation document, if there is one.
func (e *epubReader) navDocument() (*html.Node, string) {
	for _, it := range e.pkg.Manifest {
		if hasProperty(it.Properties, "nav") {
			if root, err := e.parseDocument(it.path); err == nil {
				return root, it.path
			}
		}
	}
	return nil, ""
}

// findNav returns the <nav> whose epub:type includes kind ("toc",
// "landmarks").
func findNav(root *html.Node, kind string) *html.Node {
	var found *html.Node
	walk(root, func(n *html.Node) bool {
		if found != nil {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "nav" && hasProperty(attr(n, "epub:type"), kind) {
			found = n
			return false
		}
		return true
	})
	return found
}

// landmarkCoverPath is the document the EPUB 3 landmarks name as the cover,
// or "".
func (e *epubReader) landmarkCoverPath() string {
	nav, navPath := e.navDocument()
	if nav == nil {
		return ""
	}
	marks := findNav(nav, "landmarks")
	if marks == nil {
		return ""
	}
	cover := ""
	walk(marks, func(n *html.Node) bool {
		if cover != "" {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasProperty(attr(n, "epub:type"), "cover") {
			cover = e.zip.resolve(navPath, attr(n, "href"))
		}
		return true
	})
	return cover
}
