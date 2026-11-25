package shirei

import (
	"image"
	"testing"
)

var black = Vec4{0, 0, 0, 1}

// lineSurfaces runs fn in a fixed 48x48 scope and returns the line surfaces it
// emitted.
func lineSurfaces(scope string, fn FrameFn) []Surface {
	ui.Host.WindowSize = Vec2{48, 48}
	ui.Host.WindowScale = 1
	ui.Host.HeadlessRender = true
	defer func() { ui.Host.HeadlessRender = false }()
	sid := softScope(scope)
	var out FrameOutputData
	for range 2 {
		out = RunFrameFn(func() {
			ModAttrs(func(a *AttrSet) { a.Animations = 0 })
			ContainerWithKey(sid, AttrSet{}, fn)
		})
	}
	var lines []Surface
	for _, s := range out.Surfaces {
		if s.LineWidth > 0 {
			lines = append(lines, s)
		}
	}
	return lines
}

// lineCanvas is a 48x48 container for lines. A container with no area is culled
// together with the lines declared in it, as it is for text.
func lineCanvas(scope string, fn func()) {
	ContainerWithKey(softScope(scope), AttrSet{
		Floats: true, MinSize: Vec2{48, 48}, MaxSize: Vec2{48, 48},
	}, fn)
}

func lightness(img *image.RGBA, x, y int) uint8 { return img.RGBAAt(x, y).R }

// A line is emitted as one surface per segment, in the container's coordinate
// space, with a padded bounding-box Rect.
func TestLineEmitsSurface(t *testing.T) {
	lines := lineSurfaces("line_emit", func() {
		ContainerWithKey(softScope("line_emit_box"), AttrSet{
			Floats: true, Float: Vec2{10, 20}, MinSize: Vec2{30, 20}, MaxSize: Vec2{30, 20},
		}, func() {
			Line(Vec2{2, 3}, Vec2{12, 13}, 4, black)
			Polyline([]Vec2{{0, 0}, {5, 0}, {5, 5}}, 2, black)
			Polyline([]Vec2{{1, 1}}, 2, black)     // too short: nothing
			Line(Vec2{0, 0}, Vec2{1, 1}, 0, black) // zero width: nothing
		})
	})
	if len(lines) != 3 {
		t.Fatalf("got %d line surfaces, want 3 (1 line + 2 polyline segments)", len(lines))
	}
	l := lines[0]
	if l.LineP0 != (Vec2{12, 23}) || l.LineP1 != (Vec2{22, 33}) || l.LineWidth != 4 {
		t.Fatalf("endpoints/width = %v %v %v", l.LineP0, l.LineP1, l.LineWidth)
	}
	// bbox of (12,23)-(22,33) padded by width/2 + AA margin = 2 + 1.
	want := Rect{Origin: Vec2{9, 20}, Size: Vec2{16, 16}}
	if l.Rect != want {
		t.Fatalf("Rect = %v, want %v", l.Rect, want)
	}
	if l.Color1 != black || l.Color2 != black {
		t.Fatalf("colors = %v %v", l.Color1, l.Color2)
	}
	if lines[1].LineP0 != (Vec2{10, 20}) || lines[2].LineP1 != (Vec2{15, 25}) {
		t.Fatalf("polyline segments = %v / %v", lines[1], lines[2])
	}
}

