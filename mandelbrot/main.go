// Mandelbrot Set Explorer — an interactive GUI for exploring the
// Mandelbrot set. Supports mouse-wheel zoom, configurable iteration precision,
// multiple color palettes, and parallel tile-based rendering.
//
// Usage:
//
//	mandelbrot [flags]
//
// Flags:
//
//	-iter N     Maximum iterations (1–10000, default 256)
//	-palette S  Color palette: PICO-8, Fire, BlueFire, Twilight (default Fire)
//	-rows N     Tile grid rows for parallel rendering (default 0 = auto)
//	-cols N     Tile grid columns for parallel rendering (default 0 = auto)
package main

import (
	"flag"
	"fmt"
	"image"
	"runtime"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"mandelbrot/mandelbrot"
)

// Game implements ebiten.Game and drives the Mandelbrot explorer.
type Game struct {
	cam      *mandelbrot.Camera
	renderer *mandelbrot.Renderer
	cfg      mandelbrot.Config

	// Cached ebiten image for the current view.
	cachedImg  *ebiten.Image
	needsRedraw bool

	// Current palette index.
	paletteIdx int
	palettes   []mandelbrot.Palette

	// Screen dimensions (updated on Layout).
	scrW, scrH int

	// FPS counter state.
	frameCount int
	fps        int
}

// NewGame creates a Game with the given configuration.
func NewGame(cfg mandelbrot.Config, paletteName string, rows, cols int) *Game {
	if rows <= 0 || cols <= 0 {
		rows = runtime.NumCPU()
		cols = runtime.NumCPU()
	}

	palettes := mandelbrot.BuiltinPalettes
	pIdx := paletteIndex(palettes, paletteName)

	return &Game{
		cam:         nil, // Initialized on first Layout.
		cfg:         cfg,
		renderer:    mandelbrot.NewRenderer(cfg, palettes[pIdx], rows, cols),
		palettes:    palettes,
		paletteIdx:  pIdx,
		needsRedraw: true,
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
		g.needsRedraw = true
		if g.cam == nil {
			g.cam = mandelbrot.NewCamera(outsideWidth, outsideHeight)
		} else {
			g.cam.AspectRatio = float64(outsideWidth) / float64(outsideHeight)
		}
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
		g.needsRedraw = true
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
		g.needsRedraw = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		g.paletteIdx = (g.paletteIdx + 1) % len(g.palettes)
		g.renderer.Palette = g.palettes[g.paletteIdx]
		g.needsRedraw = true
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
		g.needsRedraw = true
	}

	// --- FPS tracking ---
	g.frameCount++

	return nil
}

// Draw renders the Mandelbrot set to the screen. Re-computation only occurs
// when the view parameters have changed (dirty flag).
func (g *Game) Draw(screen *ebiten.Image) {
	if g.needsRedraw || g.cachedImg == nil {
		g.recompute()
	}

	op := &ebiten.DrawImageOptions{}
	screen.DrawImage(g.cachedImg, op)

	// Draw a simple HUD overlay.
	g.drawHUD(screen)
}

// recompute re-renders the Mandelbrot set using the parallel tile renderer
// and uploads the result to the GPU as an ebiten image.
func (g *Game) recompute() {
	if g.scrW <= 0 || g.scrH <= 0 {
		return
	}
	img := g.renderer.Render(g.scrW, g.scrH, g.cam)
	g.cachedImg = ebiten.NewImageFromImage(img)
	g.needsRedraw = false
}

// drawHUD renders a minimal status line showing zoom, iterations, and palette.
func (g *Game) drawHUD(screen *ebiten.Image) {
	// Ebitengine doesn't have built-in text rendering, so we use a minimal
	// approach: the window title is updated in Draw with current info.
	// This keeps the display unobstructed.
	if g.frameCount%30 == 0 {
		g.fps = int(ebiten.ActualFPS())
	}
	title := fmt.Sprintf("Mandelbrot | Zoom: %.2e | Iter: %d | Palette: %s | FPS: %d",
		g.cam.Zoom, g.cfg.MaxIterations, g.palettes[g.paletteIdx].Name, g.fps)
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
		g.needsRedraw = true
	}
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	// Parse flags.
	iter := flag.Int("iter", 256, "Maximum iterations (1–10000)")
	paletteName := flag.String("palette", "Fire", "Color palette: PICO-8, Fire, BlueFire, Twilight")
	rows := flag.Int("rows", 0, "Tile grid rows (0 = auto: NumCPU)")
	cols := flag.Int("cols", 0, "Tile grid cols (0 = auto: NumCPU)")
	flag.Parse()

	// Best-effort parse of extra os.Args that have no leading dash,
	// allowing positional args like "mandelbrot iter=1000 palette=BlueFire".
	for _, a := range flag.Args() {
		parseExtraArg(a, iter, paletteName, rows, cols)
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

	game := NewGame(cfg, *paletteName, *rows, *cols)

	if err := ebiten.RunGame(game); err != nil {
		// Normal exit via Escape terminates with an error; suppress it.
		if err.Error() == "exit" {
			return
		}
		fmt.Fprintf(flag.CommandLine.Output(), "error: %v\n", err)
	}
}

// parseExtraArg handles key=value positional arguments.
func parseExtraArg(a string, iter *int, paletteName *string, rows, cols *int) {
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
	case "rows":
		if v, err := strconv.Atoi(val); err == nil {
			*rows = v
		}
	case "cols":
		if v, err := strconv.Atoi(val); err == nil {
			*cols = v
		}
	}
}

// Ensure Game satisfies ebiten.Game.
var _ ebiten.Game = (*Game)(nil)

// Ensure image.Image is used (for documentation clarity).
var _ image.Image = (*image.RGBA)(nil)
