// Mandelbrot Set Explorer — an interactive GUI for exploring the
// Mandelbrot set. Supports mouse-wheel zoom, configurable iteration precision,
// multiple color palettes, and asynchronous progressive rendering.
//
// Usage:
//
//	mandelbrot [flags]
//
// Flags:
//
//	-iter N      Maximum iterations (1–10000, default 256)
//	-palette S   Color palette: PICO-8, Fire, BlueFire, Twilight (default Fire)
//	-workers N   Render worker goroutines (0 = auto: NumCPU)
package main

import (
	"flag"
	"fmt"
	"image"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"mandelbrot/mandelbrot"
)

// jobRestartDelay throttles how often a new render job may start. During
// rapid input (wheel scrolling) the previous frame, warped to the current
// view, provides instant feedback, so jobs only need to start this often.
const jobRestartDelay = 100 * time.Millisecond

// Game implements ebiten.Game and drives the Mandelbrot explorer.
type Game struct {
	cam      *mandelbrot.Camera
	renderer *mandelbrot.Renderer
	cfg      mandelbrot.Config

	// Current palette index.
	paletteIdx int
	palettes   []mandelbrot.Palette

	// Screen dimensions (updated on Layout).
	scrW, scrH int

	// Asynchronous render pipeline.
	job        *mandelbrot.Job // In-flight render, nil when idle.
	viewDirty  bool            // View changed; a new job should start soon.
	lastStart  time.Time       // Last job start, for throttling.
	prevUpload bool            // Current job's preview was uploaded already.

	texFull *ebiten.Image // Full-res texture; finished bands are uploaded as they arrive.
	texOld  *ebiten.Image // Last completed frame, warped during interaction.
	texPrev *ebiten.Image // Low-res preview texture.

	frameCam      mandelbrot.Camera // Camera of the completed frame in texOld/texFull.
	frameCamValid bool

	// FPS counter state.
	frameCount int
	fps        int
}

// NewGame creates a Game with the given configuration.
func NewGame(cfg mandelbrot.Config, paletteName string, workers int) *Game {
	palettes := mandelbrot.BuiltinPalettes
	pIdx := paletteIndex(palettes, paletteName)

	r := mandelbrot.NewRenderer(cfg, palettes[pIdx], 1, 1)
	r.Workers = workers

	return &Game{
		cam:        nil, // Initialized on first Layout.
		cfg:        cfg,
		renderer:   r,
		palettes:   palettes,
		paletteIdx: pIdx,
		viewDirty:  true,
	}
}

// paletteIndex finds a palette by name, returning 0 if not found.
func paletteIndex(palettes []mandelbrot.Palette, name string) int {
	for i, p := range palettes {
		if p.Name == name {
			return i
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// ebiten.Game interface
// ---------------------------------------------------------------------------

// Layout reports the logical screen size. In fullscreen mode, outsideWidth
// and outsideHeight are the monitor's native resolution.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.scrW != outsideWidth || g.scrH != outsideHeight {
		g.scrW = outsideWidth
		g.scrH = outsideHeight
		if g.cam == nil {
			g.cam = mandelbrot.NewCamera(outsideWidth, outsideHeight)
		} else {
			g.cam.AspectRatio = float64(outsideWidth) / float64(outsideHeight)
		}
		// Textures must match the new size; drop everything and re-render.
		g.texFull, g.texOld, g.texPrev = nil, nil, nil
		g.frameCamValid = false
		g.viewChanged()
	}
	return outsideWidth, outsideHeight
}

// Update handles input events: zoom, iteration control, palette switching.
func (g *Game) Update() error {
	// --- Quit ---
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return fmt.Errorf("exit")
	}

	// --- Reset view ---
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.cam.Reset(g.scrW, g.scrH)
		g.viewChanged()
		return nil
	}

	// --- Iteration precision ---
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		g.adjustIterations(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		g.adjustIterations(-1)
	}

	// --- Palette switching ---
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		g.paletteIdx = (g.paletteIdx - 1 + len(g.palettes)) % len(g.palettes)
		g.renderer.Palette = g.palettes[g.paletteIdx]
		g.viewChanged()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.paletteIdx = (g.paletteIdx + 1) % len(g.palettes)
		g.renderer.Palette = g.palettes[g.paletteIdx]
		g.viewChanged()
	}

	// --- Mouse wheel zoom ---
	_, wy := ebiten.Wheel()
	if wy != 0 {
		mx, my := ebiten.CursorPosition()
		// Zoom factor: scroll up = zoom in (scale > 1), scroll down = zoom out.
		factor := 1.0
		if wy > 0 {
			factor = 1.0 / 1.25 // Zoom in.
		} else {
			factor = 1.25 // Zoom out.
		}
		g.cam.ZoomAt(factor, mx, my, g.scrW, g.scrH)
		g.viewChanged()
	}

	// --- Start a throttled render job if the view is stale ---
	g.maybeStartJob()

	// --- FPS tracking ---
	g.frameCount++

	return nil
}

