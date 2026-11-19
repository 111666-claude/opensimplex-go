package opensimplex

import (
	"errors"
	"math"
	"testing"
)

const sphereTestSeed = int64(42)

const sphereEpsilon = 1e-12

func TestNewSphereRejectsNonFiniteFrequency(t *testing.T) {
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range bad {
		if _, err := NewSphere(sphereTestSeed, f); !errors.Is(err, ErrInvalidFrequency) {
			t.Errorf("NewSphere(seed, %v): want ErrInvalidFrequency, got %v", f, err)
		}
		if _, err := NewNormalizedSphere(sphereTestSeed, f); !errors.Is(err, ErrInvalidFrequency) {
			t.Errorf("NewNormalizedSphere(seed, %v): want ErrInvalidFrequency, got %v", f, err)
		}
	}

	// Zero and negative frequencies are finite and must be accepted.
	for _, f := range []float64{0, -1, -1e9, 1e-9} {
		if _, err := NewSphere(sphereTestSeed, f); err != nil {
			t.Errorf("NewSphere(seed, %v): unexpected error %v", f, err)
		}
	}
}

// EvalLonLat must be exactly the underlying Eval3 evaluated at the
// frequency-scaled spherical point. This also guarantees that existing Eval3
// output is unchanged: the sphere code is a pure caller of Eval3.
func TestSphereUsesEval3AtSphericalPoint(t *testing.T) {
	n := New(sphereTestSeed)
	const freq = 2.5
	s, err := NewSphere(sphereTestSeed, freq)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ lon, lat float64 }{
		{0, 0},
		{1.2, -0.4},
		{-2.7, 0.9},
		{5.1, -1.1},
		{twoPi, math.Pi / 2},
	}
	for _, c := range cases {
		x, y, z := spherePoint(c.lon, c.lat)
		want := n.Eval3(x*freq, y*freq, z*freq)
		if got := s.EvalLonLat(c.lon, c.lat); got != want {
			t.Errorf("EvalLonLat(%v, %v) = %v, want Eval3 at spherical point %v",
				c.lon, c.lat, got, want)
		}
	}
}

// The prime meridian must not be a seam: longitudes one full turn apart,
// including the exact 0 vs 2π boundary used by a wrapped texture, must
// produce identical samples.
func TestSphereLongitudeSeamIsContinuous(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 3.0)
	if err != nil {
		t.Fatal(err)
	}

	lats := []float64{-1.2, -0.3, 0, 0.7, 1.3}
	for _, lat := range lats {
		// Exact boundary at the prime meridian: both sides must be bit-equal.
		if got, want := s.EvalLonLat(twoPi, lat), s.EvalLonLat(0, lat); got != want {
			t.Errorf("seam at lat %v: EvalLonLat(2π) = %v, EvalLonLat(0) = %v", lat, got, want)
		}
		if got, want := s.EvalLonLat(-twoPi, lat), s.EvalLonLat(0, lat); got != want {
			t.Errorf("seam at lat %v: EvalLonLat(-2π) = %v, EvalLonLat(0) = %v", lat, got, want)
		}

		// General periodicity: lon and lon+2π sample the same point. Floating
		// point rounding of large angles allows a tiny tolerance here.
		lon := 0.8
		if math.Abs(s.EvalLonLat(lon, lat)-s.EvalLonLat(lon+twoPi, lat)) > sphereEpsilon {
			t.Errorf("periodicity broken at lon %v, lat %v", lon, lat)
		}
		if math.Abs(s.EvalLonLat(lon, lat)-s.EvalLonLat(lon-3*twoPi, lat)) > sphereEpsilon {
			t.Errorf("periodicity broken at lon %v, lat %v", lon-3*twoPi, lat)
		}
	}
}

// A rendered texture must tile horizontally: the virtual column "width"
// wraps exactly onto column 0.
func TestTextureWrapsAcrossPrimeMeridian(t *testing.T) {
	const width, height = 64, 32
	s, err := NewSphere(sphereTestSeed, 4.0)
	if err != nil {
		t.Fatal(err)
	}
	tex, err := s.Texture(width, height)
	if err != nil {
		t.Fatal(err)
	}

	// Skip the pole rows, which are covered by TestSpherePolesAreSinglePoint.
	for row := 1; row < height-1; row++ {
		lat := math.Pi*float64(row)/float64(height-1) - math.Pi/2
		offset := row * width
		// The virtual column "width" is longitude 2π: it must match column 0
		// bit-for-bit, since Remainder wraps it exactly to zero.
		if got := s.EvalLonLat(twoPi, lat); got != tex[offset] {
			t.Errorf("row %d does not wrap: virtual column = %v, column 0 = %v",
				row, got, tex[offset])
		}

		// Deterministic seam check: the edge pair (last column, virtual
		// wrapped column) samples the same two physical points as the pair
		// (-2π/width, 0), approached entirely from the negative-longitude
		// side of the branch cut. Their jump can differ only by ulp-level
		// arithmetic noise; a genuine seam artifact would show up as a large,
		// systematic difference between the two.
		seamJump := math.Abs(tex[offset+width-1] - tex[offset])
		negJump := math.Abs(s.EvalLonLat(-twoPi/float64(width), lat) - s.EvalLonLat(0, lat))
		if math.Abs(seamJump-negJump) > sphereEpsilon {
			t.Errorf("row %d: seam jump %v differs from same-pair jump %v across the branch cut",
				row, seamJump, negJump)
		}
	}
}

