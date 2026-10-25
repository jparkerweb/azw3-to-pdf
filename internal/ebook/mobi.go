package ebook

import "fmt"

// decodeMOBI decodes the Palm database family: MOBI 6, KF8 / AZW3 and the
// dual-format files that carry both.
func decodeMOBI(data []byte) (*Book, error) {
	db, err := parsePalmDB(data)
	if err != nil {
		return nil, err
	}
	if db.typ != "BOOK" || (db.creator != "MOBI" && db.creator != "TEXt") {
		return nil, fmt.Errorf("not a Kindle book (Palm type %q, creator %q)", printable([]byte(db.typ)), printable([]byte(db.creator)))
	}

	h, err := parseMobiHeader(db.record(0))
	if err != nil {
		return nil, err
	}

	// A dual-format .azw3 stores the KF8 part after a BOUNDARY record; the
	// MOBI 6 part comes first. Prefer KF8 when both are present.
	if !h.isKF8() {
		if start, ok := findKF8Boundary(db); ok {
			sub := &palmDB{records: db.records[start:], typ: db.typ, creator: db.creator, name: db.name}
			if sh, err := parseMobiHeader(sub.record(0)); err == nil && sh.isKF8() {
				db, h = sub, sh
			}
		}
	}

	raw, err := decompressText(db, h)
	if err != nil {
		return nil, err
	}

	book := &Book{
		Title:       h.title,
		Format:      formatName(h),
		Compression: compressionName(h.compression),
		Encoding:    encodingName(h.codepage),
		TextBytes:   len(raw),
		Resources:   map[int]*Resource{},
	}
	applyMetadata(book, h)

	flows := splitFlows(db, h, raw)
	if len(flows) == 0 {
		return nil, fmt.Errorf("the book contains no text flow")
	}
	book.HTML = decodeText(flows[0], h.codepage)
	for _, f := range flows[1:] {
		book.Flows = append(book.Flows, decodeText(f, h.codepage))
	}

	loadResources(book, db, h)
	return book, nil
}

func applyMetadata(b *Book, h *mobiHeader) {
	e := h.exth
	if t := e.first(exthUpdatedTitle); t != "" {
		b.Title = t
	}
	if b.Title == "" {
		b.Title = "Untitled"
	}
	b.Authors = e.all(exthAuthor)
	b.Publisher = e.first(exthPublisher)
	b.Description = e.first(exthDescription)
	b.ISBN = e.first(exthISBN)
	b.Published = e.first(exthPublished)
	b.Rights = e.first(exthRights)
	b.ASIN = e.first(exthASIN)
	b.Subjects = e.all(exthSubject)
	if lang := e.first(exthLanguage); lang != "" {
		b.Language = lang
	} else {
		b.Language = localeName(h.locale)
	}
}

// splitFlows carves the decompressed text into KF8 flows using the FDST
// record. MOBI 6 files have a single flow: the whole text.
func splitFlows(db *palmDB, h *mobiHeader, raw []byte) [][]byte {
	// Without a flow table the text length is authoritative: text records are
	// padded to a fixed size, so the tail of the last one is noise.
	whole := func() [][]byte {
		if h.textLength > 0 && len(raw) > h.textLength {
			return [][]byte{raw[:h.textLength]}
		}
		return [][]byte{raw}
	}

	if !h.isKF8() || h.fdstRecord <= 0 {
		return whole()
	}
	fdst := db.record(h.fdstRecord)
	if len(fdst) < 12 || string(fdst[0:4]) != "FDST" {
		return whole()
	}
	count := int(be32(fdst, 8))
	if count <= 0 || 12+count*8 > len(fdst) {
		return whole()
	}
	var flows [][]byte
	for i := 0; i < count; i++ {
		start := int(be32(fdst, 12+i*8))
		end := int(be32(fdst, 12+i*8+4))
		if start < 0 || end > len(raw) || start > end {
			continue
		}
		flows = append(flows, raw[start:end])
	}
	if len(flows) == 0 {
		return whole()
	}
	return flows
}

// loadResources indexes the records that follow the text, which hold the
// book's images. Non-image records (FDST, FLIS, SRCS and friends) keep their
// slot so that the 1-based indices in the markup stay aligned.
func loadResources(b *Book, db *palmDB, h *mobiHeader) {
	start := h.firstResource
	if start <= 0 || start >= db.count() {
		start = h.firstNonText
	}
	if start <= 0 || start >= db.count() {
		return
	}

	for i := start; i < db.count(); i++ {
		rec := db.record(i)
		kind := sniffImage(rec)
		if kind == "" {
			continue
		}
		idx := i - start + 1
		b.Resources[idx] = &Resource{Index: idx, Kind: kind, Data: rec}
	}

	if off, ok := h.exth.number(exthCoverOffset); ok {
		if r, found := b.Resources[int(off)+1]; found {
			b.Cover = r
		}
	}
	if b.Cover == nil {
		if off, ok := h.exth.number(exthThumbOffset); ok {
			if r, found := b.Resources[int(off)+1]; found {
				b.Cover = r
			}
		}
	}
}

// findKF8Boundary locates the record index where the KF8 half of a dual-format
// file begins.
func findKF8Boundary(db *palmDB) (int, bool) {
	for i := 0; i < db.count(); i++ {
		if string(db.record(i)) == "BOUNDARY" {
			if i+1 < db.count() {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func formatName(h *mobiHeader) string {
	if h.isKF8() {
		return fmt.Sprintf("KF8 / AZW3 (MOBI v%d)", h.version)
	}
	return fmt.Sprintf("MOBI v%d", h.version)
}

func compressionName(c int) string {
	switch c {
	case compressionNone:
		return "none"
	case compressionPalmDoc:
		return "PalmDOC"
	case compressionHuffCdic:
		return "HUFF/CDIC"
	default:
		return fmt.Sprintf("unknown (%d)", c)
	}
}

func encodingName(codepage int) string {
	switch codepage {
	case 65001:
		return "UTF-8"
	case 1252:
		return "Windows-1252"
	default:
		return fmt.Sprintf("codepage %d", codepage)
	}
}

// localeName maps the MOBI locale field to a language tag. Only the low byte
// identifies the language; the high byte is a regional dialect.
func localeName(locale int) string {
	if locale == 0 {
		return ""
	}
	names := map[int]string{
		0x01: "ar", 0x02: "bg", 0x03: "ca", 0x04: "zh", 0x05: "cs", 0x06: "da",
		0x07: "de", 0x08: "el", 0x09: "en", 0x0a: "es", 0x0b: "fi", 0x0c: "fr",
		0x0d: "he", 0x0e: "hu", 0x0f: "is", 0x10: "it", 0x11: "ja", 0x12: "ko",
		0x13: "nl", 0x14: "no", 0x15: "pl", 0x16: "pt", 0x18: "ro", 0x19: "ru",
		0x1a: "hr", 0x1b: "sk", 0x1d: "sv", 0x1e: "th", 0x1f: "tr", 0x22: "uk",
	}
	if name, ok := names[locale&0xff]; ok {
		return name
	}
	return fmt.Sprintf("locale %d", locale)
}
