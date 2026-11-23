package mandelbrot

import (
	"image/color"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// IterCount tests
// ---------------------------------------------------------------------------

func TestIterCountInsideSet(t *testing.T) {
	cfg := Config{MaxIterations: 256, EscapeRadius: 4.0}
	// The origin is firmly inside the Mandelbrot set.
	c := complex(0, 0)
	n := IterCount(c, cfg)
	if n != cfg.MaxIterations {
		t.Errorf("origin should be inside the set, got %d iterations (want %d)", n, cfg.MaxIterations)
	}
}

func TestIterCountOutsideSet(t *testing.T) {
	cfg := Config{MaxIterations: 256, EscapeRadius: 4.0}
	// Point far outside the set should escape in very few iterations.
	c := complex(3, 0)
	n := IterCount(c, cfg)
	if n >= cfg.MaxIterations {
		t.Errorf("point (3+0i) should escape, got %d iterations", n)
	}
	if n == 0 {
		// The point (3+0i): z0=0, z1=3, |z1|=9 > 16? No. z2=12, |z2|=144 > 16, escapes at iter 2.
		// So n should be >= 1.
		t.Errorf("point (3+0i) should escape at iter >= 1, got %d", n)
	}
}

func TestIterCountKnownPoint(t *testing.T) {
	cfg := Config{MaxIterations: 1000, EscapeRadius: 4.0}
	// c = -1 should be inside (period-2 bulb center).
	if n := IterCount(complex(-1, 0), cfg); n != cfg.MaxIterations {
		t.Errorf("(-1+0i) should be inside set, got %d", n)
	}
	// c = i should be inside.
	if n := IterCount(complex(0, 1), cfg); n != cfg.MaxIterations {
		t.Errorf("(0+1i) should be inside set, got %d", n)
	}
}

func TestIterCountLowMaxIter(t *testing.T) {
	cfg := Config{MaxIterations: 1, EscapeRadius: 4.0}
	n := IterCount(complex(0, 0), cfg)
	if n != 1 {
		t.Errorf("with MaxIterations=1, origin should return 1, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// SmoothIterCount tests
// ---------------------------------------------------------------------------

func TestSmoothIterCountInside(t *testing.T) {
	cfg := Config{MaxIterations: 256, EscapeRadius: 4.0}
	s := SmoothIterCount(complex(0, 0), cfg)
	if s != float64(cfg.MaxIterations) {
		t.Errorf("origin should return MaxIterations=%d, got %f", cfg.MaxIterations, s)
	}
}

func TestSmoothIterCountOutside(t *testing.T) {
	cfg := Config{MaxIterations: 256, EscapeRadius: 4.0}
	s := SmoothIterCount(complex(3, 0), cfg)
	if s >= float64(cfg.MaxIterations) {
		t.Errorf("point (3+0i) should escape: got %f >= %d", s, cfg.MaxIterations)
	}
	if s < 0 {
		t.Errorf("smooth iteration should be >= 0, got %f", s)
	}
}

func TestSmoothVersusDiscrete(t *testing.T) {
	cfg := Config{MaxIterations: 256, EscapeRadius: 4.0}
	// Test several points: smooth and discrete should be close.
	points := []complex128{
		complex(-0.5, 0.5),
		complex(-1.5, 0),
		complex(-0.75, 0.1),
	}
	for _, c := range points {
		d := IterCount(c, cfg)
		s := SmoothIterCount(c, cfg)
		if d == cfg.MaxIterations && s == float64(cfg.MaxIterations) {
			continue
		}
		// Smooth should be within [d-1, d+1].
		if s < float64(d)-1.5 || s > float64(d)+1.5 {
			t.Errorf("c=%v: discrete=%d, smooth=%f (expected %d±1)", c, d, s, d)
		}
	}
}

// ---------------------------------------------------------------------------
// Config tests
// ---------------------------------------------------------------------------

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantIter int
		wantR    float64
	}{
		{"default", DefaultConfig(), 256, 4.0},
		{"negative iter", Config{MaxIterations: -5, EscapeRadius: 4.0}, 1, 4.0},
		{"too high iter", Config{MaxIterations: 20000, EscapeRadius: 4.0}, 10000, 4.0},
		{"zero radius", Config{MaxIterations: 100, EscapeRadius: 0}, 100, 4.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Validate()
			if tt.cfg.MaxIterations != tt.wantIter {
				t.Errorf("MaxIterations: want %d, got %d", tt.wantIter, tt.cfg.MaxIterations)
			}
			if tt.cfg.EscapeRadius != tt.wantR {
				t.Errorf("EscapeRadius: want %f, got %f", tt.wantR, tt.cfg.EscapeRadius)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Camera tests
// ---------------------------------------------------------------------------

func TestCameraNew(t *testing.T) {
	cam := NewCamera(1920, 1080)
	if cam.CenterX != -0.75 {
		t.Errorf("CenterX: want -0.75, got %f", cam.CenterX)
	}
	if cam.CenterY != 0.0 {
		t.Errorf("CenterY: want 0, got %f", cam.CenterY)
	}
	if cam.Zoom != 1.0 {
		t.Errorf("Zoom: want 1.0, got %f", cam.Zoom)
	}
}

func TestCameraScreenToComplexOrigin(t *testing.T) {
	cam := NewCamera(800, 600)
	// Center of screen should map to camera center.
	c := cam.ScreenToComplex(400, 300, 800, 600)
	if real(c) != cam.CenterX || imag(c) != cam.CenterY {
		t.Errorf("screen center should map to (%f, %f), got (%f, %f)",
			cam.CenterX, cam.CenterY, real(c), imag(c))
	}
}

func TestCameraZoomAt(t *testing.T) {
	cam := NewCamera(800, 600)
	// Zoom in by factor 0.5 at screen center → center unchanged.
	oldCX, oldCY := cam.CenterX, cam.CenterY
	cam.ZoomAt(2.0, 400, 300, 800, 600) // factor=2 means zoom is multiplied by 2.
	if cam.Zoom != 2.0 {
		t.Errorf("Zoom: want 2.0, got %f", cam.Zoom)
	}
	if cam.CenterX != oldCX || cam.CenterY != oldCY {
		t.Errorf("zoom at center should not move center")
	}

	// Check that the complex point under a given pixel is invariant.
	cam2 := NewCamera(800, 600)
	px, py := 200, 150
	before := cam2.ScreenToComplex(px, py, 800, 600)
	cam2.ZoomAt(0.5, px, py, 800, 600) // Zoom out.
	after := cam2.ScreenToComplex(px, py, 800, 600)
	tolerance := 1e-6
	if diff := real(before) - real(after); diff < -tolerance || diff > tolerance {
		t.Errorf("zoom-at should preserve complex point: before=%v, after=%v", before, after)
	}
}

func TestCameraReset(t *testing.T) {
	cam := NewCamera(800, 600)
	cam.ZoomAt(10, 100, 100, 800, 600) // Move and zoom.
	cam.Reset(1920, 1080)
	if cam.CenterX != -0.75 || cam.CenterY != 0.0 || cam.Zoom != 1.0 {
		t.Error("reset should restore defaults")
	}
	if cam.AspectRatio != 1920.0/1080.0 {
		t.Errorf("AspectRatio should be updated on reset")
	}
}

func TestCameraViewDimensions(t *testing.T) {
	cam := NewCamera(1600, 900)
	w := cam.ViewWidth()
	h := cam.ViewHeight()
	if w <= 0 || h <= 0 {
		t.Errorf("view dimensions should be positive: w=%f, h=%f", w, h)
	}
	// w/h should equal aspect ratio.
	if ratio := w / h; ratio < cam.AspectRatio-0.01 || ratio > cam.AspectRatio+0.01 {
		t.Errorf("w/h = %f, aspect = %f", ratio, cam.AspectRatio)
	}
}

// ---------------------------------------------------------------------------
// Palette tests
// ---------------------------------------------------------------------------

func TestPaletteColorForIterInside(t *testing.T) {
	p := PICO8
	// Points inside the set (iter == maxIterations) should be black.
	c := p.ColorForIter(256, 256)
	if c != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("inside set should be black, got %v", c)
	}
}

func TestPaletteColorForIterOutside(t *testing.T) {
	p := PICO8
	// First iteration should map to first color.
	c := p.ColorForIter(0, 256)
	// Should be one of the palette colors (first index area).
	if c.A != 255 {
		t.Errorf("alpha should be 255, got %d", c.A)
	}
}

func TestPaletteByName(t *testing.T) {
	if p := PaletteByName("Fire"); p.Name != "Fire" {
		t.Errorf("expected Fire, got %s", p.Name)
	}
	if p := PaletteByName("BlueFire"); p.Name != "BlueFire" {
		t.Errorf("expected BlueFire, got %s", p.Name)
	}
	if p := PaletteByName("Twilight"); p.Name != "Twilight" {
		t.Errorf("expected Twilight, got %s", p.Name)
	}
	if p := PaletteByName("NonExistent"); p.Name != "PICO-8" {
		t.Errorf("unknown palette should default to PICO-8, got %s", p.Name)
	}
}

func TestPaletteGradientLength(t *testing.T) {
	for _, p := range BuiltinPalettes {
		if len(p.Colors) == 0 {
			t.Errorf("palette %s has no colors", p.Name)
		}
	}
}

func TestBuildGradient(t *testing.T) {
	stops := []colorStop{
		{0.0, color.RGBA{0, 0, 0, 255}},
		{1.0, color.RGBA{255, 255, 255, 255}},
	}
	colors := buildGradient(stops, 256)
	if len(colors) != 256 {
		t.Errorf("expected 256 colors, got %d", len(colors))
	}
	if colors[0] != stops[0].C {
		t.Errorf("first color should be black")
	}
	if colors[255] != stops[1].C {
		t.Errorf("last color should be white, got %v", colors[255])
	}
	// Middle should be gray.
	mid := colors[127]
	if mid.R < 100 || mid.R > 155 {
		t.Errorf("middle color should be gray, got %v", mid)
	}
}

func TestBuildGradientSingleStop(t *testing.T) {
	stops := []colorStop{{0.5, color.RGBA{100, 100, 100, 255}}}
	// Single stop → falls back to a single black color (needs at least 2 stops).
	colors := buildGradient(stops, 10)
	if len(colors) != 1 {
		t.Errorf("single stop should return 1 default color, got %d", len(colors))
	}
	if colors[0] != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("single stop should fallback to black, got %v", colors[0])
	}
}

// ---------------------------------------------------------------------------
// Renderer tests
// ---------------------------------------------------------------------------

func TestNewRenderer(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRenderer(cfg, PICO8, 4, 6)
	if r.Rows != 4 {
		t.Errorf("Rows: want 4, got %d", r.Rows)
	}
	if r.Cols != 6 {
		t.Errorf("Cols: want 6, got %d", r.Cols)
	}
}

func TestNewRendererClamp(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRenderer(cfg, PICO8, 0, -1)
	if r.Rows != 1 {
		t.Errorf("Rows should be clamped to 1, got %d", r.Rows)
	}
	if r.Cols != 1 {
		t.Errorf("Cols should be clamped to 1, got %d", r.Cols)
	}
}

func TestRenderProducesValidImage(t *testing.T) {
	cfg := Config{MaxIterations: 64, EscapeRadius: 4.0}
	cam := NewCamera(320, 240)
	r := NewRenderer(cfg, PICO8, 2, 3)

	img := r.Render(320, 240, cam)
	if img == nil {
		t.Fatal("Render returned nil")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 320 || bounds.Dy() != 240 {
		t.Errorf("image size: want 320×240, got %d×%d", bounds.Dx(), bounds.Dy())
	}

	// Verify that non-uniform tile sizes work correctly.
	img2 := r.Render(319, 241, cam)
	if img2 == nil {
		t.Fatal("Render returned nil for odd dimensions")
	}
	b2 := img2.Bounds()
	if b2.Dx() != 319 || b2.Dy() != 241 {
		t.Errorf("odd image size: want 319×241, got %d×%d", b2.Dx(), b2.Dy())
	}
}

func TestRenderSingleTile(t *testing.T) {
	cfg := Config{MaxIterations: 64, EscapeRadius: 4.0}
	cam := NewCamera(100, 100)
	r := NewRenderer(cfg, PICO8, 1, 1) // Single tile.

	img := r.Render(100, 100, cam)
	if img == nil {
		t.Fatal("Render returned nil")
	}
	// All pixels near the center of the standard view should be mostly in-set (black).
	c := img.RGBAAt(50, 50)
	if c.A != 255 {
		t.Errorf("alpha channel should be 255")
	}
}

func TestRenderConcurrentConsistency(t *testing.T) {
	cfg := Config{MaxIterations: 64, EscapeRadius: 4.0}
	cam := NewCamera(100, 100)

	// Single-tile and multi-tile renderers should produce the same image.
	r1 := NewRenderer(cfg, PICO8, 1, 1)
	rMany := NewRenderer(cfg, PICO8, 4, 4)

	img1 := r1.Render(100, 100, cam)
	imgMany := rMany.Render(100, 100, cam)

	// Compare pixel-by-pixel.
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			c1 := img1.RGBAAt(x, y)
			c2 := imgMany.RGBAAt(x, y)
			if c1 != c2 {
				t.Errorf("pixel (%d,%d) differs: %v vs %v", x, y, c1, c2)
				return // One failure is sufficient.
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Interior fast-path tests
// ---------------------------------------------------------------------------

func TestInsideMainStructures(t *testing.T) {
	inside := []struct {
		name   string
		cr, ci float64
	}{
		{"origin (cardioid)", 0, 0},
		{"cardioid interior", 0.2, 0.1},
		{"period-2 bulb center", -1, 0},
		{"bulb interior", -0.9, 0.1},
	}
	for _, p := range inside {
		if !insideMainStructures(p.cr, p.ci) {
			t.Errorf("%s (%v,%v) should be inside", p.name, p.cr, p.ci)
		}
	}
	outside := []struct {
		name   string
		cr, ci float64
	}{
		{"far outside", 3, 0},
		{"above cardioid", 0.25, 0.8},
		{"between bulb and cardioid", -1.6, 0},
		{"tip of antenna", -2, 0},
	}
	for _, p := range outside {
		if insideMainStructures(p.cr, p.ci) {
			t.Errorf("%s (%v,%v) should be outside", p.name, p.cr, p.ci)
		}
	}
}

func TestInteriorFastPathMatchesIteration(t *testing.T) {
	cfg := Config{MaxIterations: 512, EscapeRadius: 4.0}
	// Points claimed inside by the fast path must return MaxIterations,
	// exactly as the full iteration loop would.
	for _, p := range [][2]float64{{0, 0}, {-1, 0}, {-0.85, 0.1}, {0.2, 0.2}} {
		if !insideMainStructures(p[0], p[1]) {
			t.Fatalf("test point (%v,%v) unexpectedly outside", p[0], p[1])
		}
		if got := IterCount(complex(p[0], p[1]), cfg); got != cfg.MaxIterations {
			t.Errorf("IterCount(%v): want %d, got %d", p, cfg.MaxIterations, got)
		}
		if got := SmoothIterCount(complex(p[0], p[1]), cfg); got != float64(cfg.MaxIterations) {
			t.Errorf("SmoothIterCount(%v): want %d, got %f", p, cfg.MaxIterations, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Palette LUT tests
// ---------------------------------------------------------------------------

func TestBuildLUTMatchesColorForIter(t *testing.T) {
	p := FirePalette
	maxIter := 100
	lut := p.buildLUT(maxIter)
	if len(lut) != 4*maxIter+1 {
		t.Fatalf("LUT length: want %d, got %d", 4*maxIter+1, len(lut))
	}
	// Every quarter-step entry must equal the direct computation.
	for i, c := range lut {
		want := p.ColorForIter(float64(i)/4, maxIter)
		if c != want {
			t.Fatalf("LUT[%d] (iter=%v): want %v, got %v", i, float64(i)/4, want, c)
		}
	}
}

// ---------------------------------------------------------------------------
// Async Job tests
// ---------------------------------------------------------------------------

func TestJobCompletes(t *testing.T) {
	cfg := Config{MaxIterations: 64, EscapeRadius: 4.0}
	cam := *NewCamera(64, 48)
	r := NewRenderer(cfg, PICO8, 1, 1)
	r.Workers = 4

	j := r.StartJob(64, 48, cam)
	defer j.Cancel()

	// Both phases must finish promptly.
	deadline := time.Now().Add(5 * time.Second)
	for !j.Finished() {
		if time.Now().After(deadline) {
			t.Fatal("job did not finish within 5s")
		}
		time.Sleep(time.Millisecond)
	}
	if !j.PreviewReady() {
		t.Error("preview should be ready when job finishes")
	}
	if j.Progress() != 1 {
		t.Errorf("progress after finish: want 1, got %f", j.Progress())
	}

	// Preview dimensions follow the downscale factor.
	if want := max(1, 64/previewDownscale); j.PreviewW != want {
		t.Errorf("PreviewW: want %d, got %d", want, j.PreviewW)
	}
	if want := max(1, 48/previewDownscale); j.PreviewH != want {
		t.Errorf("PreviewH: want %d, got %d", want, j.PreviewH)
	}

	// Full image must be fully painted.
	if got := j.Full.Bounds(); got.Dx() != 64 || got.Dy() != 48 {
		t.Errorf("full image bounds: want 64×48, got %v", got)
	}

	// Bands channel must have delivered every band.
	bands := 0
	for range j.Bands {
		bands++
	}
	if bands == 0 {
		t.Error("expected at least one band event")
	}
}

func TestJobCancelIsSafe(t *testing.T) {
	cfg := Config{MaxIterations: 1000, EscapeRadius: 4.0}
	cam := *NewCamera(320, 240)
	r := NewRenderer(cfg, PICO8, 1, 1)
	r.Workers = 2

	j := r.StartJob(320, 240, cam)
	j.Cancel() // Abandon immediately, as rapid input would.
	if !j.Cancelled() {
		t.Error("Cancelled() should report true after Cancel()")
	}
	// Smoke-test the accessors on an abandoned job.
	_ = j.PreviewReady()
	_ = j.Finished()
	_ = j.Progress()
	// Give abandoned workers a moment to bail out; nothing should panic
	// or deadlock under the race detector.
	time.Sleep(20 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkIterCount(b *testing.B) {
	cfg := DefaultConfig()
	c := complex(-0.5, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IterCount(c, cfg)
	}
}

func BenchmarkSmoothIterCount(b *testing.B) {
	cfg := DefaultConfig()
	c := complex(-0.5, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SmoothIterCount(c, cfg)
	}
}

func BenchmarkRender(b *testing.B) {
	cfg := Config{MaxIterations: 64, EscapeRadius: 4.0}
	cam := NewCamera(320, 240)
	r := NewRenderer(cfg, PICO8, 4, 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Render(320, 240, cam)
	}
}
