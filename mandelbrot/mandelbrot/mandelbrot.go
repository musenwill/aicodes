// Package mandelbrot provides core computation and rendering for the Mandelbrot set.
package mandelbrot

import (
	"math"
)

// Config holds the parameters for Mandelbrot computation.
type Config struct {
	MaxIterations int     // Maximum iterations per pixel (up to 10000).
	EscapeRadius  float64 // Bailout radius (default 4.0, squared: 16.0).
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		MaxIterations: 256,
		EscapeRadius:  4.0,
	}
}

// Validate clamps Config values to safe ranges.
func (c *Config) Validate() {
	if c.MaxIterations < 1 {
		c.MaxIterations = 1
	}
	if c.MaxIterations > 10000 {
		c.MaxIterations = 10000
	}
	if c.EscapeRadius <= 0 {
		c.EscapeRadius = 4.0
	}
}

// IterCount computes the Mandelbrot escape iteration count for a single point
// in the complex plane using the standard formula: z = z² + c.
//
// Returns the number of iterations before escape, up to MaxIterations.
// For points inside the set, returns MaxIterations (no escape).
func IterCount(c complex128, cfg Config) int {
	if insideMainStructures(real(c), imag(c)) {
		return cfg.MaxIterations
	}
	escapeSq := cfg.EscapeRadius * cfg.EscapeRadius
	z := complex(0, 0)
	for i := 0; i < cfg.MaxIterations; i++ {
		if real(z)*real(z)+imag(z)*imag(z) > escapeSq {
			return i
		}
		z = z*z + c
	}
	return cfg.MaxIterations
}

// insideMainStructures reports whether the point (cr, ci) lies inside the
// main cardioid or the period-2 bulb. Such points are guaranteed members of
// the Mandelbrot set, so the full iteration loop can be skipped for them —
// a large fraction of the screen at low zoom levels.
func insideMainStructures(cr, ci float64) bool {
	// Period-2 bulb: disk of radius 1/4 centered at -1.
	if xx := cr + 1; xx*xx+ci*ci < 0.0625 {
		return true
	}
	// Main cardioid: ((x-¼)² + y²)² ≤ (x-¼)/2 + y²/4 in shifted form.
	x := cr - 0.25
	q := x*x + ci*ci
	return q*(q+x) < 0.25*ci*ci
}

// SmoothIterCount computes a smoothed iteration count for continuous coloring.
// The returned value is in [0, MaxIterations] and is fractional, producing
// smoother color transitions at the boundary of the Mandelbrot set.
//
// Formula: mu = n - log2(log(|z_n|) / log(R))
// where n is the discrete iteration at escape, |z_n| is the magnitude at escape,
// and R is the escape radius.
func SmoothIterCount(c complex128, cfg Config) float64 {
	return smoothIter(real(c), imag(c), cfg.MaxIterations,
		cfg.EscapeRadius*cfg.EscapeRadius, math.Log(cfg.EscapeRadius))
}

// smoothIter is the allocation-free hot path used by the renderer: plain
// float64 arithmetic instead of complex128, plus the interior fast path.
func smoothIter(cr, ci float64, maxIter int, escapeSq, logR float64) float64 {
	if insideMainStructures(cr, ci) {
		return float64(maxIter)
	}
	var zr, zi float64
	for i := 0; i < maxIter; i++ {
		zr2 := zr * zr
		zi2 := zi * zi
		if zr2+zi2 > escapeSq {
			// log(|z|) = 0.5 * log(zr² + zi²)
			logZn := 0.5 * math.Log(zr2+zi2)
			mu := float64(i) - math.Log2(logZn/logR)
			if mu < 0 {
				return 0
			}
			return mu
		}
		zi = 2*zr*zi + ci
		zr = zr2 - zi2 + cr
	}
	return float64(maxIter)
}