// Pixels on a diagonal line are fully covered, far pixels untouched, and pixels
// straddling the edge partially covered.
func TestSoftRenderLineDiagonal(t *testing.T) {
	img := softRenderImage("line_diag", 48, 48, 1, func() {
		lineCanvas("line_diag_box", func() { Line(Vec2{4, 4}, Vec2{44, 44}, 3, black) })
	})
	if v := lightness(img, 24, 24); v > 8 {
		t.Errorf("center of line: R=%d, want ~0 (covered)", v)
	}
	if v := lightness(img, 8, 8); v > 8 {
		t.Errorf("on the line near start: R=%d, want ~0", v)
	}
	if v := lightness(img, 40, 6); v != 255 {
		t.Errorf("far from line: R=%d, want 255 (untouched)", v)
	}
	if v := lightness(img, 4, 40); v != 255 {
		t.Errorf("far from line: R=%d, want 255 (untouched)", v)
	}
	// Pixel (24,26) has its center 1.41px from the line: inside the 1.5px
	// half-width edge band, so partially covered (coverage 0.59).
	if v := lightness(img, 24, 26); v <= 40 || v >= 215 {
		t.Errorf("edge pixel: R=%d, want partial coverage (40..215)", v)
	}
	// Round end cap: just past the start point still covered, well past is not.
	if v := lightness(img, 3, 3); v > 100 {
		t.Errorf("round cap pixel: R=%d, want mostly covered", v)
	}
	if v := lightness(img, 0, 0); v != 255 {
		t.Errorf("beyond the cap: R=%d, want 255", v)
	}
	// Axis-aligned hairline: a 1px stroke centered on a pixel row covers it fully.
	img = softRenderImage("line_hair", 48, 48, 1, func() {
		lineCanvas("line_hair_box", func() { Line(Vec2{4, 10.5}, Vec2{40, 10.5}, 1, black) })
	})
	if v := lightness(img, 20, 10); v > 8 {
		t.Errorf("hairline row: R=%d, want ~0", v)
	}
	if v := lightness(img, 20, 12); v != 255 {
		t.Errorf("row off the hairline: R=%d, want 255", v)
	}
	// A line thinner than one device pixel is widened to one device pixel.
	img = softRenderImage("line_thin", 48, 48, 1, func() {
		lineCanvas("line_thin_box", func() { Line(Vec2{4, 10.5}, Vec2{40, 10.5}, 0.1, black) })
	})
	if v := lightness(img, 20, 10); v > 8 {
		t.Errorf("sub-pixel stroke row: R=%d, want ~0 (widened to 1px)", v)
	}
}

// Lines are clipped by their container, including a rounded clip, and follow
// transparency groups.
func TestSoftRenderLineClipAndAlpha(t *testing.T) {
	img := softRenderImage("line_clip", 48, 48, 1, func() {
		ContainerWithKey(softScope("line_clip_box"), AttrSet{
			Floats: true, Float: Vec2{10, 10}, MinSize: Vec2{28, 28}, MaxSize: Vec2{28, 28},
			Clip: true,
		}, func() {
			Line(Vec2{-10, 14}, Vec2{40, 14}, 4, black) // spills past both sides
		})
	})
	if v := lightness(img, 24, 24); v > 8 {
		t.Errorf("inside clip: R=%d, want ~0", v)
	}
	if v := lightness(img, 5, 24); v != 255 {
		t.Errorf("left of clip: R=%d, want 255 (clipped)", v)
	}
	if v := lightness(img, 42, 24); v != 255 {
		t.Errorf("right of clip: R=%d, want 255 (clipped)", v)
	}

	img = softRenderImage("line_round_clip", 48, 48, 1, func() {
		ContainerWithKey(softScope("line_round_clip_box"), AttrSet{
			Floats: true, Float: Vec2{4, 4}, MinSize: Vec2{40, 40}, MaxSize: Vec2{40, 40},
			Corners: N4(20), Clip: true,
		}, func() {
			Line(Vec2{-4, -4}, Vec2{44, 44}, 8, black) // runs through the cut corners
		})
	})
	if v := lightness(img, 24, 24); v > 8 {
		t.Errorf("round clip center: R=%d, want ~0", v)
	}
	if v := lightness(img, 5, 5); v != 255 {
		t.Errorf("round clip corner: R=%d, want 255 (cut by the corner)", v)
	}

	img = softRenderImage("line_alpha", 48, 48, 1, func() {
		ContainerWithKey(softScope("line_alpha_box"), AttrSet{
			Floats: true, Float: Vec2{0, 0}, MinSize: Vec2{48, 48}, MaxSize: Vec2{48, 48},
			Transparency: 0.5,
		}, func() {
			Line(Vec2{4, 24}, Vec2{44, 24}, 4, black)
		})
	})
	if v := lightness(img, 24, 24); v < 110 || v > 145 {
		t.Errorf("50%% transparency group: R=%d, want ~128", v)
	}
}

