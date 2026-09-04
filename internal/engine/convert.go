// Package engine converts Kindle books to PDF and manages batches of those
// conversions.
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
)

// Stage identifies which part of a conversion is running.
type Stage int

const (
	StageRead Stage = iota
	StageParse
	StageRender
	StageWrite
	StageDone
)

// String returns a human-readable stage name.
func (s Stage) String() string {
	switch s {
	case StageRead:
		return "Reading book"
	case StageParse:
		return "Parsing markup"
	case StageRender:
		return "Laying out pages"
	case StageWrite:
		return "Writing PDF"
	case StageDone:
		return "Done"
	}
	return "Working"
}

// Progress is an update from a running conversion.
type Progress struct {
	Stage   Stage
	Percent float64 // 0..1 across the whole conversion
	Page    int
	Blocks  int
	Block   int
}

// Options describe one conversion.
type Options struct {
	Input  string
	Output OutputOptions
	PDF    pdfout.Options
}

// Result summarises a finished conversion.
type Result struct {
	Book       *ebook.Book
	InputPath  string
	OutputPath string
	InputSize  int64
	OutputSize int64
	Pages      int
	Images     int
	Dropped    int
	Headings   int
	Blocks     int
	Font       string
	Elapsed    time.Duration
}

// SizeRatio returns the output size as a fraction of the input size.
func (r *Result) SizeRatio() float64 {
	if r.InputSize == 0 {
		return 0
	}
	return float64(r.OutputSize) / float64(r.InputSize)
}

// Convert reads a Kindle book and writes a PDF. Progress callbacks are
// optional and are made from the calling goroutine.
func Convert(ctx context.Context, opts Options, onProgress func(Progress)) (*Result, error) {
	started := time.Now()
	report := func(p Progress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	report(Progress{Stage: StageRead, Percent: 0.02})
	book, err := ebook.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	report(Progress{Stage: StageParse, Percent: 0.12})
	doc := htmldoc.ParseWithCSS(book.HTML, htmldoc.ParseCSS(book.Flows...))
	if len(doc.Blocks) == 0 {
		return nil, fmt.Errorf("%s: no readable content was found in the book", filepath.Base(opts.Input))
	}

	outPath, err := ResolveOutput(opts.Input, opts.Output)
	if err != nil {
		return nil, err
	}

	// Render to a temporary file in the destination directory, then move it
	// into place, so an interrupted run never leaves a half-written PDF.
	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".azw3topdf-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("creating temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	meta := pdfout.Meta{
		Title:     book.Title,
		Author:    book.AuthorLine(),
		Publisher: book.Publisher,
		Date:      book.Published,
	}
	if len(book.Subjects) > 0 {
		meta.Subject = book.Subjects[0]
	}

	images := imageLookup(book)

	report(Progress{Stage: StageRender, Percent: 0.15, Blocks: len(doc.Blocks)})
	res, err := pdfout.Render(ctx, tmp, doc, meta, images, opts.PDF, func(p pdfout.Progress) {
		frac := 0.0
		if p.Blocks > 0 {
			frac = float64(p.Block) / float64(p.Blocks)
		}
		report(Progress{
			Stage:   StageRender,
			Percent: 0.15 + frac*0.8,
			Page:    p.Page,
			Blocks:  p.Blocks,
			Block:   p.Block,
		})
	})
	if err != nil {
		return nil, err
	}

	report(Progress{Stage: StageWrite, Percent: 0.97, Page: res.Pages})
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := replaceFile(tmpName, outPath); err != nil {
		return nil, err
	}

	out := &Result{
		Book:       book,
		InputPath:  opts.Input,
		OutputPath: outPath,
		InputSize:  book.FileSize,
		Pages:      res.Pages,
		Images:     res.Images,
		Dropped:    res.Dropped,
		Headings:   res.Headings,
		Blocks:     len(doc.Blocks),
		Font:       res.Font,
		Elapsed:    time.Since(started),
	}
	if st, err := os.Stat(outPath); err == nil {
		out.OutputSize = st.Size()
	}
	report(Progress{Stage: StageDone, Percent: 1, Page: res.Pages})
	return out, nil
}

// imageLookup adapts a book's resources to the renderer's image callback.
func imageLookup(book *ebook.Book) pdfout.Images {
	return func(index int) (pdfout.Image, bool) {
		var res *ebook.Resource
		if index < 0 {
			res = book.Cover
		} else {
			res = book.Resources[index]
		}
		if !res.IsImage() {
			return pdfout.Image{}, false
		}
		return pdfout.Image{Data: res.Data, Kind: res.Kind}, true
	}
}

// replaceFile moves src onto dst, overwriting it. os.Rename already does this
// on POSIX; on Windows the destination has to be removed first.
func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replacing %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	return nil
}
