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

// SmoothIterCount computes a smoothed iteration count for continuous coloring.
// The returned value is in [0, MaxIterations] and is fractional, producing
// smoother color transitions at the boundary of the Mandelbrot set.
//
// Formula: mu = n - log2(log(|z_n|) / log(R))
// where n is the discrete iteration at escape, |z_n| is the magnitude at escape,
// and R is the escape radius.
func SmoothIterCount(c complex128, cfg Config) float64 {
	escapeSq := cfg.EscapeRadius * cfg.EscapeRadius
	logR := math.Log(cfg.EscapeRadius)
	z := complex(0, 0)

	for i := 0; i < cfg.MaxIterations; i++ {
		zr, zi := real(z), imag(z)
		if zr*zr+zi*zi > escapeSq {
			// log(|z|) = 0.5 * log(zr² + zi²)
			logZn := 0.5 * math.Log(zr*zr+zi*zi)
			mu := float64(i) - math.Log2(logZn/logR)
			if mu < 0 {
				return 0
			}
			return mu
		}
		z = z*z + c
	}
	return float64(cfg.MaxIterations)
}