// Draw composites the render pipeline every frame: the warped previous
// frame for instant feedback, the low-res preview once it lands, and
// finished full-resolution bands revealed incrementally on top.
func (g *Game) Draw(screen *ebiten.Image) {
	switch {
	case g.job == nil && g.frameCamValid:
		// Steady state: the completed frame.
		screen.DrawImage(g.texFull, &ebiten.DrawImageOptions{})

	case g.job != nil:
		// Transitional: previous frame warped to the current view.
		if g.frameCamValid {
			screen.DrawImage(g.texOld, g.warpGeoM())
		}
		// Low-res preview replaces the warp as soon as it is ready.
		if g.job.PreviewReady() && !g.prevUpload {
			g.uploadPreview()
			g.prevUpload = true
		}
		if g.prevUpload {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(float64(g.scrW)/float64(g.job.PreviewW),
				float64(g.scrH)/float64(g.job.PreviewH))
			screen.DrawImage(g.texPrev, op)
		}
		// Reveal finished full-res bands over the preview.
		g.drainBands(screen)
		if g.job.Finished() {
			g.frameCam = g.job.Cam
			g.frameCamValid = true
			// Snapshot the completed frame as the next warp source.
			g.texOld.DrawImage(g.texFull, &ebiten.DrawImageOptions{})
			screen.DrawImage(g.texFull, &ebiten.DrawImageOptions{})
			g.job = nil
		}
	}
	// Default (startup, no job yet): background only; preview appears shortly.

	g.drawHUD(screen)
}

// viewChanged cancels any in-flight render and marks the view stale.
func (g *Game) viewChanged() {
	if g.job != nil {
		g.job.Cancel()
		g.job = nil
	}
	g.viewDirty = true
}

// maybeStartJob starts a render job for a stale view, throttled by
// jobRestartDelay so rapid input does not churn jobs.
func (g *Game) maybeStartJob() {
	if !g.viewDirty || g.scrW <= 0 || g.scrH <= 0 {
		return
	}
	if time.Since(g.lastStart) < jobRestartDelay {
		return
	}
	g.ensureTextures()
	g.job = g.renderer.StartJob(g.scrW, g.scrH, *g.cam)
	g.prevUpload = false
	g.viewDirty = false
	g.lastStart = time.Now()
}

// ensureTextures (re)creates the GPU textures for the current screen size.
func (g *Game) ensureTextures() {
	if g.texFull != nil && g.texOld != nil &&
		g.texFull.Bounds().Dx() == g.scrW && g.texFull.Bounds().Dy() == g.scrH {
		return
	}
	g.texFull = ebiten.NewImage(g.scrW, g.scrH)
	g.texOld = ebiten.NewImage(g.scrW, g.scrH)
	g.texPrev = nil // Recreated on preview upload.
	g.frameCamValid = false
}

// uploadPreview uploads the job's low-res preview into texPrev.
func (g *Game) uploadPreview() {
	pw, ph := g.job.PreviewW, g.job.PreviewH
	if g.texPrev == nil || g.texPrev.Bounds().Dx() != pw || g.texPrev.Bounds().Dy() != ph {
		g.texPrev = ebiten.NewImage(pw, ph)
	}
	g.texPrev.WritePixels(g.job.Preview.Pix)
}

// drainBands uploads every full-res band finished since the last frame to
// the GPU and draws the refreshed regions over the preview. SubImage scopes
// WritePixels to just the band's rows.
func (g *Game) drainBands(screen *ebiten.Image) {
	stride := g.job.Full.Stride
	for {
		select {
		case b, ok := <-g.job.Bands:
			if !ok {
				return // Channel closed: job finished, nothing more to drain.
			}
			pixels := g.job.Full.Pix[b.Y0*stride : b.Y1*stride]
			sub := g.texFull.SubImage(image.Rect(0, b.Y0, g.scrW, b.Y1)).(*ebiten.Image)
			sub.WritePixels(pixels)
			screen.DrawImage(sub, &ebiten.DrawImageOptions{})
		default:
			return
		}
	}
}

