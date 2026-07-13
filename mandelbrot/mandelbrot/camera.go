package mandelbrot

// Camera manages the view into the complex plane, supporting pan and zoom.
// It maps between pixel coordinates and complex plane coordinates.
type Camera struct {
	// CenterX, CenterY define the center of the view in the complex plane.
	CenterX float64
	CenterY float64
	// Zoom defines the scale: at zoom=1.0 the view spans ~3.5 units horizontally.
	Zoom float64
	// AspectRatio is width / height of the viewport.
	AspectRatio float64
}

// NewCamera creates a camera with default Mandelbrot framing.
// The classic view shows the real axis [-2.5, 1.0] and imag axis [-1.0, 1.0].
func NewCamera(width, height int) *Camera {
	aspect := float64(width) / float64(height)
	return &Camera{
		CenterX:     -0.75,
		CenterY:     0.0,
		Zoom:        1.0,
		AspectRatio: aspect,
	}
}

// ViewWidth returns the visible width in the complex plane.
func (cam *Camera) ViewWidth() float64 {
	return 3.5 / cam.Zoom
}

// ViewHeight returns the visible height in the complex plane.
func (cam *Camera) ViewHeight() float64 {
	return cam.ViewWidth() / cam.AspectRatio
}

// ScreenToComplex maps a pixel coordinate to a point on the complex plane.
func (cam *Camera) ScreenToComplex(px, py int, screenW, screenH int) complex128 {
	real := cam.CenterX + (float64(px)/float64(screenW)-0.5)*cam.ViewWidth()
	imag := cam.CenterY + (float64(py)/float64(screenH)-0.5)*cam.ViewHeight()
	return complex(real, imag)
}

// ZoomAt adjusts the zoom level and shifts the center so that the point
// at (px, py) in screen coordinates remains fixed in the complex plane.
func (cam *Camera) ZoomAt(factor float64, px, py int, screenW, screenH int) {
	// Record the complex point under the cursor BEFORE changing zoom.
	cx := cam.CenterX + (float64(px)/float64(screenW)-0.5)*cam.ViewWidth()
	cy := cam.CenterY + (float64(py)/float64(screenH)-0.5)*cam.ViewHeight()

	// Apply zoom.
	cam.Zoom *= factor
	if cam.Zoom < 0.01 {
		cam.Zoom = 0.01
	}
	if cam.Zoom > 1e12 {
		cam.Zoom = 1e12
	}

	// Adjust center so the complex point under cursor stays in place.
	newW := cam.ViewWidth()
	newH := cam.ViewHeight()
	cam.CenterX = cx - (float64(px)/float64(screenW)-0.5)*newW
	cam.CenterY = cy - (float64(py)/float64(screenH)-0.5)*newH
}

// Reset restores the camera to the default view.
func (cam *Camera) Reset(width, height int) {
	cam.CenterX = -0.75
	cam.CenterY = 0.0
	cam.Zoom = 1.0
	cam.AspectRatio = float64(width) / float64(height)
}
