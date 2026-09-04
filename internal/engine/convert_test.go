package engine

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

// writeTestBook writes an uncompressed MOBI 6 file, which is the smallest
// thing the whole pipeline can be run against.
func writeTestBook(t *testing.T, dir, name, title, markup string) string {
	t.Helper()

	record0 := make([]byte, 0x100)
	binary.BigEndian.PutUint16(record0[0x00:], 1) // no compression
	binary.BigEndian.PutUint32(record0[0x04:], uint32(len(markup)))
	binary.BigEndian.PutUint16(record0[0x08:], 1)
	binary.BigEndian.PutUint16(record0[0x0a:], 4096)
	copy(record0[0x10:], "MOBI")
	binary.BigEndian.PutUint32(record0[0x14:], 0xe8)
	binary.BigEndian.PutUint32(record0[0x18:], 2)
	binary.BigEndian.PutUint32(record0[0x1c:], 65001)
	binary.BigEndian.PutUint32(record0[0x24:], 6)
	binary.BigEndian.PutUint32(record0[0x54:], 0xc0)
	binary.BigEndian.PutUint32(record0[0x58:], uint32(len(title)))
	copy(record0[0xc0:], title)

	records := [][]byte{record0, []byte(markup)}

	var buf bytes.Buffer
	header := make([]byte, 78)
	copy(header[60:64], "BOOK")
	copy(header[64:68], "MOBI")
	binary.BigEndian.PutUint16(header[76:], uint16(len(records)))
	buf.Write(header)

	offset := 78 + len(records)*8
	for _, rec := range records {
		entry := make([]byte, 8)
		binary.BigEndian.PutUint32(entry, uint32(offset))
		buf.Write(entry)
		offset += len(rec)
	}
	for _, rec := range records {
		buf.Write(rec)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const testMarkup = `<html><head><title>Ignored</title></head><body>` +
	`<h1>The First Chapter</h1>` +
	`<p>` + `Sentences that go on for long enough to wrap. ` +
	`Sentences that go on for long enough to wrap. ` +
	`Sentences that go on for long enough to wrap. ` +
	`Sentences that go on for long enough to wrap.</p>` +
	`<p>A second paragraph with <b>bold</b> and <i>italic</i> words in it.</p>` +
	`</body></html>`

func TestConvertEndToEnd(t *testing.T) {
	dir := t.TempDir()
	input := writeTestBook(t, dir, "Sample Book.mobi", "Sample Book", testMarkup)

	opts, err := presets.Default().Options()
	if err != nil {
		t.Fatal(err)
	}
	opts.Cover = false

	var stages []Stage
	res, err := Convert(context.Background(), Options{Input: input, PDF: opts}, func(p Progress) {
		if len(stages) == 0 || stages[len(stages)-1] != p.Stage {
			stages = append(stages, p.Stage)
		}
	})
	if err != nil {
		t.Fatalf("Convert returned %v", err)
	}

	if want := filepath.Join(dir, "Sample Book.pdf"); res.OutputPath != want {
		t.Errorf("wrote %q, want %q", res.OutputPath, want)
	}
	if res.Pages < 1 {
		t.Errorf("produced %d pages", res.Pages)
	}
	if res.Headings != 1 {
		t.Errorf("recorded %d bookmarks, want 1", res.Headings)
	}
	if res.Book == nil || res.Book.Title != "Sample Book" {
		t.Errorf("the book metadata did not survive: %+v", res.Book)
	}
	if res.OutputSize == 0 {
		t.Error("the output size was not recorded")
	}
	if res.Elapsed <= 0 {
		t.Error("the elapsed time was not recorded")
	}

	data, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatalf("the PDF was not written: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Error("the output is not a PDF")
	}

	if len(stages) < 3 || stages[0] != StageRead || stages[len(stages)-1] != StageDone {
		t.Errorf("progress stages were %v", stages)
	}
}

func TestConvertLeavesNoTemporaryFileBehind(t *testing.T) {
	dir := t.TempDir()
	input := writeTestBook(t, dir, "book.mobi", "Book", testMarkup)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Convert(ctx, Options{Input: input, PDF: mustOptions(t)}, nil); err == nil {
		t.Fatal("Convert ignored a cancelled context")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".pdf") {
			t.Errorf("a partial PDF was left behind: %s", e.Name())
		}
	}
}

func TestConvertRejectsUnreadableInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.azw3")
	if err := os.WriteFile(path, []byte("this is not a Kindle book at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(context.Background(), Options{Input: path, PDF: mustOptions(t)}, nil)
	if err == nil {
		t.Fatal("Convert accepted a file that is not a book")
	}
	if !strings.Contains(err.Error(), "broken.azw3") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// mustOptions returns the default layout, failing the test if it cannot be
// built.
func mustOptions(t *testing.T) pdfout.Options {
	t.Helper()
	opts, err := presets.Default().Options()
	if err != nil {
		t.Fatalf("the default preset is broken: %v", err)
	}
	opts.Cover = false
	return opts
}
