package ebook

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// maxHTMLResource caps each image or stylesheet read from beside an HTML
// file, so a stray link to a huge file cannot exhaust memory.
const maxHTMLResource = 64 << 20

// decodeHTMLFile reads a single HTML file. Its images and stylesheets are
// read from the file's own folder tree and nowhere else: nothing outside that
// folder, and never anything over the network.
func decodeHTMLFile(path string, data []byte) (*Book, error) {
	enc, name, _ := charset.DetermineEncoding(data, "text/html")
	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("this HTML file is not valid %s text; save it as UTF-8 and try again: %w", name, err)
	}
	// html.Parse keeps a byte order mark as body text.
	decoded = bytes.TrimPrefix(decoded, []byte("\xef\xbb\xbf"))
	// An .xhtml page may write <title/> or <a id="x"/>, which the HTML
	// parser would read as an element left open (see expandSelfClosing).
	root, err := html.Parse(bytes.NewReader(expandSelfClosing(decoded)))
	if err != nil {
		return nil, fmt.Errorf("this HTML file cannot be read: %w", err)
	}

	b := &Book{Format: "HTML", Encoding: name}
	htmlMetadata(root, b)

	folder := filepath.Dir(path)
	// Containment is checked on real paths, so the folder itself is resolved
	// once: it may sit behind a link, which is fine.
	realFolder := folder
	if p, err := filepath.EvalSymlinks(folder); err == nil {
		realFolder = p
	}
	r := &htmlFileReader{
		folder:    folder,
		real:      realFolder,
		images:    map[string]int{},
		resources: map[int]*Resource{},
		css:       &cssGatherer{seen: map[string]bool{}},
	}
	r.rewrite(root)

	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return nil, fmt.Errorf("this HTML file cannot be read: %w", err)
	}
	b.HTML = buf.String()
	b.Flows = r.css.flows
	b.Resources = r.resources
	b.TextBytes = len(b.HTML)
	return b, nil
}

// htmlMetadata fills the bibliographic fields from <title>, <html lang> and
// the <meta> tags.
func htmlMetadata(root *html.Node, b *Book) {
	if t := findElement(root, "title"); t != nil {
		b.Title = strings.Join(strings.Fields(textOf(t)), " ")
	}
	if h := findElement(root, "html"); h != nil {
		b.Language = strings.TrimSpace(attr(h, "lang"))
	}
	walk(root, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "meta" {
			return true
		}
		content := strings.TrimSpace(attr(n, "content"))
		if content == "" {
			return true
		}
		switch strings.ToLower(strings.TrimSpace(attr(n, "name"))) {
		case "author", "dc.creator":
			b.Authors = append(b.Authors, content)
		case "description":
			if b.Description == "" {
				b.Description = plainText(content)
			}
		case "keywords":
			for k := range strings.SplitSeq(content, ",") {
				if k = strings.TrimSpace(k); k != "" {
					b.Subjects = append(b.Subjects, k)
				}
			}
		}
		return true
	})
}

// textOf concatenates the text directly and indirectly inside n.
func textOf(n *html.Node) string {
	var sb strings.Builder
	walk(n, func(c *html.Node) bool {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		return true
	})
	return sb.String()
}

// htmlFileReader numbers the images an HTML file uses, in the order they are
// first referenced, and gathers its stylesheets.
type htmlFileReader struct {
	folder    string
	real      string         // folder with its symbolic links resolved
	images    map[string]int // local path or data: URI to resource number
	resources map[int]*Resource
	css       *cssGatherer
}