// Every longitude at a pole must evaluate to exactly the same value, both
// through EvalLonLat and within rendered pole rows. This is what prevents
// pole artifacts when the texture is projected onto a sphere.
func TestSpherePolesAreSinglePoint(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 3.5)
	if err != nil {
		t.Fatal(err)
	}

	longitudes := []float64{0, 0.5, 1.0, math.Pi, -math.Pi, 3.7, -5.2, twoPi, 10 * twoPi}
	north := s.EvalLonLat(0, math.Pi/2)
	south := s.EvalLonLat(0, -math.Pi/2)

	// The two poles are distinct points of the sphere and must not collapse.
	if north == south {
		t.Fatalf("north and south poles coincide at %v", north)
	}

	for _, lon := range longitudes {
		if got := s.EvalLonLat(lon, math.Pi/2); got != north {
			t.Errorf("north pole varies with longitude %v: %v vs %v", lon, got, north)
		}
		if got := s.EvalLonLat(lon, -math.Pi/2); got != south {
			t.Errorf("south pole varies with longitude %v: %v vs %v", lon, got, south)
		}
	}

	// Out-of-range latitudes that land on a pole must also be exact.
	for _, lat := range []float64{3 * math.Pi / 2, -3 * math.Pi / 2, 5 * math.Pi / 2} {
		want := north
		if math.Sin(lat) < 0 {
			want = south
		}
		if got := s.EvalLonLat(1.23, lat); got != want {
			t.Errorf("out-of-range pole lat %v: got %v, want %v", lat, got, want)
		}
	}

	const width, height = 48, 24
	tex, err := s.Texture(width, height)
	if err != nil {
		t.Fatal(err)
	}
	for col := 1; col < width; col++ {
		if tex[col] != tex[0] {
			t.Errorf("south pole row varies at column %d", col)
		}
		top := (height - 1) * width
		if tex[top+col] != tex[top] {
			t.Errorf("north pole row varies at column %d", col)
		}
	}
	if tex[0] != south || tex[(height-1)*width] != north {
		t.Errorf("pole rows do not match direct pole samples")
	}
}

// Longitudes are wrapped and latitudes reflected, so out-of-range angles
// sample the same spherical point as their in-range equivalents.
func TestSphereOutOfRangeAngles(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 1.75)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		lon, lat, refLon, refLat float64
	}{
		// Longitude wrapping.
		{twoPi, 0.4, 0, 0.4},
		{-twoPi, 0.4, 0, 0.4},
		{7 * twoPi, -0.6, 0, -0.6},
		{-9 * twoPi, 0.2, 0, 0.2},
		// Latitude reflection across the poles.
		{0.6, math.Pi + 0.3, 0.6, -0.3},
		{0.6, -math.Pi - 0.3, 0.6, 0.3},
		{0.6, 2*twoPi + 0.3, 0.6, 0.3},
		{0.6, -2*twoPi - 0.3, 0.6, -0.3},
	}
	for _, c := range cases {
		got := s.EvalLonLat(c.lon, c.lat)
		want := s.EvalLonLat(c.refLon, c.refLat)
		if math.Abs(got-want) > sphereEpsilon {
			t.Errorf("EvalLonLat(%v, %v) = %v, want %v from (%v, %v)",
				c.lon, c.lat, got, want, c.refLon, c.refLat)
		}
	}
}

