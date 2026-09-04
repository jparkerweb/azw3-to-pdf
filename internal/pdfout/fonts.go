package pdfout

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Font family names registered inside the PDF.
const (
	familyBody = "body"
	familyMono = "mono"
)

// FontChoice names the typeface used for body text.
type FontChoice string

const (
	FontSerif FontChoice = "serif"
	FontSans  FontChoice = "sans"
	FontMono  FontChoice = "mono"
)

// faces holds the four style variants of one family as raw TTF data.
type faces struct {
	name       string
	regular    []byte
	bold       []byte
	italic     []byte
	boldItalic []byte
}

// fill copies the regular face into any variant the system did not provide,
// so that requesting bold or italic never fails.
func (f *faces) fill() {
	if f.bold == nil {
		f.bold = f.regular
	}
	if f.italic == nil {
		f.italic = f.regular
	}
	if f.boldItalic == nil {
		if f.bold != nil {
			f.boldItalic = f.bold
		} else {
			f.boldItalic = f.regular
		}
	}
}

// candidate is one system font family, listed by file name across platforms.
type candidate struct {
	name    string
	files   [4]string // regular, bold, italic, bold-italic
	dirHint string
}

func systemFontDirs() []string {
	switch runtime.GOOS {
	case "windows":
		dirs := []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	case "darwin":
		home, _ := os.UserHomeDir()
		return []string{
			"/System/Library/Fonts/Supplemental",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		}
	default:
		return []string{
			"/usr/share/fonts/truetype/dejavu",
			"/usr/share/fonts/truetype/liberation",
			"/usr/share/fonts/truetype/liberation2",
			"/usr/share/fonts/TTF",
			"/usr/share/fonts/truetype",
			"/usr/local/share/fonts",
		}
	}
}

var serifCandidates = []candidate{
	{name: "Georgia", files: [4]string{"georgia.ttf", "georgiab.ttf", "georgiai.ttf", "georgiaz.ttf"}},
	{name: "Times New Roman", files: [4]string{"times.ttf", "timesbd.ttf", "timesi.ttf", "timesbi.ttf"}},
	{name: "Times New Roman", files: [4]string{"Times New Roman.ttf", "Times New Roman Bold.ttf", "Times New Roman Italic.ttf", "Times New Roman Bold Italic.ttf"}},
	{name: "DejaVu Serif", files: [4]string{"DejaVuSerif.ttf", "DejaVuSerif-Bold.ttf", "DejaVuSerif-Italic.ttf", "DejaVuSerif-BoldItalic.ttf"}},
	{name: "Liberation Serif", files: [4]string{"LiberationSerif-Regular.ttf", "LiberationSerif-Bold.ttf", "LiberationSerif-Italic.ttf", "LiberationSerif-BoldItalic.ttf"}},
}

var sansCandidates = []candidate{
	{name: "Segoe UI", files: [4]string{"segoeui.ttf", "segoeuib.ttf", "segoeuii.ttf", "segoeuiz.ttf"}},
	{name: "Arial", files: [4]string{"arial.ttf", "arialbd.ttf", "ariali.ttf", "arialbi.ttf"}},
	{name: "Helvetica", files: [4]string{"Helvetica.ttf", "Helvetica-Bold.ttf", "Helvetica-Oblique.ttf", "Helvetica-BoldOblique.ttf"}},
	{name: "DejaVu Sans", files: [4]string{"DejaVuSans.ttf", "DejaVuSans-Bold.ttf", "DejaVuSans-Oblique.ttf", "DejaVuSans-BoldOblique.ttf"}},
	{name: "Liberation Sans", files: [4]string{"LiberationSans-Regular.ttf", "LiberationSans-Bold.ttf", "LiberationSans-Italic.ttf", "LiberationSans-BoldItalic.ttf"}},
}

var monoCandidates = []candidate{
	{name: "Consolas", files: [4]string{"consola.ttf", "consolab.ttf", "consolai.ttf", "consolaz.ttf"}},
	{name: "Menlo", files: [4]string{"Menlo.ttc", "", "", ""}},
	{name: "DejaVu Sans Mono", files: [4]string{"DejaVuSansMono.ttf", "DejaVuSansMono-Bold.ttf", "DejaVuSansMono-Oblique.ttf", "DejaVuSansMono-BoldOblique.ttf"}},
	{name: "Courier New", files: [4]string{"cour.ttf", "courbd.ttf", "couri.ttf", "courbi.ttf"}},
}

