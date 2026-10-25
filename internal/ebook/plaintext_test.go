package ebook

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const gutenbergText = "The Project Gutenberg eBook of A Test Story\r\n" +
	"\r\n" +
	"Title: A Test Story\r\n" +
	"\r\n" +
	"Author: Ann Author\r\n" +
	"\r\n" +
	"*** START OF THE PROJECT GUTENBERG EBOOK A TEST STORY ***\r\n" +
	"\r\n" +
	"CHAPTER I\r\n" +
	"\r\n" +
	"It was a dark night, and the wind\r\n" +
	"was up.\r\n" +
	"\r\n" +
	"*** END OF THE PROJECT GUTENBERG EBOOK A TEST STORY ***\r\n" +
	"\r\n" +
	"Section 1. General Terms of Use of the licence.\r\n"

func TestDecodePlainTextGutenberg(t *testing.T) {
	b, err := decodePlainText([]byte(gutenbergText))
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "A Test Story" || !slices.Equal(b.Authors, []string{"Ann Author"}) {
		t.Errorf("title %q, authors %q", b.Title, b.Authors)
	}
	if b.Format != "Plain text" || b.Encoding != "UTF-8" {
		t.Errorf("format %q, encoding %q", b.Format, b.Encoding)
	}
	for _, gone := range []string{"Project Gutenberg", "Title:", "licence", "START OF"} {
		if strings.Contains(b.HTML, gone) {
			t.Errorf("markup still holds %q: %s", gone, b.HTML)
		}
	}
	if !strings.Contains(b.HTML, "<p>It was a dark night, and the wind was up.</p>") {
		t.Errorf("hard-wrapped lines were not joined into one paragraph: %s", b.HTML)
	}
	if got := headingsOf(b); !slices.Equal(got, []string{"2:CHAPTER I"}) {
		t.Errorf("headings %q", got)
	}
}

func TestDecodePlainTextWindows1252(t *testing.T) {
	b, err := decodePlainText([]byte("\x93Quoted,\x94 she said.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b.Encoding != "Windows-1252" {
		t.Errorf("encoding %q", b.Encoding)
	}
	if !strings.Contains(b.HTML, "“Quoted,” she said.") {
		t.Errorf("quotes not decoded: %s", b.HTML)
	}
}

func TestDecodePlainTextBOMAndEscaping(t *testing.T) {
	b, err := decodePlainText([]byte("\xef\xbb\xbfFish & <chips>\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b.HTML != "<p>Fish &amp; &lt;chips&gt;</p>\n" {
		t.Errorf("markup %q", b.HTML)
	}
}

func TestTextHeadings(t *testing.T) {
	for _, line := range []string{"CHAPTER I", "Chapter 12: The Return", "Part Two", "XIV", "IV.", "Prologue", "Book 3 - The Sea"} {
		if !isTextHeading(line) {
			t.Errorf("%q should be a heading", line)
		}
	}
	long := "Chapter " + strings.Repeat("x", 82)
	for _, line := range []string{
		"Chapter and verse were quoted at length.",
		"I",
		"I said nothing.",
		"Book me a table for two, please",
		long,
		"Did",
	} {
		if isTextHeading(line) {
			t.Errorf("%q should not be a heading", line)
		}
	}
}

func TestDecodePlainTextEmpty(t *testing.T) {
	_, err := decodePlainText([]byte("\xef\xbb\xbf \r\n\t\n"))
	if !errors.Is(err, errEmptyText) || err.Error() != "this text file is empty" {
		t.Errorf("err = %v", err)
	}
}
