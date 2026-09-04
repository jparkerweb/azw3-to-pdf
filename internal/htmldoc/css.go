package htmldoc

import (
	"strconv"
	"strings"
)

// Props are the handful of CSS declarations that change how a block of book
// text is laid out. Everything else in a Kindle stylesheet is either
// decoration this renderer does not attempt (colours, backgrounds) or is
// already implied by the markup.
type Props struct {
	Align      Align
	Scale      float64 // font-size as a multiple of the body size, 0 when unset
	Bold       bool
	Italic     bool
	Mono       bool
	Block      bool // display:block on an otherwise inline element
	MarginLeft float64
	Indent     float64 // text-indent, negative for a hanging indent
	BreakStart bool    // page-break-before: always
	BreakEnd   bool    // page-break-after: always
	Small      bool
}

// merge layers other on top of p, with other winning wherever it is set.
func (p Props) merge(other Props) Props {
	out := p
	if other.Align != AlignDefault {
		out.Align = other.Align
	}
	if other.Scale != 0 {
		out.Scale = other.Scale
	}
	out.Bold = out.Bold || other.Bold
	out.Italic = out.Italic || other.Italic
	out.Mono = out.Mono || other.Mono
	out.Block = out.Block || other.Block
	out.Small = out.Small || other.Small
	out.BreakStart = out.BreakStart || other.BreakStart
	out.BreakEnd = out.BreakEnd || other.BreakEnd
	if other.MarginLeft != 0 {
		out.MarginLeft = other.MarginLeft
	}
	if other.Indent != 0 {
		out.Indent = other.Indent
	}
	return out
}

// style converts the inline-affecting declarations into a Style set.
func (p Props) style() Style {
	var s Style
	if p.Bold {
		s |= StyleBold
	}
	if p.Italic {
		s |= StyleItalic
	}
	if p.Mono {
		s |= StyleMono
	}
	return s
}

// Stylesheet maps class and tag names to their declarations.
type Stylesheet struct {
	classes map[string]Props
	tags    map[string]Props
}

// Lookup resolves the properties for an element with the given tag and class
// attribute. Classes win over bare tag rules, matching CSS specificity closely
// enough for book markup.
func (s *Stylesheet) Lookup(tag, class string) Props {
	if s == nil {
		return Props{}
	}
	out := s.tags[tag]
	for _, c := range strings.Fields(class) {
		if p, ok := s.classes[strings.ToLower(c)]; ok {
			out = out.merge(p)
		}
	}
	return out
}

// ParseCSS reads the stylesheet flows of a book. It is a deliberately small
// parser: rule blocks with comma-separated simple selectors, nothing else.
func ParseCSS(sheets ...string) *Stylesheet {
	s := &Stylesheet{classes: map[string]Props{}, tags: map[string]Props{}}
	for _, sheet := range sheets {
		s.parse(sheet)
	}
	return s
}

func (s *Stylesheet) parse(sheet string) {
	sheet = stripComments(sheet)
	for len(sheet) > 0 {
		open := strings.IndexByte(sheet, '{')
		if open < 0 {
			return
		}
		close := strings.IndexByte(sheet[open:], '}')
		if close < 0 {
			return
		}
		close += open

		selectors := strings.TrimSpace(sheet[:open])
		body := sheet[open+1 : close]
		sheet = sheet[close+1:]

		// Skip at-rules such as @page or @media, whose body is not a
		// declaration list we can use.
		if strings.HasPrefix(selectors, "@") {
			continue
		}
		props := parseDeclarations(body)
		for _, sel := range strings.Split(selectors, ",") {
			s.add(strings.TrimSpace(sel), props)
		}
	}
}

