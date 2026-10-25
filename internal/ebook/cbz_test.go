package ebook

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	want := []string{"p1", "p2", "p10", "P11", "p100"}
	got := []string{"p100", "P11", "p2", "p10", "p1"}
	slices.SortFunc(got, naturalCompare)
	if !slices.Equal(got, want) {
		t.Errorf("sorted %v, want %v", got, want)
	}

	nested := []string{"ch10/p1.jpg", "ch1/p10.jpg", "ch2/p1.jpg", "ch1/p9.jpg", "ch1/p1.jpg"}
	slices.SortFunc(nested, naturalCompare)
	want = []string{"ch1/p1.jpg", "ch1/p9.jpg", "ch1/p10.jpg", "ch2/p1.jpg", "ch10/p1.jpg"}
	if !slices.Equal(nested, want) {
		t.Errorf("sorted %v, want %v", nested, want)
	}

	if naturalLess("a", "a") {
		t.Error("a name is not less than itself")
	}
	if !naturalLess("a01", "a1") && !naturalLess("a1", "a01") {
		t.Error("equal numbers with different padding must still have an order")
	}
}

const testComicInfo = `<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>The Long Night</Title>
  <Series>Night Watch</Series>
  <Number>3</Number>
  <Writer>Ann Writer, Bob Writer</Writer>
  <Summary>The watch stays up.</Summary>
  <Publisher>Panel House</Publisher>
  <Year>2021</Year>
  <Month>4</Month>
  <LanguageISO>en</LanguageISO>
  <Genre>Fantasy, Adventure</Genre>
</ComicInfo>`

func TestDecodeCBZ(t *testing.T) {
	one, two, ten := pngOf(t, 10, 10), pngOf(t, 20, 20), pngOf(t, 30, 30)
	data := buildZip(t,
		zipEntry{Name: "10.png", Data: ten},
		zipEntry{Name: "2.png", Data: two},
		zipEntry{Name: "1.png", Data: one},
		zipEntry{Name: ".hidden.png", Data: one},
		zipEntry{Name: "__MACOSX/._1.png", Data: one},
		zipEntry{Name: "notes.txt", Data: []byte("not a page")},
		zipEntry{Name: "ComicInfo.xml", Data: []byte(testComicInfo)},
	)
	b, err := decodeCBZ(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Resources) != 3 {
		t.Fatalf("%d pages, want 3", len(b.Resources))
	}
	for i, want := range [][]byte{one, two, ten} {
		if r := b.Resources[i+1]; r == nil || string(r.Data) != string(want) {
			t.Errorf("page %d is not the expected image", i+1)
		}
	}
	if got := strings.Count(b.HTML, `data-ebook-fullpage="1"`); got != 3 {
		t.Errorf("%d full-page images in the markup, want 3", got)
	}
	if !strings.Contains(b.HTML, `<img recindex="1" data-ebook-fullpage="1">`) {
		t.Errorf("markup does not address page 1: %s", b.HTML)
	}
	if !b.Paged {
		t.Error("a comic must be marked as paged")
	}
	if b.Cover == nil || b.Cover != b.Resources[1] {
		t.Error("the cover should be the first page")
	}
	if b.Format != "Comic archive (CBZ)" || b.TextBytes != 0 {
		t.Errorf("Format %q, TextBytes %d", b.Format, b.TextBytes)
	}
	if b.Title != "The Long Night" {
		t.Errorf("title %q", b.Title)
	}
	if !slices.Equal(b.Authors, []string{"Ann Writer", "Bob Writer"}) {
		t.Errorf("authors %q", b.Authors)
	}
	if b.Published != "2021-04" || b.Publisher != "Panel House" || b.Language != "en" {
		t.Errorf("published %q, publisher %q, language %q", b.Published, b.Publisher, b.Language)
	}
	if !slices.Equal(b.Subjects, []string{"Fantasy", "Adventure"}) || b.Description != "The watch stays up." {
		t.Errorf("subjects %q, description %q", b.Subjects, b.Description)
	}
}

func TestDecodeCBZSeriesTitle(t *testing.T) {
	info := `<ComicInfo><Series>Night Watch</Series><Number>3</Number></ComicInfo>`
	b, err := decodeCBZ(buildZip(t,
		zipEntry{Name: "comicinfo.XML", Data: []byte(info)},
		zipEntry{Name: "p1.png", Data: pngOf(t, 10, 10)},
	))
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Night Watch #3" {
		t.Errorf("title %q, want the series and number", b.Title)
	}
}

func TestDecodeCBZBadComicInfo(t *testing.T) {
	b, err := decodeCBZ(buildZip(t,
		zipEntry{Name: "ComicInfo.xml", Data: []byte("<ComicInfo><Title>Broken")},
		zipEntry{Name: "p1.png", Data: pngOf(t, 10, 10)},
	))
	if err != nil {
		t.Fatalf("a malformed ComicInfo.xml should be ignored: %v", err)
	}
	if b.Title != "" {
		t.Errorf("title %q from a malformed ComicInfo.xml", b.Title)
	}
}

func TestDecodeCBZNoImages(t *testing.T) {
	_, err := decodeCBZ(buildZip(t,
		zipEntry{Name: "readme.txt", Data: []byte("hello")},
		zipEntry{Name: "fake.png", Data: []byte("not really a picture")},
	))
	if !errors.Is(err, errNoComicImages) || err.Error() != "this comic archive contains no images" {
		t.Errorf("err = %v", err)
	}
}
