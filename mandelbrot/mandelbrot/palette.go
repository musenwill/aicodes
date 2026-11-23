package mandelbrot

import (
	"image/color"
	"math"
)

// Palette maps iteration counts to RGBA colors.
type Palette struct {
	Name   string
	Colors []color.RGBA
}

// PICO8 is a retro 16-color palette inspired by the PICO-8 fantasy console.
var PICO8 = Palette{
	Name: "PICO-8",
	Colors: []color.RGBA{
		{0, 0, 0, 255},       // black
		{29, 43, 83, 255},    // dark blue
		{126, 37, 83, 255},   // dark purple
		{0, 135, 81, 255},    // dark green
		{171, 82, 54, 255},   // brown
		{95, 87, 79, 255},    // dark gray
		{194, 195, 199, 255}, // light gray
		{255, 241, 232, 255}, // white
		{255, 0, 77, 255},    // red
		{255, 163, 0, 255},   // orange
		{255, 236, 39, 255},  // yellow
		{0, 228, 54, 255},    // green
		{41, 173, 255, 255},  // blue
		{131, 118, 156, 255}, // indigo
		{255, 119, 168, 255}, // pink
		{255, 204, 170, 255}, // peach
	},
}

// FirePalette generates a fire-like gradient: black → red → orange → yellow → white.
var FirePalette = Palette{
	Name: "Fire",
	Colors: buildGradient([]colorStop{
		{0.00, color.RGBA{0, 0, 0, 255}},
		{0.25, color.RGBA{89, 0, 0, 255}},
		{0.50, color.RGBA{200, 30, 0, 255}},
		{0.70, color.RGBA{255, 140, 0, 255}},
		{0.85, color.RGBA{255, 220, 50, 255}},
		{1.00, color.RGBA{255, 255, 240, 255}},
	}, 1024),
}

// BlueFire generates a cyan/blue gradient.
var BlueFire = Palette{
	Name: "BlueFire",
	Colors: buildGradient([]colorStop{
		{0.00, color.RGBA{0, 0, 32, 255}},
		{0.25, color.RGBA{0, 40, 100, 255}},
		{0.50, color.RGBA{0, 120, 200, 255}},
		{0.70, color.RGBA{60, 180, 255, 255}},
		{0.85, color.RGBA{150, 220, 255, 255}},
		{1.00, color.RGBA{255, 255, 255, 255}},
	}, 1024),
}

// Twilight creates a deep purple → gold gradient.
var Twilight = Palette{
	Name: "Twilight",
	Colors: buildGradient([]colorStop{
		{0.00, color.RGBA{10, 5, 30, 255}},
		{0.30, color.RGBA{60, 20, 100, 255}},
		{0.55, color.RGBA{180, 80, 60, 255}},
		{0.75, color.RGBA{240, 160, 40, 255}},
		{0.90, color.RGBA{255, 230, 160, 255}},
		{1.00, color.RGBA{255, 250, 240, 255}},
	}, 1024),
}

// BuiltinPalettes lists all built-in color palettes.
var BuiltinPalettes = []Palette{PICO8, FirePalette, BlueFire, Twilight}

// PaletteByName returns a built-in palette by name, or PICO8 if not found.
func PaletteByName(name string) Palette {
	for _, p := range BuiltinPalettes {
		if p.Name == name {
			return p
		}
	}
	return PICO8
}

// colorStop is an intermediate color at a specific position in the gradient.
type colorStop struct {
	Pos float64
	C   color.RGBA
}

// buildGradient interpolates between color stops to create a smooth gradient.
func buildGradient(stops []colorStop, numColors int) []color.RGBA {
	if len(stops) < 2 {
		return []color.RGBA{{0, 0, 0, 255}}
	}

	colors := make([]color.RGBA, numColors)
	for i := 0; i < numColors; i++ {
		t := float64(i) / float64(numColors-1)
		colors[i] = interpolateStops(stops, t)
	}
	return colors
}

// interpolateStops finds the two stops surrounding t and linearly interpolates.
func interpolateStops(stops []colorStop, t float64) color.RGBA {
	if t <= stops[0].Pos {
		return stops[0].C
	}
	if t >= stops[len(stops)-1].Pos {
		return stops[len(stops)-1].C
	}

	for i := 0; i < len(stops)-1; i++ {
		if t >= stops[i].Pos && t <= stops[i+1].Pos {
			segment := stops[i+1].Pos - stops[i].Pos
			local := (t - stops[i].Pos) / segment
			return lerpRGBA(stops[i].C, stops[i+1].C, local)
		}
	}
	return stops[0].C
}

// lerpRGBA linearly interpolates between two RGBA colors.
func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 255,
	}
}

// ColorForIter maps an iteration count to an RGBA color using the given palette.
// If count equals maxIterations (point is in the set), returns black.
func (p Palette) ColorForIter(iter float64, maxIterations int) color.RGBA {
	if int(iter) >= maxIterations {
		return color.RGBA{0, 0, 0, 255}
	}
	// Map iteration to palette index, wrapping for high iteration counts.
	frac := iter / float64(maxIterations)
	// Use sinusoidal modulation for more visual interest.
	idx := frac * float64(len(p.Colors))
	// Apply a slight non-linear mapping for more contrast at the boundary.
	idx = math.Sqrt(idx*float64(len(p.Colors))) / math.Sqrt(float64(len(p.Colors))) * float64(len(p.Colors))
	i := int(idx) % len(p.Colors)
	return p.Colors[i]
}

// buildLUT precomputes a quarter-step color lookup table covering the smooth
// iteration range [0, maxIterations]. The render hot loop then replaces all
// per-pixel palette math (division, sqrt, modulo) with one indexed read.
// Index with int(mu*4); values agree with ColorForIter to within ¼ iteration,
// which is invisible after color quantization.
func (p Palette) buildLUT(maxIterations int) []color.RGBA {
	lut := make([]color.RGBA, 4*maxIterations+1)
	for i := range lut {
		lut[i] = p.ColorForIter(float64(i)/4, maxIterations)
	}
	return lut
}