// warpGeoM builds the transform that maps the completed frame (rendered for
// frameCam) onto the current view, so zooming shows an aligned stretch of
// the old image while the new one computes.
//
// Derived from p' = ((c - centerNew)/viewNew)·size + size/2 with
// c = centerOld + ((p/size) - 1/2)·viewOld, giving p' = p·s + B with
// s = viewOld/viewNew and B = size·((centerOld-centerNew)/viewNew + 1/2 - s/2).
func (g *Game) warpGeoM() *ebiten.DrawImageOptions {
	old := g.frameCam
	vwNew := g.cam.ViewWidth()
	vhNew := g.cam.ViewHeight()
	sx := old.ViewWidth() / vwNew
	sy := old.ViewHeight() / vhNew
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(
		float64(g.scrW)*((old.CenterX-g.cam.CenterX)/vwNew+0.5-0.5*sx),
		float64(g.scrH)*((old.CenterY-g.cam.CenterY)/vhNew+0.5-0.5*sy),
	)
	return op
}

// drawHUD reports zoom, iterations, palette and render progress via the
// window title, keeping the display unobstructed.
func (g *Game) drawHUD(screen *ebiten.Image) {
	if g.frameCount%30 == 0 {
		g.fps = int(ebiten.ActualFPS())
	}
	status := ""
	if g.job != nil {
		status = fmt.Sprintf(" | Rendering: %d%%", int(g.job.Progress()*100))
	}
	title := fmt.Sprintf("Mandelbrot | Zoom: %.2e | Iter: %d | Palette: %s | FPS: %d%s",
		g.cam.Zoom, g.cfg.MaxIterations, g.palettes[g.paletteIdx].Name, g.fps, status)
	ebiten.SetWindowTitle(title)
}

// adjustIterations changes the iteration cap by the given delta (logarithmic
// steps for fine control at low values and coarse control at high values).
func (g *Game) adjustIterations(delta int) {
	newVal := g.cfg.MaxIterations
	switch {
	case newVal < 100:
		newVal += delta * 10
	case newVal < 1000:
		newVal += delta * 100
	default:
		newVal += delta * 1000
	}
	if newVal < 10 {
		newVal = 10
	}
	if newVal > 10000 {
		newVal = 10000
	}
	if newVal != g.cfg.MaxIterations {
		g.cfg.MaxIterations = newVal
		g.renderer.Config.MaxIterations = newVal
		g.viewChanged()
	}
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	// Parse flags.
	iter := flag.Int("iter", 256, "Maximum iterations (1–10000)")
	paletteName := flag.String("palette", "Fire", "Color palette: PICO-8, Fire, BlueFire, Twilight")
	workers := flag.Int("workers", 0, "Render worker goroutines (0 = auto: NumCPU)")
	flag.Parse()

	// Best-effort parse of extra os.Args that have no leading dash,
	// allowing positional args like "mandelbrot iter=1000 palette=BlueFire".
	for _, a := range flag.Args() {
		parseExtraArg(a, iter, paletteName, workers)
	}

	// Build config.
	cfg := mandelbrot.Config{
		MaxIterations: *iter,
		EscapeRadius:  4.0,
	}
	cfg.Validate()
	*iter = cfg.MaxIterations

	// Fill the screen with a decorated window (close button, title bar).
	ebiten.SetWindowTitle("Mandelbrot Set Explorer")
	ebiten.SetWindowDecorated(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	sw, sh := ebiten.ScreenSizeInFullscreen()
	ebiten.SetWindowSize(sw, sh)

	game := NewGame(cfg, *paletteName, *workers)

	if err := ebiten.RunGame(game); err != nil {
		// Normal exit via Escape terminates with an error; suppress it.
		if err.Error() == "exit" {
			return
		}
		fmt.Fprintf(flag.CommandLine.Output(), "error: %v\n", err)
	}
}

// parseExtraArg handles key=value positional arguments.
func parseExtraArg(a string, iter *int, paletteName *string, workers *int) {
	// Quick manual split to avoid importing strings for a minor feature.
	eq := -1
	for i := 0; i < len(a); i++ {
		if a[i] == '=' {
			eq = i
			break
		}
	}
	if eq < 0 {
		return
	}
	key, val := a[:eq], a[eq+1:]
	switch key {
	case "iter":
		if v, err := strconv.Atoi(val); err == nil {
			*iter = v
		}
	case "palette":
		*paletteName = val
	case "workers":
		if v, err := strconv.Atoi(val); err == nil {
			*workers = v
		}
	}
}

// Ensure Game satisfies ebiten.Game.
var _ ebiten.Game = (*Game)(nil)

// Ensure image.Image is used (for documentation clarity).
var _ image.Image = (*image.RGBA)(nil)
