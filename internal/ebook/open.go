package ebook

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Format is a kind of ebook file, as recognised by Detect.
type Format int

const (
	FormatUnknown Format = iota
	FormatMOBI
	FormatEPUB
	FormatCBZ
	FormatFB2
	FormatFB2Zip
	FormatText
	FormatHTML
)

// String names the format for messages.
func (f Format) String() string {
	switch f {
	case FormatMOBI:
		return "Kindle (MOBI/AZW3)"
	case FormatEPUB:
		return "EPUB"
	case FormatCBZ:
		return "comic archive (CBZ)"
	case FormatFB2:
		return "FictionBook (FB2)"
	case FormatFB2Zip:
		return "zipped FictionBook (FB2)"
	case FormatText:
		return "plain text"
	case FormatHTML:
		return "HTML"
	default:
		return "unknown"
	}
}

// ErrUnsupported reports a file that is not any format this package reads.
var ErrUnsupported = errors.New("not a supported ebook")

// sniffLen is how much of a file the content checks look at.
const sniffLen = 4 << 10

var zipMagic = []byte("PK\x03\x04")

// Open reads and decodes an ebook from disk. Every error names the file.
func Open(path string) (*Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	format, err := detect(path, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var b *Book
	if format == FormatHTML {
		b, err = decodeHTMLFile(path, data)
	} else {
		b, err = decodeAs(format, data)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	b.Path = path
	b.FileSize = int64(len(data))
	if b.Title == "" {
		b.Title = titleFromName(path)
	}
	return b, nil
}

// Decode decodes an ebook already held in memory. With no file name to go
// on, HTML cannot be recognised (its images live beside the file anyway), and
// anything unrecognised that looks like text is read as plain text.
func Decode(data []byte) (*Book, error) {
	format, err := detect("", data)
	if err != nil {
		return nil, err
	}
	if format == FormatUnknown && bytes.IndexByte(head(data), 0) < 0 {
		format = FormatText
	}
	b, err := decodeAs(format, data)
	if err != nil {
		return nil, err
	}
	if b.Title == "" {
		b.Title = "Untitled"
	}
	return b, nil
}

func decodeAs(format Format, data []byte) (*Book, error) {
	switch format {
	case FormatMOBI:
		return decodeMOBI(data)
	case FormatEPUB:
		return decodeEPUB(data)
	case FormatCBZ:
		return decodeCBZ(data)
	case FormatFB2:
		return decodeFB2(data)
	case FormatFB2Zip:
		return decodeFB2Zip(data)
	case FormatText:
		return decodePlainText(data)
	}
	return nil, fmt.Errorf("%w (expected EPUB, Kindle, CBZ, FB2, TXT or HTML)", ErrUnsupported)
}

// Detect works out a file's format, from its content first and its name
// second, so a Kindle book misnamed .epub is still read as a Kindle book.
func Detect(name string, data []byte) Format {
	f, _ := detect(name, data)
	return f
}

// detect is Detect that also reports a zip file too damaged to look inside,
// so Open can say so rather than calling the file unsupported.
func detect(name string, data []byte) (Format, error) {
	if isPalm(data) {
		return FormatMOBI, nil
	}
	if bytes.HasPrefix(data, zipMagic) {
		a, err := openZip(data)
		if err != nil {
			return FormatUnknown, err
		}
		return classifyZip(a), nil
	}
	return detectText(name, data), nil
}

// DetectFile is Detect for a file on disk, reading only what it needs: the
// first few KiB, or a zip file's central directory.
func DetectFile(name string) (Format, error) {
	f, err := os.Open(name)
	if err != nil {
		return FormatUnknown, err
	}
	data, err := io.ReadAll(io.LimitReader(f, sniffLen))
	_ = f.Close()
	if err != nil {
		return FormatUnknown, err
	}
	if isPalm(data) {
		return FormatMOBI, nil
	}
	if bytes.HasPrefix(data, zipMagic) {
		r, err := zip.OpenReader(name)
		if err != nil {
			return FormatUnknown, fmt.Errorf("%s: %w: %v", name, errBrokenArchive, err)
		}
		defer func() { _ = r.Close() }()
		return classifyZip(newZipArchive(r.File)), nil
	}
	return detectText(name, data), nil
}

func isPalm(data []byte) bool {
	if len(data) < 68 {
		return false
	}
	id := string(data[60:68])
	return id == "BOOKMOBI" || id == "TEXtREAd"
}

// classifyZip tells EPUB, zipped FictionBook and comic archives apart by what
// they hold.
func classifyZip(a *zipArchive) Format {
	if a.has("mimetype") {
		if m, err := a.read("mimetype"); err == nil && bytes.Contains(m, []byte("application/epub+zip")) {
			return FormatEPUB
		}
	}
	if a.has("META-INF/container.xml") {
		return FormatEPUB
	}
	var files, images int
	for _, n := range a.names() {
		if strings.HasSuffix(strings.ToLower(n), ".fb2") {
			return FormatFB2Zip
		}
	}
	for _, n := range a.names() {
		base := path.Base(n)
		if strings.HasPrefix(n, "__MACOSX/") || strings.HasPrefix(base, ".") || strings.EqualFold(base, "ComicInfo.xml") {
			continue
		}
		files++
		if isImageName(n) {
			images++
		}
	}
	if images > 0 && images*2 >= files {
		return FormatCBZ
	}
	return FormatUnknown
}

func isImageName(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// detectText recognises FictionBook by content and plain text and HTML by
// name, since neither has a signature of its own.
func detectText(name string, data []byte) Format {
	h := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	h = bytes.TrimLeft(h, " \t\r\n")
	if bytes.Contains(head(h), []byte("<FictionBook")) {
		return FormatFB2
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		return FormatText
	case ".html", ".htm", ".xhtml":
		return FormatHTML
	}
	return FormatUnknown
}

func head(data []byte) []byte {
	if len(data) > sniffLen {
		return data[:sniffLen]
	}
	return data
}

// titleFromName is the fallback title: the file name without its extension,
// counting .fb2.zip as one.
func titleFromName(name string) string {
	base := filepath.Base(name)
	if lower := strings.ToLower(base); strings.HasSuffix(lower, ".fb2.zip") {
		return base[:len(base)-len(".fb2.zip")]
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}
