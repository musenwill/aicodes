# Mandelbrot Set Explorer

A fullscreen interactive GUI program written in Go for exploring the Mandelbrot set fractal.

Runs on WSL2 + Ubuntu with X11, featuring parallel tile-based rendering, smooth coloring, and
mouse-wheel zoom.

![Mandelbrot Set]()

## Features

- **Fullscreen only** — immersive fractal exploration.
- **Mouse-wheel zoom** — zoom in/out at cursor position for precise navigation.
- **Configurable iteration precision** — from 1 up to 10,000 iterations.
- **Multiple color palettes** — PICO-8, Fire, BlueFire, and Twilight gradients.
- **Parallel rendering** — the screen is divided into an M×N tile grid; each tile is
  computed in a separate goroutine, fully utilizing multi-core CPUs.
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
./build/mandelbrot -iter 1000 -palette BlueFire -rows 6 -cols 6
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
│   ├── palette.go                  # Color palettes and gradient interpolation
│   └── renderer.go                 # Parallel tile-based image renderer
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

1. The screen is divided into `rows × cols` rectangular tiles.
2. Each tile is dispatched to a goroutine.
3. Goroutines compute independently — no shared state, no synchronization needed.
4. A final pass assembles all tile results into a single RGBA image.
5. The image is uploaded to the GPU as an ebiten texture and drawn to the screen.

This achieves linear speedup with core count for the rendering step.

### Camera

The camera maintains a view rectangle in the complex plane defined by:
- **Center** `(cx, cy)` — the complex point at screen center.
- **Zoom** — scale factor; base view width is 3.5 / zoom units.

When zooming at cursor position, the complex point under the cursor is preserved:
1. Record `c = screenToComplex(cursorX, cursorY)` before zoom.
2. Adjust zoom level.
3. Shift center so that `screenToComplex(cursorX, cursorY)` still maps to `c`.
