# Mandelbrot Set Explorer

A fullscreen interactive GUI program written in Go for exploring the Mandelbrot set fractal.

Runs on WSL2 + Ubuntu with X11, featuring parallel tile-based rendering, smooth coloring, and
mouse-wheel zoom.

![Mandelbrot Set]()

## Features

- **Fullscreen only** — immersive fractal exploration.
- **Mouse-wheel zoom** — zoom in/out at cursor position for precise navigation.
- **Non-blocking progressive rendering** — rendering never stalls the UI: while a new
  frame computes in the background, the previous frame is warped to the current view,
  a low-resolution preview appears within milliseconds, and finished bands are revealed
  incrementally on top.
- **Configurable iteration precision** — from 1 up to 10,000 iterations.
- **Multiple color palettes** — PICO-8, Fire, BlueFire, and Twilight gradients.
- **Parallel rendering** — a pool of worker goroutines renders dynamically scheduled
  horizontal bands (≈4× worker count), so slow boundary bands and fast sky bands
  balance themselves via work stealing.
- **Smooth coloring** — continuous iteration count for gradient-free color transitions.

## Prerequisites

- Go 1.21+
- Ubuntu (or WSL2 + X11 server)
- X11 development libraries

### Install X11 dependencies

```bash
sudo apt-get install -y \
    libx11-dev \
    libxxf86vm-dev \
    libxi-dev \
    libxrandr-dev \
    libxinerama-dev \
    libxcursor-dev
```

### WSL2 Notes

You need an X11 server running on Windows (e.g., [VcXsrv](https://sourceforge.net/projects/vcxsrv/),
[GWSL](https://opticos.github.io/gwsl/), or WSLg). Make sure `DISPLAY` is set:

```bash
export DISPLAY=:0
```

## Quick Start

```bash
# Build
make

# Run tests
make test

# Run with benchmarks
make bench

# Launch the explorer
make run
```

### Custom options

```bash
make run  # or:
./build/mandelbrot -iter 1000 -palette BlueFire -workers 8
```

## Controls

| Input            | Action                                    |
| ---------------- | ----------------------------------------- |
| Mouse wheel ↑    | Zoom in at cursor position                |
| Mouse wheel ↓    | Zoom out at cursor position               |
| ↑ / `+`          | Increase iteration precision              |
| ↓ / `-`          | Decrease iteration precision              |
| ←                | Previous color palette                    |
| →                | Next color palette                        |
| R                | Reset view to default                     |
| Esc              | Quit                                      |

## Make Targets

| Target        | Description                                   |
| ------------- | --------------------------------------------- |
| `make`        | Build the binary to `build/mandelbrot`        |
| `make test`   | Run unit tests with race detector             |
| `make bench`  | Run benchmarks                                |
| `make run`    | Build and launch                              |
| `make clean`  | Remove build artifacts                        |
| `make help`   | Show help                                     |

## Project Structure

```
mandelbrot/
├── main.go                          # Entry point, game loop, input handling
├── mandelbrot/
│   ├── mandelbrot.go               # Core Mandelbrot computation (escape-time algorithm)
│   ├── mandelbrot_test.go          # Unit tests and benchmarks
│   ├── camera.go                   # View state: complex-plane mapping and zoom
│   ├── palette.go                  # Color palettes, gradient interpolation, LUT
│   └── renderer.go                 # Parallel band renderer + async progressive Job
├── Makefile                         # Build / test / clean automation
├── go.mod
├── go.sum
└── README.md
```

## Architecture

### Computation

The Mandelbrot set is computed using the standard escape-time algorithm:

```
z₀ = 0
zₙ₊₁ = zₙ² + c
```

A point `c` is in the set if `|zₙ| ≤ R` for all iterations (up to `MaxIterations`).

### Smooth Coloring

To produce smooth color gradients, the fractional iteration count is computed using:

```
μ = n - log₂(log(|zₙ|) / log(R))
```

where `n` is the discrete escape iteration, `|zₙ|` the magnitude at escape, and `R` the escape radius.

### Parallel Rendering

1. The image is split into horizontal bands — roughly `4 × workers`, far more than
   there are goroutines — queued on an atomic cursor.
2. Each worker goroutine claims the next band, computes it, and moves on: fast
   bands (sky) and slow bands (set boundary) balance themselves dynamically.
3. Workers write RGBA pixels directly into the shared output buffer (disjoint rows,
   no locks, no intermediate tile copies).
4. Per-pixel work is kept lean: precomputed complex-plane stepping (no divisions in
   the hot loop), an interior fast path for the main cardioid and period-2 bulb,
   plain `float64` arithmetic, and a quarter-step palette lookup table.

This achieves near-linear speedup with core count for the rendering step.

### Asynchronous Progressive Pipeline

`Renderer.StartJob` returns a `Job` that streams results without blocking the UI:

1. Workers first render a **1/8-scale preview** cooperatively — it lands within
   milliseconds and is upscaled to fill the screen.
2. They then fill the **full-resolution image** band by band, delivering each
   finished band on a channel; the UI uploads it to the GPU (`WritePixels`) and
   reveals it over the preview the same frame.
3. While a job runs, the **previous completed frame is warped** (scaled and
   translated) to the current camera, so zooming always shows an aligned image.
4. New input cancels the in-flight job; restarts are throttled to 100 ms because
   the warped frame already tracks the cursor exactly.

### Camera

The camera maintains a view rectangle in the complex plane defined by:
- **Center** `(cx, cy)` — the complex point at screen center.
- **Zoom** — scale factor; base view width is 3.5 / zoom units.

When zooming at cursor position, the complex point under the cursor is preserved:
1. Record `c = screenToComplex(cursorX, cursorY)` before zoom.
2. Adjust zoom level.
3. Shift center so that `screenToComplex(cursorX, cursorY)` still maps to `c`.
