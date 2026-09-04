package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

func probeCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:               "probe <book>",
		Short:             "Inspect a Kindle book without converting it",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBookFiles,
		RunE: func(cmd *cobra.Command, args []string) error {
			book, err := ebook.Open(args[0])
			if err != nil {
				return err
			}
			doc := htmldoc.ParseWithCSS(book.HTML, htmldoc.ParseCSS(book.Flows...))

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			row := func(label, value string) {
				if value != "" {
					_, _ = fmt.Fprintf(w, "%s\t%s\n", label, value)
				}
			}
			row("Title", book.Title)
			row("Author", book.AuthorLine())
			row("Publisher", book.Publisher)
			row("Published", book.Published)
			row("Language", book.Language)
			row("ISBN", book.ISBN)
			row("ASIN", book.ASIN)
			row("Subjects", strings.Join(book.Subjects, ", "))
			row("Format", book.Format)
			row("Compression", book.Compression)
			row("Encoding", book.Encoding)
			row("File size", humanBytes(book.FileSize))
			row("Text", fmt.Sprintf("%s of markup", humanBytes(int64(book.TextBytes))))
			row("Stylesheets", fmt.Sprintf("%d", len(book.Flows)))
			row("Images", fmt.Sprintf("%d embedded", len(book.Resources)))
			row("Cover", map[bool]string{true: "yes", false: "no"}[book.Cover != nil])

			counts := map[htmldoc.Kind]int{}
			for _, b := range doc.Blocks {
				counts[b.Kind]++
			}
			row("Blocks", fmt.Sprintf("%d (%d paragraphs, %d headings, %d images, %d breaks)",
				len(doc.Blocks), counts[htmldoc.KindParagraph], counts[htmldoc.KindHeading],
				counts[htmldoc.KindImage], counts[htmldoc.KindPageBreak]))
			if err := w.Flush(); err != nil {
				return err
			}

			if book.Description != "" {
				fmt.Printf("\n%s\n", wrapText(book.Description, 76))
			}

			if verbose {
				fmt.Println("\nOutline:")
				for _, i := range doc.Headings() {
					b := doc.Blocks[i]
					fmt.Printf("  %s%s\n", strings.Repeat("  ", b.Level-1), b.Text())
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "also print the headings found in the book")
	return cmd
}

// wrapText hard-wraps a paragraph for terminal output.
func wrapText(s string, width int) string {
	var out strings.Builder
	line := 0
	for _, word := range strings.Fields(s) {
		if line > 0 && line+1+len(word) > width {
			out.WriteByte('\n')
			line = 0
		} else if line > 0 {
			out.WriteByte(' ')
			line++
		}
		out.WriteString(word)
		line += len(word)
	}
	return out.String()
}