// Lines paint after the container's own background and before its children, in
// declaration order.
func TestSoftRenderLineOrdering(t *testing.T) {
	img := softRenderImage("line_order", 48, 48, 1, func() {
		ContainerWithKey(softScope("line_order_box"), AttrSet{
			Floats: true, Float: Vec2{0, 0}, MinSize: Vec2{48, 48}, MaxSize: Vec2{48, 48},
			Background: Vec4{0, 0, 80, 1},
		}, func() {
			Line(Vec2{0, 12}, Vec2{48, 12}, 4, black)                     // over the background
			Line(Vec2{0, 12}, Vec2{48, 12}, 4, Vec4{0, 0, 100, 1})        // later line wins
			box(0, 30, 48, 10, AttrSet{Background: Vec4{120, 70, 45, 1}}) // child draws over lines
			Line(Vec2{0, 35}, Vec2{48, 35}, 2, black)                     // line declared after, still under the child
		})
	})
	if v := lightness(img, 24, 12); v < 250 {
		t.Errorf("later line should cover earlier one: R=%d, want white", v)
	}
	if v := lightness(img, 24, 6); v < 190 || v > 215 {
		t.Errorf("container background should show beside line: R=%d, want ~204", v)
	}
	if v := lightness(img, 24, 35); v > 60 {
		// green child (R small) paints over the black line: R of green 120/70/45 is low,
		// so compare against the line-free row instead.
		t.Errorf("child fill should cover the line: R=%d", v)
	}
	if a, b := img.RGBAAt(24, 35), img.RGBAAt(24, 32); a != b {
		t.Errorf("line leaked through child: %v vs %v", a, b)
	}
}

// A chart-like scene: gridlines, a polyline series and a dimmed one, inside a
// rounded clip. Snapshot, so changes to line geometry are reviewed visually.
func TestSoftRenderLines(t *testing.T) {
	softSnapshot(t, "lines", 160, 100, 1, func() {
		ContainerWithKey(softScope("lines_chart"), AttrSet{
			Floats: true, Float: Vec2{10, 10}, MinSize: Vec2{140, 80}, MaxSize: Vec2{140, 80},
			Background: Vec4{220, 20, 96, 1}, Corners: N4(10), Clip: true,
		}, func() {
			grid := Vec4{220, 10, 82, 1}
			for _, y := range []float32{10, 30, 50, 70} {
				Line(Vec2{0, y}, Vec2{140, y}, 1, grid)
			}
			Polyline([]Vec2{{4, 60}, {30, 40}, {55, 52}, {80, 20}, {110, 34}, {136, 12}}, 2.5, blue)
			Polyline([]Vec2{{4, 30}, {40, 36}, {70, 66}, {100, 58}, {136, 70}}, 1.5, Vec4{0, 80, 50, 1})
		})
	})
}

// The region raster cache must reproduce lines. Regions are clip push/pop
// spans; the cache rasterizes them into a local buffer at an integer device
// offset, so line endpoints must follow that offset exactly.
func TestRegionCacheMatchesInlineLines(t *testing.T) {
	card := Rect{Origin: Vec2{9.5, 7.25}, Size: Vec2{60, 40}}
	scene := []Surface{
		{Rect: Rect{Size: Vec2{100, 80}}, Color1: Vec4{0, 0, 100, 1}, Color2: Vec4{0, 0, 100, 1}},
		{Rect: card, Color1: Vec4{220, 20, 96, 1}, Color2: Vec4{220, 20, 96, 1}, Corners: N4(8), Clip: ClipPush},
		{Rect: Rect{Origin: Vec2{9.5, 7.25}, Size: Vec2{60, 40}}, LineWidth: 2.5,
			LineP0: Vec2{12, 40}, LineP1: Vec2{40, 18}, Color1: blue, Color2: blue},
		{Rect: Rect{Origin: Vec2{9.5, 7.25}, Size: Vec2{60, 40}}, LineWidth: 1,
			LineP0: Vec2{20.5, 10}, LineP1: Vec2{66, 44.5}, Color1: black, Color2: black},
		{Rect: card, Clip: ClipPop},
	}
	frames := [][]Surface{scene, scene, scene, scene}
	// Anti-aliased edges are partially transparent, so a region blit over the
	// backdrop may differ from inline compositing by 8-bit premultiplied
	// rounding (see assertCacheMatchesInline): within 1 level.
	assertCacheMatchesInline(t, frames, 200, 160, 2, 2)
	assertCacheMatchesInline(t, frames, 100, 80, 1, 2)
}
