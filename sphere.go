package opensimplex

import (
	"errors"
	"math"
)

// maxTextureElements is the largest number of pixels that Texture will
// allocate. It is independent of the host int width so that callers on both
// 32-bit and 64-bit platforms get the same, well-defined rejection of huge
// textures instead of an allocation panic.
const maxTextureElements = 1<<31 - 1

const twoPi = 2 * math.Pi

// Sentinel errors returned by the spherical texture API. They are returned
// directly so callers can check them with errors.Is.
var (
	// ErrInvalidFrequency is returned when a sphere is constructed with a
	// non-finite frequency (NaN or ±Inf).
	ErrInvalidFrequency = errors.New("opensimplex: frequency must be a finite number")

	// ErrInvalidTextureSize is returned when Texture is called with a
	// non-positive width or height, or when the requested texture contains
	// more than maxTextureElements pixels.
	ErrInvalidTextureSize = errors.New("opensimplex: texture width and height must be positive and contain at most 2^31-1 pixels")
)

// Sphere samples OpenSimplex noise over the surface of a unit sphere.
//
// Longitude and latitude are mapped onto a point of the unit sphere, and the
// existing 3D noise is evaluated at that point scaled by the spatial
// frequency. Because sampling happens in 3D space rather than on a flat
// longitude/latitude grid, the noise is continuous everywhere on the sphere:
// longitude wraps without a seam at the prime meridian, and every longitude
// at a given pole evaluates to exactly the same value.
//
// Geographical conventions:
//
//   - Longitude is in radians and is periodic with period 2π: any finite
//     longitude is wrapped into [-π, π], so lon and lon+2π sample the same
//     point. The seam at ±π therefore cannot produce a discontinuity.
//   - Latitude is in radians in [-π/2, π/2] (south pole to north pole).
//     Values outside that range are reflected across the poles rather than
//     clamped, so crossing a pole keeps the sample continuous. A latitude of
//     exactly ±π/2 is snapped to the pole point (0, 0, ±1), independent of
//     the longitude.
//   - Non-finite longitude or latitude (NaN or ±Inf) yield NaN.
//
// A zero frequency collapses every sample to the noise at the origin, so a
// texture is uniformly constant. A negative frequency mirrors the noise
// through the origin; it is accepted and produces a valid but different
// pattern.
type Sphere struct {
	noise     Noise
	frequency float64
}

// NewSphere constructs a spherical sampler backed by a regular Noise instance
// with the given 64-bit seed. frequency is the spatial frequency: it scales
// the unit-sphere sample point before Eval3 is called, so larger values yield
// finer-grained features. A non-finite frequency returns ErrInvalidFrequency;
// zero and negative frequencies are accepted (see the Sphere documentation).
func NewSphere(seed int64, frequency float64) (*Sphere, error) {
	if !isFinite(frequency) {
		return nil, ErrInvalidFrequency
	}
	return &Sphere{noise: New(seed), frequency: frequency}, nil
}

// NewNormalizedSphere constructs a spherical sampler backed by a normalized
// Noise instance. EvalLonLat and Texture return values in [0, 1). It behaves
// identically to NewSphere in every other respect.
func NewNormalizedSphere(seed int64, frequency float64) (*Sphere, error) {
	if !isFinite(frequency) {
		return nil, ErrInvalidFrequency
	}
	return &Sphere{noise: NewNormalized(seed), frequency: frequency}, nil
}

// Frequency returns the spatial frequency used by this sampler.
func (s *Sphere) Frequency() float64 {
	return s.frequency
}

// EvalLonLat evaluates the noise at a point of the unit sphere identified by
// longitude and latitude in radians. Longitude may be any finite value (it is
// wrapped with period 2π), and latitude is reflected into [-π/2, π/2]. The
// result is exactly the output of the underlying Noise's Eval3 at the
// frequency-scaled spherical point; a non-finite argument yields NaN.
func (s *Sphere) EvalLonLat(lon, lat float64) float64 {
	// Eval3 cannot be relied on to propagate NaN: a non-finite angle reaches
	// its float-to-int lattice conversions with platform-defined results, so
	// handle it explicitly here.
	if !isFinite(lon) || !isFinite(lat) {
		return math.NaN()
	}
	x, y, z := spherePoint(lon, lat)
	f := s.frequency
	return s.noise.Eval3(x*f, y*f, z*f)
}

// Texture renders an equirectangular (plate carrée) noise texture of the
// requested width and height, returning width*height values in row-major
// order. Column x samples longitude 2π*x/width, so column 0 sits on the prime
// meridian and the virtual column "width" wraps exactly back to column 0.
// Row 0 is the south pole and the last row is the north pole; every value in
// a pole row is bit-identical. A texture with height 1 samples the equator.
//
// It returns ErrInvalidTextureSize for a non-positive width or height, or
// when the texture would contain more than maxTextureElements pixels.
func (s *Sphere) Texture(width, height int) ([]float64, error) {
	if width <= 0 || height <= 0 ||
		int64(width) > maxTextureElements || int64(height) > maxTextureElements {
		return nil, ErrInvalidTextureSize
	}

	elements := int64(width) * int64(height)
	if elements > maxTextureElements {
		return nil, ErrInvalidTextureSize
	}

	texture := make([]float64, elements)
	for row := 0; row < height; row++ {
		// A single row has no pole information; sample the equator.
		lat := 0.0
		if height > 1 {
			lat = math.Pi*float64(row)/float64(height-1) - math.Pi/2
		}

		offset := row * width
		for col := 0; col < width; col++ {
			lon := twoPi * float64(col) / float64(width)
			texture[offset+col] = s.EvalLonLat(lon, lat)
		}
	}
	return texture, nil
}

// spherePoint maps longitude and latitude in radians onto a point of the unit
// sphere using the conventional geographical mapping
//
//	x = cos(lat) cos(lon), y = cos(lat) sin(lon), z = sin(lat).
//
// Longitude is wrapped with period 2π and latitude is reflected into
// [-π/2, π/2], so every finite (lon, lat) maps to a well-defined spherical
// point and out-of-range coordinates sample the same point as their in-range
// equivalents. The poles are returned as the exact points (0, 0, ±1), which
// makes all longitudes at a pole bit-identical instead of differing by
// floating-point error. A non-finite argument yields (NaN, NaN, NaN).
func spherePoint(lon, lat float64) (x, y, z float64) {
	if math.IsNaN(lon) || math.IsNaN(lat) || math.IsInf(lon, 0) || math.IsInf(lat, 0) {
		return math.NaN(), math.NaN(), math.NaN()
	}

	// Remainder maps both angles into [-π, π].
	lon = math.Remainder(lon, twoPi)
	lat = math.Remainder(lat, twoPi)

	// Reflect latitude back across the poles into [-π/2, π/2]: traveling past
	// the north pole heads back down the opposite meridian, and vice versa.
	if lat > math.Pi/2 {
		lat = math.Pi - lat
	} else if lat < -math.Pi/2 {
		lat = -math.Pi - lat
	}

	// Snap the poles to exact axis points; cos(±π/2) is not zero in floating
	// point, which would otherwise smear the pole across longitudes.
	if lat >= math.Pi/2 {
		return 0, 0, 1
	}
	if lat <= -math.Pi/2 {
		return 0, 0, -1
	}

	cosLat := math.Cos(lat)
	x = cosLat * math.Cos(lon)
	y = cosLat * math.Sin(lon)
	z = math.Sin(lat)
	return x, y, z
}

// isFinite reports whether f is neither NaN nor an infinity.
func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}
