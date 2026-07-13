package mandelbrot

import (
	"image"
	"image/color"
	"sync"
)

// Tile describes a rectangular region of the screen to render.
type Tile struct {
	X, Y          int // Top-left pixel coordinates.
	Width, Height int // Tile dimensions in pixels.
}

// TileResult holds the computed pixel data for a single tile.
type TileResult struct {
	Tile  Tile
	Pixels []color.RGBA // Row-major pixel data, length = Width * Height.
}

// Renderer manages parallel Mandelbrot rendering using screen-space tiling.
type Renderer struct {
	Config  Config
	Palette Palette
	Rows    int // Number of tile rows.
	Cols    int // Number of tile columns.
}

// NewRenderer creates a Renderer with default settings.
// rows and cols define the concurrency grid (e.g. 4x4 = 16 goroutines).
func NewRenderer(cfg Config, pal Palette, rows, cols int) *Renderer {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	return &Renderer{
		Config:  cfg,
		Palette: pal,
		Rows:    rows,
		Cols:    cols,
	}
}

// Render computes the full Mandelbrot set image for the given screen size and camera.
// The screen is divided into Rows×Cols tiles, each computed in a separate goroutine.
func (r *Renderer) Render(screenW, screenH int, cam *Camera) *image.RGBA {
	// Precompute tile dimensions.
	tileW := screenW / r.Cols
	tileH := screenH / r.Rows
	lastColW := tileW + screenW%r.Cols
	lastRowH := tileH + screenH%r.Rows

	// Build tile list.
	tiles := make([]Tile, 0, r.Rows*r.Cols)
	for row := 0; row < r.Rows; row++ {
		for col := 0; col < r.Cols; col++ {
			w := tileW
			h := tileH
			if col == r.Cols-1 {
				w = lastColW
			}
			if row == r.Rows-1 {
				h = lastRowH
			}
			tiles = append(tiles, Tile{
				X:      col * tileW,
				Y:      row * tileH,
				Width:  w,
				Height: h,
			})
		}
	}

	// Render each tile concurrently.
	results := make([]TileResult, len(tiles))
	var wg sync.WaitGroup
	for i, tile := range tiles {
		wg.Add(1)
		go func(idx int, t Tile) {
			defer wg.Done()
			results[idx] = r.renderTile(t, screenW, screenH, cam)
		}(i, tile)
	}
	wg.Wait()

	// Assemble final image.
	img := image.NewRGBA(image.Rect(0, 0, screenW, screenH))
	for _, result := range results {
		for py := 0; py < result.Tile.Height; py++ {
			for px := 0; px < result.Tile.Width; px++ {
				imgX := result.Tile.X + px
				imgY := result.Tile.Y + py
				c := result.Pixels[py*result.Tile.Width+px]
				img.SetRGBA(imgX, imgY, c)
			}
		}
	}
	return img
}

// renderTile computes Mandelbrot iteration counts for a single tile.
func (r *Renderer) renderTile(t Tile, screenW, screenH int, cam *Camera) TileResult {
	pixels := make([]color.RGBA, t.Width*t.Height)
	idx := 0
	for py := 0; py < t.Height; py++ {
		screenY := t.Y + py
		for px := 0; px < t.Width; px++ {
			screenX := t.X + px
			c := cam.ScreenToComplex(screenX, screenY, screenW, screenH)
			iter := SmoothIterCount(c, r.Config)
			pixels[idx] = r.Palette.ColorForIter(iter, r.Config.MaxIterations)
			idx++
		}
	}
	return TileResult{Tile: t, Pixels: pixels}
}
