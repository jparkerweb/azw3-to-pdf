package htmldoc

// Font-size scaling lives here and nowhere else. The stylesheet parser stores
// a block's raw CSS size (Block.Scale), and the renderer asks EffectiveScale
// for the multiple of the body size to draw it at.
//
// None of the constants below has a recorded reason in the git history; they
// are kept so the output does not change.
const (
	// Stylesheet sizes are limited to this range. Origin unknown, kept for
	// output parity.
	minCSSScale = 0.5
	maxCSSScale = 3.5

	// dampen shrinks the part of a size above the body size, so a very large
	// heading still fits a narrow page. Origin of 0.62 unknown, kept for
	// output parity.
	dampen = 0.62

	// maxScale is the largest multiple of the body size any block gets.
	// Origin unknown, kept for output parity.
	maxScale = 2.4
)

// headingScales are the default sizes for headings with no stylesheet size,
// as a multiple of the body size, indexed by level 1-6. Origin unknown, kept
// for output parity.
var headingScales = [7]float64{1, 1.75, 1.45, 1.28, 1.15, 1.06, 1.0}

// EffectiveScale returns the multiple of the body font size a block is drawn
// at. cssScale is the block's raw stylesheet size (0 when it has none); level
// matters only for headings, where an out-of-range level is treated as 3.
// A heading is never smaller than its default size for the level.
func EffectiveScale(cssScale float64, kind Kind, level int) float64 {
	scale := 1.0
	if cssScale > 0 {
		scale = min(max(cssScale, minCSSScale), maxCSSScale)
		if scale > 1 {
			scale = 1 + (scale-1)*dampen
		}
		scale = min(scale, maxScale)
	}

	if kind == KindHeading {
		if level < 1 || level > 6 {
			level = 3
		}
		if cssScale <= 0 {
			return headingScales[level]
		}
		return max(scale, headingScales[level])
	}
	return scale
}
