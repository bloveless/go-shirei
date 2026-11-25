//go:build darwin || linux || android || windows || js

package gpurender

import (
	"image"
	"testing"
	"unsafe"

	"go.hasen.dev/shirei"
)

func TestQuadLayout(t *testing.T) {
	if unsafe.Sizeof(Quad{}) != 64 {
		t.Fatalf("Quad size %d want 64 (C GPUQuad)", unsafe.Sizeof(Quad{}))
	}
	if unsafe.Sizeof(Batch{}) != 168 {
		t.Fatalf("Batch size %d want 168 (C GPUBatch)", unsafe.Sizeof(Batch{}))
	}
}

func TestBuildFillAndClip(t *testing.T) {
	var b builder
	bg := shirei.Vec4{0, 0, 50, 1}
	child := shirei.Vec4{0, 100, 50, 1}
	surfaces := []shirei.Surface{
		{
			Rect:   shirei.Rect{Size: shirei.Vec2{100, 80}},
			Color1: bg, Color2: bg,
			Clip: shirei.ClipPush,
		},
		{
			Rect:   shirei.Rect{Origin: shirei.Vec2{-10, -10}, Size: shirei.Vec2{50, 50}},
			Color1: child, Color2: child,
		},
		{
			Rect:   shirei.Rect{Size: shirei.Vec2{100, 80}},
			Color1: bg, Color2: bg,
			Stroke: 1,
			Clip:   shirei.ClipPop,
		},
	}
	b.build(surfaces, nil, 1, 100, 80)
	if len(b.quads) < 2 {
		t.Fatalf("quads=%d want at least fill+child", len(b.quads))
	}
	if b.quads[0].W != 100 || b.quads[0].H != 80 {
		t.Fatalf("bg dest = %v", b.quads[0])
	}
	// child is emitted while clip is the 100x80 push; scissor should be full
	// viewport intersected with the push rect (the whole target).
	if len(b.batches) < 1 {
		t.Fatal("no batches")
	}
	// After ClipPush the second surface uses the 100x80 clip. Border after pop
	// uses the full viewport — that should start a new batch if the clip differs,
	// but both are 100x80 so they may merge. Just check we got a clipped child.
	if b.quads[1].X != -10 || b.quads[1].Y != -10 {
		t.Fatalf("child dest origin = %v,%v", b.quads[1].X, b.quads[1].Y)
	}
}

func TestUnknownGlyphAndImageSkipped(t *testing.T) {
	var b builder
	bg := shirei.Vec4{0, 0, 50, 1}
	surfaces := []shirei.Surface{
		{Rect: shirei.Rect{Size: shirei.Vec2{20, 20}}, Color1: bg, Color2: bg},
		{Rect: shirei.Rect{Size: shirei.Vec2{20, 20}}, FontId: 1, GlyphId: 1, Color1: bg, Color2: bg},
		{Rect: shirei.Rect{Size: shirei.Vec2{20, 20}}, ImageId: 1, Color1: bg, Color2: bg},
	}
	b.build(surfaces, nil, 1, 20, 20)
	if len(b.quads) != 1 {
		t.Fatalf("quads=%d want 1 (glyph+image skipped)", len(b.quads))
	}
}

func TestShelfAtlasPack(t *testing.T) {
	a := shelfAtlas{w: 16, h: 16}
	x, y, ok := a.alloc(6, 6) // 8×8 with pad
	if !ok || x != 0 || y != 0 {
		t.Fatalf("first alloc %d,%d ok=%v", x, y, ok)
	}
	x, y, ok = a.alloc(6, 6)
	if !ok || x != 8 || y != 0 {
		t.Fatalf("second alloc %d,%d ok=%v", x, y, ok)
	}
	x, y, ok = a.alloc(6, 6)
	if !ok || x != 0 || y != 8 {
		t.Fatalf("third alloc %d,%d ok=%v", x, y, ok)
	}
	x, y, ok = a.alloc(6, 6)
	if !ok || x != 8 || y != 8 {
		t.Fatalf("fourth alloc %d,%d ok=%v", x, y, ok)
	}
	_, _, ok = a.alloc(6, 6)
	if ok {
		t.Fatal("fifth 8×8 should not fit on 16×16")
	}
}