func (s *Stylesheet) add(selector string, props Props) {
	// Descendant, pseudo and attribute selectors are out of scope; book
	// stylesheets are overwhelmingly "tag.class" rules.
	selector = strings.ToLower(selector)
	if selector == "" || strings.ContainsAny(selector, " >+~:[*#") {
		return
	}
	if i := strings.IndexByte(selector, '.'); i >= 0 {
		class := selector[i+1:]
		if class == "" {
			return
		}
		s.classes[class] = s.classes[class].merge(props)
		return
	}
	s.tags[selector] = s.tags[selector].merge(props)
}

func parseDeclarations(body string) Props {
	var p Props
	for _, decl := range strings.Split(body, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		switch name {
		case "text-align":
			switch value {
			case "center":
				p.Align = AlignCenter
			case "right":
				p.Align = AlignRight
			case "left":
				p.Align = AlignLeft
			}
		case "font-size":
			if v, ok := parseSize(value); ok {
				p.Scale = v
				p.Small = v < 0.92
			}
		case "font-weight":
			if value == "bold" || value == "bolder" || atLeast(value, 600) {
				p.Bold = true
			}
		case "font-style":
			if value == "italic" || value == "oblique" {
				p.Italic = true
			}
		case "font-family":
			if strings.Contains(value, "monospace") || strings.Contains(value, "courier") {
				p.Mono = true
			}
		case "display":
			if value == "block" {
				p.Block = true
			}
		case "margin-left", "padding-left":
			if v, ok := parseLength(value); ok {
				p.MarginLeft = v
			}
		case "text-indent":
			if v, ok := parseLength(value); ok {
				p.Indent = v
			}
		case "page-break-before":
			if value == "always" || value == "left" || value == "right" {
				p.BreakStart = true
			}
		case "page-break-after":
			if value == "always" {
				p.BreakEnd = true
			}
		}
	}
	return p
}

// parseSize converts a font-size to a multiple of the surrounding size.
func parseSize(v string) (float64, bool) {
	switch v {
	case "xx-small":
		return 0.6, true
	case "x-small":
		return 0.75, true
	case "small", "smaller":
		return 0.85, true
	case "medium":
		return 1, true
	case "large", "larger":
		return 1.2, true
	case "x-large":
		return 1.5, true
	case "xx-large":
		return 2, true
	}
	switch {
	case strings.HasSuffix(v, "%"):
		if f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64); err == nil {
			return clampScale(f / 100), true
		}
	case strings.HasSuffix(v, "em"), strings.HasSuffix(v, "rem"):
		if f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(v, "em"), "r"), 64); err == nil {
			return clampScale(f), true
		}
	case strings.HasSuffix(v, "pt"), strings.HasSuffix(v, "px"):
		// Absolute sizes are relative to a nominal 12pt/16px body.
		base := 12.0
		if strings.HasSuffix(v, "px") {
			base = 16
		}
		if f, err := strconv.ParseFloat(v[:len(v)-2], 64); err == nil && f > 0 {
			return clampScale(f / base), true
		}
	}
	return 0, false
}

func clampScale(f float64) float64 {
	if f < 0.5 {
		return 0.5
	}
	if f > 3.5 {
		return 3.5
	}
	return f
}

// parseLength returns a length in em, which is the only unit the layout uses.
func parseLength(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	switch {
	case strings.HasSuffix(v, "em"), strings.HasSuffix(v, "rem"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(v, "em"), "r"), 64)
		return f, err == nil
	case strings.HasSuffix(v, "px"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
		return f / 16, err == nil
	case strings.HasSuffix(v, "pt"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(v, "pt"), 64)
		return f / 12, err == nil
	case strings.HasSuffix(v, "%"):
		f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64)
		return f / 25, err == nil // rough: percentage of the text column
	case v == "0":
		return 0, true
	}
	return 0, false
}

func atLeast(value string, n int) bool {
	f, err := strconv.Atoi(value)
	return err == nil && f >= n
}

func stripComments(s string) string {
	for {
		i := strings.Index(s, "/*")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i+2:], "*/")
		if j < 0 {
			return s[:i]
		}
		s = s[:i] + s[i+2+j+2:]
	}
}