// goFallback is always available: the Go fonts ship inside the binary.
func goFallback(mono bool) faces {
	if mono {
		return faces{name: "Go Mono (embedded)", regular: gomono.TTF, bold: gomonobold.TTF}
	}
	return faces{
		name:       "Go (embedded)",
		regular:    goregular.TTF,
		bold:       gobold.TTF,
		italic:     goitalic.TTF,
		boldItalic: gobolditalic.TTF,
	}
}

// resolveFaces picks the best available typeface for a choice. A choice that
// is not one of the known names is treated as a path to a .ttf file.
func resolveFaces(choice string) faces {
	var list []candidate
	switch FontChoice(strings.ToLower(strings.TrimSpace(choice))) {
	case FontSerif, "":
		list = serifCandidates
	case FontSans:
		list = sansCandidates
	case FontMono:
		list = monoCandidates
	default:
		if f, ok := facesFromPath(choice); ok {
			return f
		}
		list = serifCandidates
	}

	dirs := systemFontDirs()
	for _, c := range list {
		for _, dir := range dirs {
			regular := filepath.Join(dir, c.files[0])
			data, err := os.ReadFile(regular)
			if err != nil || !usableTTF(data) {
				continue
			}
			f := faces{name: c.name, regular: data}
			for i, dst := range []*[]byte{nil, &f.bold, &f.italic, &f.boldItalic} {
				if i == 0 || c.files[i] == "" {
					continue
				}
				if b, err := os.ReadFile(filepath.Join(dir, c.files[i])); err == nil && usableTTF(b) {
					*dst = b
				}
			}
			f.fill()
			return f
		}
	}

	f := goFallback(FontChoice(strings.ToLower(choice)) == FontMono)
	f.fill()
	return f
}

func facesFromPath(path string) (faces, bool) {
	data, err := os.ReadFile(path)
	if err != nil || !usableTTF(data) {
		return faces{}, false
	}
	f := faces{name: filepath.Base(path), regular: data}

	// Pick up the obvious sibling variants when they exist alongside the file.
	base := strings.TrimSuffix(path, filepath.Ext(path))
	for _, v := range []struct {
		suffixes []string
		dst      *[]byte
	}{
		{[]string{"-Bold", "bd", "b", "-bold"}, &f.bold},
		{[]string{"-Italic", "i", "-italic", "-Oblique"}, &f.italic},
		{[]string{"-BoldItalic", "z", "bi", "-bolditalic"}, &f.boldItalic},
	} {
		for _, sfx := range v.suffixes {
			if b, err := os.ReadFile(base + sfx + ".ttf"); err == nil && usableTTF(b) {
				*v.dst = b
				break
			}
		}
	}
	f.fill()
	return f, true
}

// usableTTF rejects font containers gopdf cannot parse (TrueType collections
// and CFF/OpenType outlines).
func usableTTF(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	switch string(data[0:4]) {
	case "\x00\x01\x00\x00", "true", "ttcf":
		return string(data[0:4]) != "ttcf"
	case "OTTO":
		return false
	}
	return false
}

// fontSet is the set of families registered on one document.
type fontSet struct {
	body faces
	mono faces
}

func loadFonts(pdf *gopdf.GoPdf, choice string) (fontSet, error) {
	set := fontSet{body: resolveFaces(choice), mono: resolveFaces(string(FontMono))}

	register := func(family string, f faces) error {
		variants := []struct {
			data  []byte
			style int
		}{
			{f.regular, gopdf.Regular},
			{f.bold, gopdf.Bold},
			{f.italic, gopdf.Italic},
			{f.boldItalic, gopdf.Bold | gopdf.Italic},
		}
		for _, v := range variants {
			if v.data == nil {
				continue
			}
			opt := gopdf.TtfOption{Style: v.style}
			if err := pdf.AddTTFFontDataWithOption(family, v.data, opt); err != nil {
				return fmt.Errorf("loading %s font %q: %w", family, f.name, err)
			}
		}
		return nil
	}

	if err := register(familyBody, set.body); err != nil {
		// Fall back to the embedded faces rather than failing the conversion.
		set.body = goFallback(false)
		set.body.fill()
		if err2 := register(familyBody, set.body); err2 != nil {
			return set, err
		}
	}
	if err := register(familyMono, set.mono); err != nil {
		set.mono = goFallback(true)
		set.mono.fill()
		if err2 := register(familyMono, set.mono); err2 != nil {
			return set, err
		}
	}
	return set, nil
}

// Name reports the typeface actually used for body text.
func (f fontSet) Name() string { return f.body.name }
