package opensimplex

import (
	"errors"
	"fmt"
	"math"
)

// MaxTilePixels bounds the number of pixels (width*height) a tileable texture
// may contain. Sampling a larger texture returns an error instead of
// attempting a huge allocation.
const MaxTilePixels = 1 << 28

// twoPi is 2*math.Pi, the angular distance of one full period.
const twoPi = 2 * math.Pi

// TileableNoise samples seamlessly repeating 2D noise. Each enabled axis is
// mapped onto a circle in 4D space (x -> (eval4.x, eval4.y), y -> (eval4.z,
// eval4.w)) and evaluated with Eval4, so the noise repeats exactly every
// period along that axis and tiled copies join without seams — including at
// the four corners where horizontal and vertical wraps meet.
//
// A TileableNoise must have at least one periodic axis; construct it with
// NewTileableNoise.
type TileableNoise struct {
	noise Noise

	// xPeriod and yPeriod are the repeat lengths in noise units. A
	// non-positive period means the axis is not periodic and is sampled
	// along a straight line instead of a circle.
	xPeriod float64
	yPeriod float64

	// Circle radii for the periodic axes: period / 2π, so one trip around
	// the circle covers exactly one period of noise space.
	xRadius float64
	yRadius float64
}

// NewTileableNoise constructs a TileableNoise from a fixed seed and the
// horizontal (x) and vertical (y) periods, in noise units.
//
// A period <= 0 disables periodicity on that axis: the axis is sampled
// linearly, using the same noise-units-per-pixel scale as the periodic axis
// when sampling a texture. A period that is NaN or ±Inf is rejected, as is a
// configuration where both axes are disabled.
func NewTileableNoise(seed int64, xPeriod, yPeriod float64) (*TileableNoise, error) {
	if math.IsNaN(xPeriod) || math.IsInf(xPeriod, 0) ||
		math.IsNaN(yPeriod) || math.IsInf(yPeriod, 0) {
		return nil, fmt.Errorf("opensimplex: tileable periods must be finite, got x=%v y=%v", xPeriod, yPeriod)
	}
	if xPeriod <= 0 && yPeriod <= 0 {
		return nil, errors.New("opensimplex: tileable noise needs a positive period on at least one axis")
	}

	t := &TileableNoise{
		noise:   New(seed),
		xPeriod: xPeriod,
		yPeriod: yPeriod,
	}
	if xPeriod > 0 {
		t.xRadius = xPeriod / twoPi
	}
	if yPeriod > 0 {
		t.yRadius = yPeriod / twoPi
	}
	return t, nil
}

// Periodic reports whether the given axis wraps: axis 0 is x (horizontal),
// axis 1 is y (vertical).
func (t *TileableNoise) Periodic(axis int) bool {
	if axis == 0 {
		return t.xPeriod > 0
	}
	return t.yPeriod > 0
}

// Periods returns the horizontal and vertical periods, in noise units. A
// non-positive value marks a non-periodic axis.
func (t *TileableNoise) Periods() (xPeriod, yPeriod float64) {
	return t.xPeriod, t.yPeriod
}

// Eval2 samples the tileable noise at (x, y), in noise units. Coordinates on
// periodic axes wrap every period: Eval2(x + k*period, y) == Eval2(x, y) for
// any integer k (up to floating-point rounding), so adjacent tiled copies are
// continuous across every seam and corner. Non-periodic axes are sampled
// linearly.
func (t *TileableNoise) Eval2(x, y float64) float64 {
	var a, b, c, d float64

	if t.xPeriod > 0 {
		angle := twoPi * x / t.xPeriod
		a = t.xRadius * math.Cos(angle)
		b = t.xRadius * math.Sin(angle)
	} else {
		a = x
	}

	if t.yPeriod > 0 {
		angle := twoPi * y / t.yPeriod
		c = t.yRadius * math.Cos(angle)
		d = t.yRadius * math.Sin(angle)
	} else {
		c = y
	}

	return t.noise.Eval4(a, b, c, d)
}

// Eval2Normalized is Eval2 with the output mapped to [0, 1) using the Eval4
// normalization constants.
func (t *TileableNoise) Eval2Normalized(x, y float64) float64 {
	return (t.Eval2(x, y) + normMin4) * normScale4
}

// Texture samples a width*height grid of tileable noise, row-major (pixel
// (x, y) at index y*width+x). The grid spans exactly one period on each
// periodic axis, so the left and right edges join seamlessly, as do the top
// and bottom edges and all four corners. On a non-periodic axis the grid
// spans the same noise-units-per-pixel scale as the periodic axis.
//
// If normalize is true, values are mapped to [0, 1); otherwise they are raw
// Eval4 outputs.
//
// Width and height must be at least 1 and their product must not exceed
// MaxTilePixels; otherwise an error is returned.
func (t *TileableNoise) Texture(width, height int, normalize bool) ([]float64, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("opensimplex: tileable texture size must be at least 1x1, got %dx%d", width, height)
	}
	// Compute the pixel count in int64 so oversized dimensions overflow
	// neither the comparison nor the allocation below.
	pixels := int64(width) * int64(height)
	if pixels > MaxTilePixels {
		return nil, fmt.Errorf("opensimplex: tileable texture %dx%d exceeds MaxTilePixels (%d)", width, height, MaxTilePixels)
	}

	// Noise units per pixel on each axis. A periodic axis spans exactly one
	// period across the texture; a non-periodic axis inherits the periodic
	// axis's scale so sampling stays isotropic.
	xScale := t.xPeriod / float64(width)
	yScale := t.yPeriod / float64(height)
	if t.xPeriod <= 0 {
		xScale = yScale * float64(height) / float64(width)
	}
	if t.yPeriod <= 0 {
		yScale = xScale * float64(width) / float64(height)
	}

	out := make([]float64, pixels)
	for y := 0; y < height; y++ {
		yu := float64(y) * yScale
		row := out[y*width : (y+1)*width]
		for x := 0; x < width; x++ {
			xu := float64(x) * xScale
			if normalize {
				row[x] = t.Eval2Normalized(xu, yu)
			} else {
				row[x] = t.Eval2(xu, yu)
			}
		}
	}
	return out, nil
}