func TestDecomposeRoundedHasCorners(t *testing.T) {
	var b builder
	b.scale = 1
	dr := image.Rect(0, 0, 40, 40)
	ni, nc := b.decompose(dr, shirei.Vec4{8, 8, 8, 8})
	if nc != 4 {
		t.Fatalf("corners=%d want 4", nc)
	}
	if ni < 1 {
		t.Fatalf("interiors=%d", ni)
	}
	if b.cornerBuf[0].rect != image.Rect(0, 0, 8, 8) {
		t.Fatalf("TL corner %v", b.cornerBuf[0].rect)
	}
	if !b.cornerBuf[1].flipH || b.cornerBuf[1].flipV {
		t.Fatalf("TR flips h=%v v=%v", b.cornerBuf[1].flipH, b.cornerBuf[1].flipV)
	}
}

func TestGlyphRunDoesNotFill(t *testing.T) {
	var b builder
	bg := shirei.Vec4{0, 0, 50, 1}
	surfaces := []shirei.Surface{
		{Rect: shirei.Rect{Size: shirei.Vec2{20, 20}}, Color1: bg, Color2: bg},
		{
			Rect:          shirei.Rect{Size: shirei.Vec2{40, 16}},
			Color1:        bg,
			Color2:        bg,
			GlyphRunCount: 2,
		},
	}
	runs := []shirei.GlyphRun{
		{Rect: shirei.Rect{Size: shirei.Vec2{8, 16}}, FontId: 1, GlyphId: 1, Color: bg},
		{Rect: shirei.Rect{Origin: shirei.Vec2{8, 0}, Size: shirei.Vec2{8, 16}}, FontId: 1, GlyphId: 2, Color: bg},
	}
	b.build(surfaces, runs, 1, 20, 20)
	if len(b.quads) != 1 {
		t.Fatalf("quads=%d want 1 (run glyphs uncached, no fill)", len(b.quads))
	}
}

func TestSkipClearOpaqueCover(t *testing.T) {
	var b builder
	bg := shirei.Vec4{0, 0, 50, 1}
	surfaces := []shirei.Surface{
		{Rect: shirei.Rect{Size: shirei.Vec2{10, 10}}, Color1: bg, Color2: bg},
	}
	b.build(surfaces, nil, 1, 10, 10)
	if !b.skipClear {
		t.Fatal("expected skipClear for opaque fullscreen fill")
	}
}

func TestExternalImageImportCachedByGeneration(t *testing.T) {
	if gpu.image == nil {
		gpu.init()
	}
	oldExt := gpuExternalImage
	imports := 0
	gpuExternalImage = func(id uint32, frame *shirei.ExternalFrame, w, h int) bool {
		imports++
		return true
	}
	defer func() { gpuExternalImage = oldExt }()

	const key = "test-drawlist-external-cache"
	defer shirei.ReleaseImage(key)

	id := shirei.UseExternalImage(key, shirei.ExternalFrame{
		Kind:   shirei.ExternalIOSurfaceBGRA,
		Handle: 1, Width: 32, Height: 18,
	})
	defer delete(gpu.image, id)

	surfaces := []shirei.Surface{
		{Rect: shirei.Rect{Size: shirei.Vec2{32, 18}}, ImageId: id},
	}
	var b builder
	b.build(surfaces, nil, 1, 64, 64)
	b.build(surfaces, nil, 1, 64, 64)
	if imports != 1 {
		t.Fatalf("imports = %d, want 1 when generation is unchanged", imports)
	}

	shirei.UseExternalImage(key, shirei.ExternalFrame{
		Kind:   shirei.ExternalIOSurfaceBGRA,
		Handle: 2, Width: 32, Height: 18,
	})
	b.build(surfaces, nil, 1, 64, 64)
	if imports != 2 {
		t.Fatalf("imports = %d, want 2 after generation bump", imports)
	}
}