// rewrite points every image at its resource number, drops scripts and the
// images that cannot be used, and collects the CSS into r.css.
func (r *htmlFileReader) rewrite(root *html.Node) {
	var imgs, scripts []*html.Node
	walk(root, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}
		switch n.Data {
		case "img":
			imgs = append(imgs, n)
		case "script":
			scripts = append(scripts, n)
			return false
		case "style":
			r.css.add(textOf(n))
			return false
		case "link":
			if !hasProperty(strings.ToLower(attr(n, "rel")), "stylesheet") {
				return true
			}
			target, ok := r.localPath(attr(n, "href"))
			if !ok || r.css.seen[target] {
				return true
			}
			r.css.seen[target] = true
			if data, ok := readCapped(target); ok {
				r.css.add(string(data))
			}
		}
		return true
	})
	for _, n := range scripts {
		n.Parent.RemoveChild(n)
	}
	for _, n := range imgs {
		idx := r.image(attr(n, "src"))
		if idx == 0 {
			// Remote, outside the folder, missing or not an image: nothing
			// the renderer could draw.
			n.Parent.RemoveChild(n)
			continue
		}
		removeAttr(n, "src")
		setAttr(n, "recindex", strconv.Itoa(idx))
	}
}

// image returns the resource number for an <img src>, reading and numbering
// the image the first time it is seen, or 0 when it cannot be used.
func (r *htmlFileReader) image(src string) int {
	src = strings.TrimSpace(src)
	if src == "" {
		return 0
	}
	var key string
	var data []byte
	if strings.HasPrefix(strings.ToLower(src), "data:") {
		key = src
		if idx, seen := r.images[key]; seen {
			return idx
		}
		d, ok := decodeDataImage(src)
		if !ok {
			return 0
		}
		data = d
	} else {
		target, ok := r.localPath(src)
		if !ok {
			return 0
		}
		key = target
		if idx, seen := r.images[key]; seen {
			return idx
		}
		d, ok := readCapped(target)
		if !ok {
			return 0
		}
		data = d
	}
	kind := sniffImage(data)
	if kind == "" {
		return 0
	}
	idx := len(r.resources) + 1
	r.resources[idx] = &Resource{Index: idx, Kind: kind, Data: data}
	r.images[key] = idx
	return idx
}

// localPath resolves a reference against the HTML file's folder. It refuses
// remote URLs (the converter never touches the network) and anything that
// resolves outside the folder tree, so a page cannot pull in arbitrary files.
func (r *htmlFileReader) localPath(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	lower := strings.ToLower(ref)
	if ref == "" || strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.HasPrefix(ref, "//") {
		return "", false
	}
	unescaped, err := url.PathUnescape(ref)
	if err != nil {
		return "", false
	}
	target := filepath.Clean(filepath.Join(r.folder, filepath.FromSlash(unescaped)))
	if !within(r.folder, target) {
		return "", false
	}
	// A symbolic link anywhere along the path, a folder as much as the file,
	// could still lead outside, so the check is repeated on the real path,
	// which is also the one read.
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil || !within(r.real, resolved) {
		return "", false
	}
	return resolved, true
}

// within reports whether target lies inside folder.
func within(folder, target string) bool {
	rel, err := filepath.Rel(folder, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// readCapped reads a regular file of at most maxHTMLResource bytes. The path
// has had its links resolved already (localPath); Lstat keeps it that way.
func readCapped(target string) ([]byte, bool) {
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxHTMLResource {
		return nil, false
	}
	f, err := os.Open(target)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	// The limit holds even if the file grew after the Lstat.
	data, err := io.ReadAll(io.LimitReader(f, maxHTMLResource+1))
	if err != nil || len(data) > maxHTMLResource {
		return nil, false
	}
	return data, true
}

// decodeDataImage decodes a data:image/...;base64, URI.
func decodeDataImage(uri string) ([]byte, bool) {
	header, payload, ok := strings.Cut(uri[len("data:"):], ",")
	if !ok {
		return nil, false
	}
	header = strings.ToLower(header)
	if !strings.HasPrefix(header, "image/") || !strings.HasSuffix(header, ";base64") {
		return nil, false
	}
	// Data URIs in hand-written HTML are often wrapped across lines.
	payload = strings.Join(strings.Fields(payload), "")
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, false
	}
	return data, true
}