// spherePoint is the geometry under test: check its mapping and reflection
// rules directly, including unit length away from the exact poles.
func TestSpherePointGeometry(t *testing.T) {
	// Poles are snapped to exact axis points regardless of longitude.
	for _, lon := range []float64{0, 1.3, -2.2, twoPi} {
		if x, y, z := spherePoint(lon, math.Pi/2); x != 0 || y != 0 || z != 1 {
			t.Errorf("north pole at lon %v = (%v, %v, %v), want (0, 0, 1)", lon, x, y, z)
		}
		if x, y, z := spherePoint(lon, -math.Pi/2); x != 0 || y != 0 || z != -1 {
			t.Errorf("south pole at lon %v = (%v, %v, %v), want (0, 0, -1)", lon, x, y, z)
		}
	}

	// Exact axis points away from the poles.
	if x, y, z := spherePoint(0, 0); x != 1 || y != 0 || z != 0 {
		t.Errorf("spherePoint(0, 0) = (%v, %v, %v), want (1, 0, 0)", x, y, z)
	}
	if x, y, z := spherePoint(math.Pi, 0); x != -1 || z != 0 {
		t.Errorf("spherePoint(π, 0) = (%v, %v, %v), want (-1, ~0, 0)", x, y, z)
	}

	// General cardinal points: trig functions of π/2 carry floating-point
	// residuals, so compare with a tolerance.
	cardinal := []struct {
		lon, lat, x, y, z float64
	}{
		{math.Pi / 2, 0, 0, 1, 0},
		{-math.Pi / 2, 0, 0, -1, 0},
		{0.6, 0.4, math.Cos(0.4) * math.Cos(0.6), math.Cos(0.4) * math.Sin(0.6), math.Sin(0.4)},
	}
	for _, c := range cardinal {
		x, y, z := spherePoint(c.lon, c.lat)
		if math.Abs(x-c.x) > sphereEpsilon || math.Abs(y-c.y) > sphereEpsilon || math.Abs(z-c.z) > sphereEpsilon {
			t.Errorf("spherePoint(%v, %v) = (%v, %v, %v), want (%v, %v, %v)",
				c.lon, c.lat, x, y, z, c.x, c.y, c.z)
		}
	}

	// General points must lie on the unit sphere.
	for _, lat := range []float64{-1.3, -0.2, 0.4, 1.1} {
		for _, lon := range []float64{-2.9, -0.7, 0.5, 2.2} {
			x, y, z := spherePoint(lon, lat)
			if d := math.Abs(x*x + y*y + z*z - 1); d > sphereEpsilon {
				t.Errorf("spherePoint(%v, %v) not unit length: deviation %v", lon, lat, d)
			}
		}
	}

	// Wrapped/reflected out-of-range angles give the same point as the
	// equivalent in-range angle.
	x1, y1, z1 := spherePoint(twoPi+0.9, math.Pi+0.4)
	x2, y2, z2 := spherePoint(0.9, -0.4)
	if math.Abs(x1-x2) > sphereEpsilon || math.Abs(y1-y2) > sphereEpsilon || math.Abs(z1-z2) > sphereEpsilon {
		t.Errorf("out-of-range point (%v, %v, %v) != in-range (%v, %v, %v)",
			x1, y1, z1, x2, y2, z2)
	}

	// Non-finite input yields NaN, never a panic.
	for _, c := range []struct{ lon, lat float64 }{
		{math.NaN(), 0},
		{0, math.NaN()},
		{math.Inf(1), 0},
		{0, math.Inf(-1)},
	} {
		x, y, z := spherePoint(c.lon, c.lat)
		if !math.IsNaN(x) || !math.IsNaN(y) || !math.IsNaN(z) {
			t.Errorf("spherePoint(%v, %v) = (%v, %v, %v), want NaNs", c.lon, c.lat, x, y, z)
		}
	}
}

