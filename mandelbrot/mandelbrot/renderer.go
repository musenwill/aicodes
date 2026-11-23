package mandelbrot

import (
	"image"
	"image/color"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

// renderTarget bundles the precomputed pixel→complex mapping and palette
// lookup table for one render pass, so the per-pixel hot loop does no
// divisions, complex unpacking, or palette math.
type renderTarget struct {
	img  *image.RGBA // Destination buffer; workers write disjoint row bands.
	w, h int
	// Complex-plane coordinates of pixel (0, 0) and per-pixel steps.
	xLeft, dx, yTop, dy float64

	lut      []color.RGBA // Quarter-step palette lookup, indexed by 4*mu.
	maxIter  int
	escapeSq float64
	logR     float64
}

// newRenderTarget precomputes the mapping for rendering cam into a w×h buffer.
func newRenderTarget(w, h int, cam *Camera, lut []color.RGBA, cfg Config) *renderTarget {
	vw := cam.ViewWidth()
	vh := cam.ViewHeight()
	return &renderTarget{
		img:      image.NewRGBA(image.Rect(0, 0, w, h)),
		w:        w,
		h:        h,
		xLeft:    cam.CenterX - 0.5*vw,
		dx:       vw / float64(w),
		yTop:     cam.CenterY - 0.5*vh,
		dy:       vh / float64(h),
		lut:      lut,
		maxIter:  cfg.MaxIterations,
		escapeSq: cfg.EscapeRadius * cfg.EscapeRadius,
		logR:     math.Log(cfg.EscapeRadius),
	}
}

// renderBand computes rows [y0, y1) of the target, writing RGBA pixels
// directly into the shared buffer. Bands are disjoint, so concurrent calls
// from multiple goroutines are safe without locks.
func (t *renderTarget) renderBand(y0, y1 int) {
	lut := t.lut
	for y := y0; y < y1; y++ {
		ci := t.yTop + float64(y)*t.dy
		base := y * t.img.Stride
		row := t.img.Pix[base : base+t.w*4]
		for x := 0; x < t.w; x++ {
			cr := t.xLeft + float64(x)*t.dx
			mu := smoothIter(cr, ci, t.maxIter, t.escapeSq, t.logR)
			idx := int(mu * 4)
			if idx >= len(lut) {
				idx = len(lut) - 1
			}
			c := lut[idx]
			o := x * 4
			row[o] = c.R
			row[o+1] = c.G
			row[o+2] = c.B
			row[o+3] = 255
		}
	}
}

// Renderer manages parallel Mandelbrot rendering using dynamically scheduled
// horizontal bands. Bands are far more numerous than workers, so fast bands
// (sky) and slow bands (set boundary) balance themselves via work stealing.
type Renderer struct {
	Config  Config
	Palette Palette
	Workers int // Number of render goroutines (0 = runtime.NumCPU()).

	// Rows and Cols are legacy tile-grid fields kept for API compatibility;
	// rendering no longer uses a static tile grid.
	Rows int
	Cols int
}

// NewRenderer creates a Renderer. rows and cols are legacy and no longer
// affect scheduling; set Workers to control concurrency.
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

func (r *Renderer) workerCount() int {
	if r.Workers > 0 {
		return r.Workers
	}
	if n := runtime.NumCPU(); n > 0 {
		return n
	}
	return 1
}

// splitBands divides h rows into roughly 4×workers bands for dynamic
// scheduling. Band count is capped by h so tiny images stay cheap.
func splitBands(h, workers int) [][2]int {
	count := workers * 4
	if count > h {
		count = h
	}
	if count < 1 {
		return nil
	}
	bandH := (h + count - 1) / count
	bands := make([][2]int, 0, count)
	for y := 0; y < h; y += bandH {
		y1 := y + bandH
		if y1 > h {
			y1 = h
		}
		bands = append(bands, [2]int{y, y1})
	}
	return bands
}

// Render computes the full Mandelbrot set image synchronously, distributing
// row bands across a worker pool. It writes pixels directly into the output
// buffer, with no intermediate tile copies.
func (r *Renderer) Render(screenW, screenH int, cam *Camera) *image.RGBA {
	if screenW <= 0 || screenH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	lut := r.Palette.buildLUT(r.Config.MaxIterations)
	t := newRenderTarget(screenW, screenH, cam, lut, r.Config)

	bands := splitBands(screenH, r.workerCount())
	var cursor atomic.Int64
	var wg sync.WaitGroup
	workers := r.workerCount()
	if workers > len(bands) {
		workers = len(bands)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				k := cursor.Add(1) - 1
				if k >= int64(len(bands)) {
					return
				}
				b := bands[k]
				t.renderBand(b[0], b[1])
			}
		}()
	}
	wg.Wait()
	return t.img
}

