package shirei

import "math"

// Anti-aliased line shapes.
//
// A line is not a layout element: it takes no space, has no identity and is not
// hit-tested. It is paint that belongs to the container it is declared in, in
// the same way a text run does. Declare lines in the container's body, before
// or after its children; they are painted after the container's own background
// and before its children, in declaration order, so later lines cover earlier
// ones.
//
// Coordinates are logical points relative to the container's top-left corner,
// the same space Float positions children in. Lines are clipped by the
// container's own Clip and by every ancestor clip (rectangular or rounded),
// follow transparency groups, and are ordered with the rest of the container's
// paint by the usual Z rules.
//
// A line is drawn as a capsule: the stroke of the given width around a segment,
// with round end caps and about one device pixel of anti-aliasing. Strokes
// narrower than one device pixel are widened to one device pixel, so hairlines
// stay visible.
//
// Overlap. Every segment is drawn on its own. Where two segments of a polyline
// meet, their round caps overlap, which gives round joins but also composites
// the overlap twice. With an opaque color this is invisible. With a translucent
// color (alpha below 1, or a transparency group) the join and any
// self-intersection are visibly darker. Use an opaque color, premixed against
// the background, when translucent polylines matter.

// lineAAMargin is the extra logical padding, beyond half the stroke width, that
// a line surface's Rect carries so the anti-aliased edge is never cut off.
const lineAAMargin = 1

type lineSeg struct {
	P0, P1 Vec2
	Width  f32
	Color  Vec4
}

// Line draws an anti-aliased line segment from p0 to p1 with the given stroke
// width and color (HSLA, like every other color in the API) in the current
// container. Coordinates are logical points relative to the container's
// top-left corner. See the package notes above for clipping, ordering and the
// translucent-overlap limitation.
func Line(p0, p1 Vec2, width f32, color Vec4) {
	if width <= 0 || color[3] <= 0 || !finite2(p0) || !finite2(p1) {
		return
	}
	ui.current.lines = append(ui.current.lines, lineSeg{P0: p0, P1: p1, Width: width, Color: color})
}

// Polyline draws connected anti-aliased segments through points: a Line for
// each consecutive pair. Fewer than two points draw nothing. Coordinates,
// clipping and the translucent-overlap limitation are those of Line.
func Polyline(points []Vec2, width f32, color Vec4) {
	for i := 1; i < len(points); i++ {
		Line(points[i-1], points[i], width, color)
	}
}

func finite2(p Vec2) bool {
	return !math.IsNaN(float64(p[0])) && !math.IsInf(float64(p[0]), 0) &&
		!math.IsNaN(float64(p[1])) && !math.IsInf(float64(p[1]), 0)
}

// LineHalfWidth returns the half stroke width in device pixels that backends
// rasterize for a line of the given logical width: at least half a device pixel,
// so a hairline covers one full pixel. Backends use it so every renderer agrees
// on line geometry.
func LineHalfWidth(width, scale f32) f32 {
	return max(width*scale/2, 0.5)
}

// emitLines turns the container's declared lines into surfaces, in order.
func emitLines(c *_Container) {
	for i := range c.lines {
		l := &c.lines[i]
		p0 := Vec2Add(c.resolvedOrigin, l.P0)
		p1 := Vec2Add(c.resolvedOrigin, l.P1)
		pad := l.Width/2 + lineAAMargin
		lo := Vec2{min(p0[0], p1[0]) - pad, min(p0[1], p1[1]) - pad}
		hi := Vec2{max(p0[0], p1[0]) + pad, max(p0[1], p1[1]) + pad}
		pushSurface(Surface{
			Rect:      Rect{Origin: lo, Size: Vec2{hi[0] - lo[0], hi[1] - lo[1]}},
			Color1:    l.Color,
			Color2:    l.Color,
			LineP0:    p0,
			LineP1:    p1,
			LineWidth: l.Width,
		})
	}
}