// Frequency must scale the spherical point before Eval3 is called, and
// different frequencies must produce different textures. Zero collapses the
// texture to the noise value at the origin; a negative frequency mirrors it.
func TestSphereFrequency(t *testing.T) {
	base := New(sphereTestSeed)
	origin := base.Eval3(0, 0, 0)

	zero, err := NewSphere(sphereTestSeed, 0)
	if err != nil {
		t.Fatal(err)
	}
	tex, err := zero.Texture(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range tex {
		if v != origin {
			t.Fatalf("zero frequency: pixel %d = %v, want origin value %v", i, v, origin)
		}
	}

	neg, err := NewSphere(sphereTestSeed, -2.0)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := NewSphere(sphereTestSeed, 2.0)
	if err != nil {
		t.Fatal(err)
	}
	lon, lat := 0.9, -0.5
	// Negating the frequency negates the point handed to Eval3.
	x, y, z := spherePoint(lon, lat)
	if got, want := neg.EvalLonLat(lon, lat), base.Eval3(-2*x, -2*y, -2*z); got != want {
		t.Errorf("negative frequency = %v, want %v", got, want)
	}
	if neg.EvalLonLat(lon, lat) == pos.EvalLonLat(lon, lat) {
		t.Errorf("positive and negative frequencies produced identical output")
	}

	coarse, err := NewSphere(sphereTestSeed, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	fine, err := NewSphere(sphereTestSeed, 8.0)
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := 0; i < 32; i++ {
		lon := twoPi * float64(i) / 32
		if coarse.EvalLonLat(lon, 0.5) != fine.EvalLonLat(lon, 0.5) {
			same = false
			break
		}
	}
	if same {
		t.Errorf("frequencies 1.0 and 8.0 produced identical textures")
	}

	if f := pos.Frequency(); f != 2.0 {
		t.Errorf("Frequency() = %v, want 2.0", f)
	}
}

// The normalized constructor must keep every sample in [0, 1) while retaining
// the seam and pole continuity guarantees.
func TestNormalizedSphereRange(t *testing.T) {
	s, err := NewNormalizedSphere(sphereTestSeed, 5.0)
	if err != nil {
		t.Fatal(err)
	}
	tex, err := s.Texture(64, 32)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range tex {
		if math.IsNaN(v) || v < 0 || v >= 1 {
			t.Errorf("normalized pixel %d out of [0, 1): %v", i, v)
		}
	}
	// Normalized pole rows are still constant.
	for col := 1; col < 64; col++ {
		if tex[col] != tex[0] {
			t.Errorf("normalized south pole row varies at column %d", col)
		}
	}
}

// Texture dimensions: zero/negative and oversized requests must fail with
// ErrInvalidTextureSize, including sizes whose product would overflow on a
// 32-bit host.
func TestTextureRejectsBadSizes(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	bad := []struct{ w, h int }{
		{0, 10},
		{10, 0},
		{-1, 10},
		{10, -1},
		{maxTextureElements + 1, 2},
		{2, maxTextureElements + 1},
		// Product overflows int32 even though each dimension is legal.
		{1 << 16, 1 << 16},
		{math.MaxInt32, 3},
	}
	for _, c := range bad {
		if tex, err := s.Texture(c.w, c.h); !errors.Is(err, ErrInvalidTextureSize) {
			t.Errorf("Texture(%d, %d): want ErrInvalidTextureSize, got (%v, %v)",
				c.w, c.h, tex, err)
		}
	}

	// The largest legal size at the boundary is accepted without allocating
	// the whole thing: check a boundary-sized product is rejected while a
	// smaller normal texture works.
	if _, err := s.Texture(16, 8); err != nil {
		t.Errorf("Texture(16, 8): unexpected error %v", err)
	}
}

// A single-row texture has no pole rows and must sample the equator.
func TestTextureSingleRowSamplesEquator(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 2.25)
	if err != nil {
		t.Fatal(err)
	}
	const width = 16
	tex, err := s.Texture(width, 1)
	if err != nil {
		t.Fatal(err)
	}
	for col := 0; col < width; col++ {
		lon := twoPi * float64(col) / width
		if want := s.EvalLonLat(lon, 0); tex[col] != want {
			t.Errorf("single-row pixel %d = %v, want equator sample %v", col, tex[col], want)
		}
	}
}

// Textures are deterministic for a fixed seed and the layout is row-major.
func TestTextureDeterministicAndRowMajor(t *testing.T) {
	const width, height = 24, 12
	a, err := NewSphere(sphereTestSeed, 3.25)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSphere(sphereTestSeed, 3.25)
	if err != nil {
		t.Fatal(err)
	}
	texA, err := a.Texture(width, height)
	if err != nil {
		t.Fatal(err)
	}
	texB, err := b.Texture(width, height)
	if err != nil {
		t.Fatal(err)
	}
	if len(texA) != width*height {
		t.Fatalf("texture length = %d, want %d", len(texA), width*height)
	}
	for i := range texA {
		if texA[i] != texB[i] {
			t.Fatalf("textures for the same seed differ at pixel %d", i)
		}
	}

	// Verify row-major layout against direct EvalLonLat calls, including the
	// exact pole rows at both ends.
	for row := 0; row < height; row++ {
		lat := math.Pi*float64(row)/float64(height-1) - math.Pi/2
		for col := 0; col < width; col++ {
			lon := twoPi * float64(col) / width
			if want := a.EvalLonLat(lon, lat); texA[row*width+col] != want {
				t.Errorf("layout mismatch at (%d, %d): %v vs %v",
					col, row, texA[row*width+col], want)
			}
		}
	}
}

// Non-finite angles propagate NaN rather than panicking or returning a
// silently wrong finite sample.
func TestSphereNonFiniteAngles(t *testing.T) {
	s, err := NewSphere(sphereTestSeed, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ lon, lat float64 }{
		{math.NaN(), 0},
		{0, math.NaN()},
		{math.Inf(1), 0},
		{0, math.Inf(-1)},
	} {
		if got := s.EvalLonLat(c.lon, c.lat); !math.IsNaN(got) {
			t.Errorf("EvalLonLat(%v, %v) = %v, want NaN", c.lon, c.lat, got)
		}
	}
}

func BenchmarkSphereTexture(b *testing.B) {
	s, err := NewSphere(0, 4.0)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.Texture(256, 128); err != nil {
			b.Fatal(err)
		}
	}
}