// ---------------------------------------------------------------------------
// Asynchronous progressive rendering
// ---------------------------------------------------------------------------

// previewDownscale is the resolution reduction factor of the preview pass
// (1/8 linear = 1/64 of the pixels), refined first for instant feedback.
const previewDownscale = 8

// Band describes a completed row range of a Job's full-resolution image,
// ready to be uploaded to the GPU incrementally.
type Band struct {
	Y0, Y1 int
}

// Job is an asynchronous progressive render produced by Renderer.StartJob.
//
// A pool of workers first renders a low-resolution preview (PreviewReady
// closes when the preview image is complete), then fills Full band by band,
// sending each finished band to Bands. Done closes when Full is complete.
// Cancel makes workers abandon remaining work; abandoned jobs are simply
// dropped by the caller. All exported fields/channels are safe to use from
// the UI thread while workers run.
type Job struct {
	Cam Camera // Snapshot of the view this job renders.
	Cfg Config // Snapshot of the config this job renders.

	PreviewW, PreviewH int
	Preview            *image.RGBA // Low-res first pass; valid after PreviewReady.

	Full  *image.RGBA // Full-res image, valid after Done.
	Bands chan Band   // Completed full-res bands (buffered; never blocks).

	previewReady chan struct{}
	done         chan struct{}
	cancel       atomic.Bool
	remaining    atomic.Int64 // Full-res bands not yet finished.
	totalBands   int64
}

// Cancel abandons the job: workers stop picking up new bands and the caller
// should stop reading its channels. Safe to call more than once.
func (j *Job) Cancel() { j.cancel.Store(true) }

// Cancelled reports whether the job has been cancelled.
func (j *Job) Cancelled() bool { return j.cancel.Load() }

// PreviewReady reports whether the low-resolution preview is complete.
func (j *Job) PreviewReady() bool {
	select {
	case <-j.previewReady:
		return true
	default:
		return false
	}
}

// Finished reports whether the full-resolution image is complete.
func (j *Job) Finished() bool {
	select {
	case <-j.done:
		return true
	default:
		return false
	}
}

// Progress returns the fraction of full-resolution bands completed, in [0, 1].
func (j *Job) Progress() float64 {
	if j.Finished() {
		return 1
	}
	return 1 - float64(j.remaining.Load())/float64(j.totalBands)
}

// task is one unit of work: a row band of a specific render target.
type task struct {
	t         *renderTarget
	y0, y1    int
	isPreview bool
}

// StartJob launches an asynchronous progressive render of the given view:
// workers cooperatively render the preview first, then chew through the
// full-resolution bands. The returned Job streams results without blocking
// the caller.
func (r *Renderer) StartJob(screenW, screenH int, cam Camera) *Job {
	lut := r.Palette.buildLUT(r.Config.MaxIterations)
	workers := r.workerCount()

	pw := max(1, screenW/previewDownscale)
	ph := max(1, screenH/previewDownscale)
	preview := newRenderTarget(pw, ph, &cam, lut, r.Config)
	full := newRenderTarget(screenW, screenH, &cam, lut, r.Config)

	prevBands := splitBands(ph, workers)
	fullBands := splitBands(screenH, workers)

	j := &Job{
		Cam:          cam,
		Cfg:          r.Config,
		PreviewW:     pw,
		PreviewH:     ph,
		Preview:      preview.img,
		Full:         full.img,
		Bands:        make(chan Band, len(fullBands)),
		previewReady: make(chan struct{}),
		done:         make(chan struct{}),
		totalBands:   int64(len(fullBands)),
	}
	j.remaining.Store(int64(len(fullBands)))

	// Preview bands are queued first so the pool finishes them collectively
	// before moving on to full-resolution work.
	tasks := make([]task, 0, len(prevBands)+len(fullBands))
	for _, b := range prevBands {
		tasks = append(tasks, task{preview, b[0], b[1], true})
	}
	for _, b := range fullBands {
		tasks = append(tasks, task{full, b[0], b[1], false})
	}

	var cursor atomic.Int64
	var previewLeft atomic.Int64
	previewLeft.Store(int64(len(prevBands)))

	for i := 0; i < workers; i++ {
		go func() {
			for {
				k := cursor.Add(1) - 1
				if k >= int64(len(tasks)) || j.cancel.Load() {
					return
				}
				tk := tasks[k]
				tk.t.renderBand(tk.y0, tk.y1)
				if tk.isPreview {
					if previewLeft.Add(-1) == 0 {
						close(j.previewReady)
					}
				} else {
					j.Bands <- Band{Y0: tk.y0, Y1: tk.y1}
					if j.remaining.Add(-1) == 0 {
						// All bands have been sent; close both signals.
						close(j.done)
						close(j.Bands)
					}
				}
			}
		}()
	}
	return j
}
