package ebook

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var errNoComicImages = errors.New("this comic archive contains no images")

// decodeCBZ reads a comic archive: every image is one page, in the order a
// person would read the file names.
func decodeCBZ(data []byte) (*Book, error) {
	a, err := openZip(data)
	if err != nil {
		return nil, err
	}
	var pages []string
	for _, n := range a.names() {
		if isComicPage(n) {
			pages = append(pages, n)
		}
	}
	slices.SortFunc(pages, naturalCompare)

	b := &Book{Format: "Comic archive (CBZ)", Paged: true, Resources: map[int]*Resource{}}
	var out strings.Builder
	for _, n := range pages {
		img, err := a.read(n)
		if err != nil {
			return nil, err
		}
		// Archives often carry thumbnails or files with an image extension
		// that are not images at all; they are skipped, not drawn as blanks.
		kind := sniffImage(img)
		if kind == "" {
			continue
		}
		idx := len(b.Resources) + 1
		b.Resources[idx] = &Resource{Index: idx, Kind: kind, Data: img}
		fmt.Fprintf(&out, `<html><body><img recindex="%d" data-ebook-fullpage="1"></body></html>`, idx)
	}
	if len(b.Resources) == 0 {
		return nil, errNoComicImages
	}
	b.Cover = b.Resources[1]
	b.HTML = out.String()
	readComicInfo(a, b)
	return b, nil
}

// isComicPage reports whether an archive entry is a page image. Dot files and
// __MACOSX/ hold the resource forks and thumbnails macOS adds when zipping.
func isComicPage(name string) bool {
	if strings.HasPrefix(name, "__MACOSX/") || !isImageName(name) {
		return false
	}
	for seg := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return false
		}
	}
	return true
}

// naturalLess orders names the way people number pages: case-insensitively,
// with runs of digits compared as numbers, so 2.jpg comes before 10.jpg. The
// plain string breaks ties, keeping the order stable.
func naturalLess(a, b string) bool {
	x, y := strings.ToLower(a), strings.ToLower(b)
	for x != "" && y != "" {
		if isDigit(x[0]) && isDigit(y[0]) {
			nx, ny := digitRun(x), digitRun(y)
			if c := compareNumbers(x[:nx], y[:ny]); c != 0 {
				return c < 0
			}
			x, y = x[nx:], y[ny:]
			continue
		}
		if x[0] != y[0] {
			return x[0] < y[0]
		}
		x, y = x[1:], y[1:]
	}
	if len(x) != len(y) {
		return len(x) < len(y)
	}
	return a < b
}

// naturalCompare is naturalLess in the form slices.SortFunc takes.
func naturalCompare(a, b string) int {
	switch {
	case naturalLess(a, b):
		return -1
	case naturalLess(b, a):
		return 1
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digitRun(s string) int {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}
	return n
}

// compareNumbers compares two runs of digits by value without parsing them,
// so a page number too long for an int still sorts correctly.
func compareNumbers(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// comicInfo is the ComicRack metadata file, reduced to what a Book holds.
type comicInfo struct {
	Title       string `xml:"Title"`
	Series      string `xml:"Series"`
	Number      string `xml:"Number"`
	Writer      string `xml:"Writer"`
	Summary     string `xml:"Summary"`
	Publisher   string `xml:"Publisher"`
	Year        string `xml:"Year"`
	Month       string `xml:"Month"`
	LanguageISO string `xml:"LanguageISO"`
	Genre       string `xml:"Genre"`
}

// readComicInfo fills b's metadata from ComicInfo.xml at the archive root.
// The file is optional and often hand-edited, so a malformed one is ignored.
func readComicInfo(a *zipArchive, b *Book) {
	name := ""
	for _, n := range a.names() {
		if !strings.Contains(n, "/") && strings.EqualFold(n, "ComicInfo.xml") {
			name = n
			break
		}
	}
	if name == "" {
		return
	}
	raw, err := a.read(name)
	if err != nil {
		return
	}
	var ci comicInfo
	if err := decodeXML(raw, &ci); err != nil {
		return
	}
	b.Title = strings.TrimSpace(ci.Title)
	if b.Title == "" {
		if series := strings.TrimSpace(ci.Series); series != "" {
			b.Title = series
			if num := strings.TrimSpace(ci.Number); num != "" {
				b.Title += " #" + num
			}
		}
	}
	b.Authors = splitList(ci.Writer)
	b.Description = strings.TrimSpace(ci.Summary)
	b.Publisher = strings.TrimSpace(ci.Publisher)
	if year := strings.TrimSpace(ci.Year); year != "" {
		b.Published = year
		if month := strings.TrimSpace(ci.Month); month != "" {
			if len(month) == 1 {
				month = "0" + month
			}
			b.Published += "-" + month
		}
	}
	b.Language = strings.TrimSpace(ci.LanguageISO)
	b.Subjects = splitList(ci.Genre)
}

// splitList splits a comma-separated ComicInfo field, dropping empty items.
func splitList(s string) []string {
	var out []string
	for v := range strings.SplitSeq(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
