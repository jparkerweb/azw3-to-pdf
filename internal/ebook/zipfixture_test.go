package ebook

import (
	"archive/zip"
	"bytes"
	"testing"
)

// zipEntry is one file in a test archive. Store writes it uncompressed, which
// EPUB requires for its mimetype entry.
type zipEntry struct {
	Name  string
	Data  []byte
	Store bool
}

// buildZip writes entries into an in-memory archive, in order. Every reader
// test builds its fixtures with it.
func buildZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		method := zip.Deflate
		if e.Store {
			method = zip.Store
		}
		f, err := w.CreateHeader(&zip.FileHeader{Name: e.Name, Method: method})
		if err != nil {
			t.Fatalf("creating %s: %v", e.Name, err)
		}
		if _, err := f.Write(e.Data); err != nil {
			t.Fatalf("writing %s: %v", e.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
