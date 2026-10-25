package ebook

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"

	"github.com/jparkerweb/ebook-to-pdf/internal/htmldoc"
)

// fb2Fixture is a small but complete FictionBook: metadata, nested sections,
// inline styling, a poem, an epigraph, a notes body and two images whose
// binaries come after the bodies, as they do in real files.
func fb2Fixture(t *testing.T) string {
	t.Helper()
	cover := base64.StdEncoding.EncodeToString(pngOf(t, 30, 40))
	pic := base64.StdEncoding.EncodeToString(pngOf(t, 20, 10))
	// Split the base64 over lines, as FictionBook writers do.
	pic = pic[:10] + "\n  " + pic[10:]
	return `<?xml version="1.0" encoding="UTF-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0" xmlns:l="http://www.w3.org/1999/xlink">
  <description>
    <title-info>
      <genre>sf</genre>
      <genre>adventure</genre>
      <author><first-name>Ann</first-name><middle-name>B.</middle-name><last-name>Author</last-name></author>
      <author><nickname>Ghost</nickname></author>
      <book-title>The Test Book</book-title>
      <annotation><p>A <emphasis>short</emphasis> book.</p><p>Second line.</p></annotation>
      <date>2001</date>
      <coverpage><image l:href="#cover.png"/></coverpage>
      <lang>en</lang>
    </title-info>
    <publish-info>
      <publisher>Test Press</publisher>
      <year>2002</year>
      <isbn>9780000000002</isbn>
    </publish-info>
  </description>
  <body>
    <section>
      <title><p>Part One</p><p>The Start</p></title>
      <epigraph><p>Quoted words.</p><text-author>Someone</text-author></epigraph>
      <section>
        <title><p>Chapter 1</p></title>
        <p>Some <emphasis>emphasised</emphasis> &amp; <strong>strong</strong> text with <a l:href="https://example.com">a link</a> and <a l:href="#n1">a note</a>.</p>
        <image l:href="#pic.png"/>
        <poem><stanza><v>Line one</v><v>Line two</v></stanza></poem>
      </section>
    </section>
    <section>
      <title><p>Part Two</p></title>
      <p>More text.</p>
    </section>
  </body>
  <body name="notes">
    <title><p>Notes</p></title>
    <section id="n1"><title><p>1</p></title><p>The note.</p></section>
  </body>
  <binary id="cover.png" content-type="image/png">` + cover + `</binary>
  <binary id="pic.png" content-type="image/png">` + pic + `</binary>
</FictionBook>`
}

func TestFB2Metadata(t *testing.T) {
	b, err := decodeFB2([]byte(fb2Fixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "The Test Book" {
		t.Errorf("Title = %q", b.Title)
	}
	if want := []string{"Ann B. Author", "Ghost"}; !reflect.DeepEqual(b.Authors, want) {
		t.Errorf("Authors = %q, want %q", b.Authors, want)
	}
	if b.Description != "A short book. Second line." {
		t.Errorf("Description = %q", b.Description)
	}
	if b.Published != "2001" || b.Language != "en" || b.Publisher != "Test Press" || b.ISBN != "9780000000002" {
		t.Errorf("Published %q, Language %q, Publisher %q, ISBN %q", b.Published, b.Language, b.Publisher, b.ISBN)
	}
	if want := []string{"sf", "adventure"}; !reflect.DeepEqual(b.Subjects, want) {
		t.Errorf("Subjects = %q, want %q", b.Subjects, want)
	}
	if b.Format != "FictionBook 2" || b.Encoding != "UTF-8" {
		t.Errorf("Format %q, Encoding %q", b.Format, b.Encoding)
	}
	if b.Cover == nil || b.Cover.Index != 1 || b.Cover.Kind != "png" {
		t.Fatalf("Cover = %+v, want the first binary", b.Cover)
	}
	if len(b.Resources) != 2 {
		t.Errorf("got %d resources, want 2", len(b.Resources))
	}
}

func TestFB2Body(t *testing.T) {
	b, err := decodeFB2([]byte(fb2Fixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	m := recindexAttr.FindAllStringSubmatch(b.HTML, -1)
	if len(m) != 1 || m[0][1] != "2" {
		t.Errorf("recindex references = %v, want only the body image as 2", m)
	}
	for _, want := range []string{
		"<em>emphasised</em>", "&amp;", "<strong>strong</strong>",
		`<a href="https://example.com">a link</a>`, "and a note.",
		"<blockquote><p>Line one<br>Line two</p></blockquote>",
		"<blockquote><p>Quoted words.</p><p>Someone</p></blockquote>",
		"<mbp:pagebreak/><h1>Notes</h1>",
	} {
		if !strings.Contains(b.HTML, want) {
			t.Errorf("HTML is missing %q:\n%s", want, b.HTML)
		}
	}
	if strings.Contains(b.HTML, "#n1") {
		t.Error("an internal link kept its target")
	}

	got := headingsOf(b)
	want := []string{"1:Part One The Start", "2:Chapter 1", "1:Part Two", "1:Notes", "2:1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("headings = %q, want %q", got, want)
	}

	// Every top-level section starts a page: there is a break straight
	// before "Part Two".
	doc := htmldoc.Parse(b.HTML)
	found := false
	for i, blk := range doc.Blocks {
		if blk.Kind == htmldoc.KindHeading && strings.TrimSpace(blk.Text()) == "Part Two" {
			found = i > 0 && doc.Blocks[i-1].Kind == htmldoc.KindPageBreak
		}
	}
	if !found {
		t.Error("no page break before the second top-level section")
	}
}

func TestFB2Windows1251(t *testing.T) {
	const title = "Война и мир"
	const para = "Съешь же ещё этих мягких французских булок."
	src := `<?xml version="1.0" encoding="windows-1251"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
<description><title-info><book-title>` + title + `</book-title></title-info></description>
<body><section><p>` + para + `</p></section></body>
</FictionBook>`
	enc, err := charmap.Windows1251.NewEncoder().String(src)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"declared":   enc,
		"undeclared": strings.Replace(enc, ` encoding="windows-1251"`, "", 1),
	} {
		b, err := decodeFB2([]byte(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if b.Title != title || !strings.Contains(b.HTML, para) {
			t.Errorf("%s: Title %q, HTML %q", name, b.Title, b.HTML)
		}
		if b.Encoding != "windows-1251" {
			t.Errorf("%s: Encoding = %q", name, b.Encoding)
		}
	}
}

func TestFB2Zip(t *testing.T) {
	src := []byte(fb2Fixture(t))
	plain, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	zipped, err := Decode(buildZip(t, zipEntry{Name: "book.fb2", Data: src}))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain, zipped) {
		t.Errorf("the zipped book decodes differently:\nplain  %+v\nzipped %+v", plain, zipped)
	}

	_, err = decodeFB2Zip(buildZip(t, zipEntry{Name: "readme.txt", Data: []byte("hi")}))
	if err == nil || err.Error() != "this zip archive does not contain a .fb2 book" {
		t.Errorf("err = %v", err)
	}
}

func TestFB2Damaged(t *testing.T) {
	_, err := decodeFB2([]byte(`<?xml version="1.0"?><FictionBook><body><p>open</body></FictionBook>`))
	if err == nil || !strings.HasPrefix(err.Error(), "this FictionBook file is damaged: ") {
		t.Errorf("err = %v", err)
	}
}