func lineSurface(p0, p1 shirei.Vec2, width float32, c shirei.Vec4) shirei.Surface {
	pad := width/2 + 1
	return shirei.Surface{
		Rect: shirei.Rect{
			Origin: shirei.Vec2{min(p0[0], p1[0]) - pad, min(p0[1], p1[1]) - pad},
			Size:   shirei.Vec2{abs32(p1[0]-p0[0]) + 2*pad, abs32(p1[1]-p0[1]) + 2*pad},
		},
		Color1: c, Color2: c,
		LineP0: p0, LineP1: p1, LineWidth: width,
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// A line is one quad over the padded device bounding box in texLine mode, with
// the endpoints packed so the interpolated UV is the position relative to P0.
func TestBuildLineQuad(t *testing.T) {
	var b builder
	black := shirei.Vec4{0, 0, 0, 1}
	b.build([]shirei.Surface{lineSurface(shirei.Vec2{10, 20}, shirei.Vec2{30, 10}, 4, black)}, nil, 2, 100, 100)
	if len(b.quads) != 1 || len(b.batches) != 1 {
		t.Fatalf("quads=%d batches=%d want 1,1", len(b.quads), len(b.batches))
	}
	if k := b.batches[0].TexKind; k != texLine {
		t.Fatalf("TexKind=%d want texLine", k)
	}
	q := b.quads[0]
	// device endpoints (20,40) and (60,20), half width 4 (width 4 * scale 2 / 2),
	// bbox padded by half width + 1: x 15..65, y 15..45.
	if q.X != 15 || q.Y != 15 || q.W != 50 || q.H != 30 {
		t.Fatalf("bbox = %v,%v %vx%v", q.X, q.Y, q.W, q.H)
	}
	// UV at the quad's top-left is (bbox origin - P0); UW/VH span the quad, so
	// UV at any point is that point minus P0.
	if q.U != 15-20 || q.V != 15-40 || q.UW != 50 || q.VH != 30 {
		t.Fatalf("uv = %v %v %v %v", q.U, q.V, q.UW, q.VH)
	}
	if q.R2 != 40 || q.G2 != -20 || q.B2 != 4 {
		t.Fatalf("segment = %v %v hw %v, want 40 -20 4", q.R2, q.G2, q.B2)
	}
	if q.A != 1 {
		t.Fatalf("alpha = %v", q.A)
	}
}

// Lines batch with each other but never with other modes, and keep paint order.
func TestLineBatchesNotMergedAcrossModes(t *testing.T) {
	var b builder
	bg := shirei.Vec4{0, 0, 50, 1}
	black := shirei.Vec4{0, 0, 0, 1}
	fill := shirei.Surface{Rect: shirei.Rect{Size: shirei.Vec2{20, 20}}, Color1: bg, Color2: bg}
	b.build([]shirei.Surface{
		fill,
		lineSurface(shirei.Vec2{2, 2}, shirei.Vec2{18, 18}, 2, black),
		lineSurface(shirei.Vec2{2, 18}, shirei.Vec2{18, 2}, 2, black),
		fill,
	}, nil, 1, 20, 20)
	var kinds []int32
	var counts []int32
	for _, bt := range b.batches {
		kinds = append(kinds, bt.TexKind)
		counts = append(counts, bt.Count)
	}
	wantK := []int32{texFill, texLine, texFill}
	wantC := []int32{1, 2, 1}
	if len(kinds) != 3 {
		t.Fatalf("batches kinds=%v counts=%v want %v %v", kinds, counts, wantK, wantC)
	}
	for i := range wantK {
		if kinds[i] != wantK[i] || counts[i] != wantC[i] {
			t.Fatalf("batches kinds=%v counts=%v want %v %v", kinds, counts, wantK, wantC)
		}
	}
}

// Lines follow the clip stack (scissor and rounded SDF clips) and group alpha.
func TestLineClipAndAlpha(t *testing.T) {
	var b builder
	black := shirei.Vec4{0, 0, 0, 1}
	clipRect := shirei.Rect{Origin: shirei.Vec2{10, 10}, Size: shirei.Vec2{40, 40}}
	b.build([]shirei.Surface{
		{Rect: clipRect, Corners: shirei.N4(12), Clip: shirei.ClipPush, Transparency: 0.5},
		lineSurface(shirei.Vec2{0, 30}, shirei.Vec2{60, 30}, 4, black), // crosses the clip
		lineSurface(shirei.Vec2{0, 90}, shirei.Vec2{60, 90}, 4, black), // far outside the clip
		{Rect: clipRect, Clip: shirei.ClipPop, PopTransparency: true},
	}, nil, 1, 100, 100)
	if len(b.quads) != 1 {
		t.Fatalf("quads=%d want 1 (outside line culled by the clip)", len(b.quads))
	}
	bt := b.batches[0]
	if bt.TexKind != texLine || bt.ClipN != 1 {
		t.Fatalf("batch kind=%d clipN=%d want line with one rounded clip", bt.TexKind, bt.ClipN)
	}
	if bt.ClipX != 10 || bt.ClipY != 10 || bt.ClipW != 40 || bt.ClipH != 40 {
		t.Fatalf("scissor = %d,%d %dx%d", bt.ClipX, bt.ClipY, bt.ClipW, bt.ClipH)
	}
	if a := b.quads[0].A; a != 0.5 {
		t.Fatalf("group alpha = %v want 0.5", a)
	}
}
