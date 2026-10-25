package ebook

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

var errEmptyText = errors.New("this text file is empty")

// gutenbergStart and gutenbergEnd find the markers Project Gutenberg puts
// around a book's text: the header before the start marker names the book,
// and the licence after the end marker is not part of it.
var (
	gutenbergStart = regexp.MustCompile(`(?im)^\*\*\* *START OF TH(?:E|IS) PROJECT GUTENBERG EBOOK.*$`)
	gutenbergEnd   = regexp.MustCompile(`(?im)^\*\*\* *END OF`)
	gutenbergField = regexp.MustCompile(`(?im)^(Title|Author): *(.+)$`)
)

// decodePlainText reads a text file: paragraphs are separated by blank lines,
// and a few conservative patterns pick out chapter headings.
func decodePlainText(data []byte) (*Book, error) {
	b := &Book{Format: "Plain text", Encoding: "UTF-8"}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	text := string(data)
	if !utf8.Valid(data) {
		// Text that is not UTF-8 is almost always a Windows code page, and
		// Windows-1252 decodes every byte, so nothing is lost.
		dec, err := charmap.Windows1252.NewDecoder().Bytes(data)
		if err != nil {
			return nil, err
		}
		text = string(dec)
		b.Encoding = "Windows-1252"
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = trimGutenberg(text, b)

	var out strings.Builder
	for _, para := range paragraphs(text) {
		line := strings.Join(para, " ")
		if len(para) == 1 && isTextHeading(line) {
			fmt.Fprintf(&out, "<mbp:pagebreak/><h2>%s</h2>\n", html.EscapeString(line))
			continue
		}
		fmt.Fprintf(&out, "<p>%s</p>\n", html.EscapeString(line))
	}
	if out.Len() == 0 {
		return nil, errEmptyText
	}
	b.HTML = out.String()
	b.TextBytes = len(text)
	return b, nil
}

// trimGutenberg keeps only the book between Project Gutenberg's markers,
// taking the title and author from the header. Text without the markers is
// returned as it is.
func trimGutenberg(text string, b *Book) string {
	start := gutenbergStart.FindStringIndex(text)
	if start == nil {
		return text
	}
	for _, m := range gutenbergField.FindAllStringSubmatch(text[:start[0]], -1) {
		v := strings.TrimSpace(m[2])
		switch {
		case strings.EqualFold(m[1], "Title") && b.Title == "":
			b.Title = v
		case strings.EqualFold(m[1], "Author") && len(b.Authors) == 0 && v != "":
			b.Authors = []string{v}
		}
	}
	body := text[start[1]:]
	if end := gutenbergEnd.FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body
}

// paragraphs splits text on blank lines, returning each paragraph's lines
// with their surrounding space trimmed.
func paragraphs(text string) [][]string {
	var out [][]string
	var cur []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if cur != nil {
				out = append(out, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, line)
	}
	if cur != nil {
		out = append(out, cur)
	}
	return out
}

// headingKeyword is a line that starts with a chapter word and then holds at
// most a number and a short title, such as "CHAPTER IV", "Part Two" or
// "Chapter 12: The Return". Anything more is likely a sentence that happens
// to start with "Chapter" or "Book".
var headingKeyword = regexp.MustCompile(`(?i)^(?:chapter|book|part|volume|prologue|epilogue|introduction|preface|foreword|afterword|appendix)\b` +
	`(?:\s+(?:\d+|[ivxlcdm]+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty))?` +
	`(?:\s*[.:\-\x{2013}\x{2014}]\s*\S.*)?$`)

// romanNumeral is a well-formed Roman numeral, so words that merely use the
// letters (DID, CIVIL) are not taken for one.
var romanNumeral = regexp.MustCompile(`(?i)^M{0,3}(?:CM|CD|D?C{0,3})(?:XC|XL|L?X{0,3})(?:IX|IV|V?I{0,3})$`)

// isTextHeading reports whether a one-line paragraph is a chapter heading.
// It errs towards no: a missed heading costs a bookmark, a false one breaks
// the page in the middle of the text.
func isTextHeading(line string) bool {
	if utf8.RuneCountInString(line) > 60 {
		return false
	}
	bare := strings.TrimSuffix(line, ".")
	if isLoneNumeral(bare) {
		return true
	}
	if strings.ContainsAny(line[len(line)-1:], ".,;:") {
		return false
	}
	return headingKeyword.MatchString(line)
}

// isLoneNumeral accepts a Roman numeral on its own, other than "I", which is
// far more often the pronoun.
func isLoneNumeral(s string) bool {
	return s != "" && len(s) <= 7 && !strings.EqualFold(s, "I") && romanNumeral.MatchString(s)
}
