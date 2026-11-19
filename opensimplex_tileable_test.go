package opensimplex

import (
	"math"
	"testing"
)

const (
	tileSeed  = int64(1337)
	tileW     = 64
	tileH     = 48
	tileXPer  = 3.0
	tileYPer  = 2.5
	tileEps   = 1e-9
	tileTestW = tileW
	tileTestH = tileH
)

func mustTileable(t *testing.T, xPeriod, yPeriod float64) *TileableNoise {
	t.Helper()
	tn, err := NewTileableNoise(tileSeed, xPeriod, yPeriod)
	if err != nil {
		t.Fatalf("NewTileableNoise(%v, %v): %v", xPeriod, yPeriod, err)
	}
	return tn
}

func mustTexture(t *testing.T, tn *TileableNoise, w, h int, normalize bool) []float64 {
	t.Helper()
	tex, err := tn.Texture(w, h, normalize)
	if err != nil {
		t.Fatalf("Texture(%d, %d): %v", w, h, err)
	}
	return tex
}

// TestTileableHorizontalSeam checks that the right edge of a texture joins
// the left edge of the next tiled copy: sampling one full period past pixel 0
// returns pixel 0 again, and the step across the seam is no larger than the
// steps inside the texture.
func TestTileableHorizontalSeam(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)
	tex := mustTexture(t, tn, tileTestW, tileTestH, false)

	for y := 0; y < tileTestH; y++ {
		yu := float64(y) * tileYPer / float64(tileTestH)
		first := tex[y*tileTestW]
		last := tex[y*tileTestW+tileTestW-1]

		// The column one full period to the right must equal column 0.
		wrapped := tn.Eval2(tileXPer, yu)
		if math.Abs(wrapped-first) > tileEps {
			t.Fatalf("row %d: Eval2(x+period)=%v, texture edge=%v", y, wrapped, first)
		}

		// Local variation across the seam must look like interior steps:
		// the jump from the last column to the first column of the adjacent
		// tile (which equals this tile's first column) must not stand out.
		maxStep := 0.0
		for x := 1; x < tileTestW; x++ {
			step := math.Abs(tex[y*tileTestW+x] - tex[y*tileTestW+x-1])
			if step > maxStep {
				maxStep = step
			}
		}
		seamStep := math.Abs(first - last)
		if seamStep > 2*maxStep {
			t.Fatalf("row %d: seam step %v exceeds interior max step %v", y, seamStep, maxStep)
		}
	}
}

// TestTileableVerticalSeam mirrors TestTileableHorizontalSeam for the
// top/bottom edges.
func TestTileableVerticalSeam(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)
	tex := mustTexture(t, tn, tileTestW, tileTestH, false)

	for x := 0; x < tileTestW; x++ {
		xu := float64(x) * tileXPer / float64(tileTestW)
		first := tex[x]
		last := tex[(tileTestH-1)*tileTestW+x]

		wrapped := tn.Eval2(xu, tileYPer)
		if math.Abs(wrapped-first) > tileEps {
			t.Fatalf("col %d: Eval2(y+period)=%v, texture edge=%v", x, wrapped, first)
		}

		maxStep := 0.0
		for y := 1; y < tileTestH; y++ {
			step := math.Abs(tex[y*tileTestW+x] - tex[(y-1)*tileTestW+x])
			if step > maxStep {
				maxStep = step
			}
		}
		seamStep := math.Abs(first - last)
		if seamStep > 2*maxStep {
			t.Fatalf("col %d: seam step %v exceeds interior max step %v", x, seamStep, maxStep)
		}
	}
}

// TestTileableCorners checks that the four corners, where the horizontal and
// vertical wraps meet, are continuous too.
func TestTileableCorners(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)
	tex := mustTexture(t, tn, tileTestW, tileTestH, false)

	origin := tex[0]
	corners := []struct {
		name string
		x, y float64
	}{
		{"(+xPer, 0)", tileXPer, 0},
		{"(0, +yPer)", 0, tileYPer},
		{"(+xPer, +yPer)", tileXPer, tileYPer},
		{"(-xPer, -yPer)", -tileXPer, -tileYPer},
		{"(+2xPer, +3yPer)", 2 * tileXPer, 3 * tileYPer},
	}
	for _, c := range corners {
		got := tn.Eval2(c.x, c.y)
		if math.Abs(got-origin) > tileEps {
			t.Errorf("corner %s: Eval2=%v, want %v", c.name, got, origin)
		}
	}
}

// TestTileableIntegerPeriodBoundaries checks that integer multiples of the
// period land on (nearly) the same value as the origin.
func TestTileableIntegerPeriodBoundaries(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)
	base := tn.Eval2(1.25, 0.75)
	for k := 1; k <= 4; k++ {
		got := tn.Eval2(1.25+float64(k)*tileXPer, 0.75-float64(k)*tileYPer)
		if math.Abs(got-base) > 1e-7 {
			t.Fatalf("k=%d: Eval2=%v, want %v", k, got, base)
		}
	}
}

// TestTileableSingleAxis checks that a texture with only one periodic axis
// wraps on that axis and samples the other linearly.
func TestTileableSingleAxis(t *testing.T) {
	// Only x periodic.
	tn := mustTileable(t, tileXPer, 0)
	if !tn.Periodic(0) || tn.Periodic(1) {
		t.Fatal("expected only the x axis to be periodic")
	}
	tex := mustTexture(t, tn, tileTestW, tileTestH, false)
	for y := 0; y < tileTestH; y++ {
		yu := float64(y) * tileXPer / float64(tileTestH) // isotropic scale
		wrapped := tn.Eval2(tileXPer, yu)
		if math.Abs(wrapped-tex[y*tileTestW]) > tileEps {
			t.Fatalf("row %d: horizontal seam mismatch: %v vs %v", y, wrapped, tex[y*tileTestW])
		}
	}
	// The vertical axis must NOT wrap.
	if tn.Eval2(0.3, 0) == tn.Eval2(0.3, tileYPer) {
		t.Fatal("non-periodic y axis unexpectedly repeats")
	}

	// Only y periodic.
	tn = mustTileable(t, 0, tileYPer)
	if tn.Periodic(0) || !tn.Periodic(1) {
		t.Fatal("expected only the y axis to be periodic")
	}
	tex = mustTexture(t, tn, tileTestW, tileTestH, false)
	for x := 0; x < tileTestW; x++ {
		xu := float64(x) * tileYPer / float64(tileTestW)
		wrapped := tn.Eval2(xu, tileYPer)
		if math.Abs(wrapped-tex[x]) > tileEps {
			t.Fatalf("col %d: vertical seam mismatch: %v vs %v", x, wrapped, tex[x])
		}
	}
}

// TestTileableNormalized checks normalized output stays in [0, 1).
func TestTileableNormalized(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)
	tex := mustTexture(t, tn, tileTestW, tileTestH, true)
	for i, v := range tex {
		if v < 0 || v >= 1 {
			t.Fatalf("normalized value at %d out of range: %v", i, v)
		}
	}
}

// TestTileableInvalidPeriods checks zero/negative/non-finite period handling.
func TestTileableInvalidPeriods(t *testing.T) {
	bad := [][2]float64{
		{0, 0},
		{-1, 0},
		{0, -2.5},
		{-1, -1},
		{math.NaN(), 1},
		{1, math.NaN()},
		{math.Inf(1), 1},
		{1, math.Inf(-1)},
	}
	for _, p := range bad {
		if _, err := NewTileableNoise(tileSeed, p[0], p[1]); err == nil {
			t.Errorf("NewTileableNoise(%v, %v): expected error", p[0], p[1])
		}
	}
}

// TestTileableTextureSizes checks tiny and oversized texture dimensions.
func TestTileableTextureSizes(t *testing.T) {
	tn := mustTileable(t, tileXPer, tileYPer)

	// Degenerate sizes are errors.
	for _, wh := range [][2]int{{0, 10}, {10, 0}, {-1, 10}, {10, -1}} {
		if _, err := tn.Texture(wh[0], wh[1], false); err == nil {
			t.Errorf("Texture(%d, %d): expected error", wh[0], wh[1])
		}
	}

	// Oversized textures are rejected before allocation.
	if _, err := tn.Texture(MaxTilePixels, 2, false); err == nil {
		t.Error("Texture(MaxTilePixels, 2): expected error")
	}
	if _, err := tn.Texture(1<<20, 1<<20, false); err == nil {
		t.Error("Texture(1<<20, 1<<20): expected error")
	}

	// Minimal sizes still work.
	tex, err := tn.Texture(1, 1, false)
	if err != nil || len(tex) != 1 {
		t.Fatalf("Texture(1, 1): %v, len=%d", err, len(tex))
	}
	if got := tn.Eval2(0, 0); got != tex[0] {
		t.Fatalf("Texture(1, 1)=%v, Eval2(0,0)=%v", tex[0], got)
	}
	tex, err = tn.Texture(1, 7, false)
	if err != nil || len(tex) != 7 {
		t.Fatalf("Texture(1, 7): %v, len=%d", err, len(tex))
	}
}

// TestTileableDeterministic checks that the fixed seed makes textures
// reproducible and distinct from other seeds.
func TestTileableDeterministic(t *testing.T) {
	a := mustTileable(t, tileXPer, tileYPer)
	b := mustTileable(t, tileXPer, tileYPer)
	ta := mustTexture(t, a, 16, 16, false)
	tb := mustTexture(t, b, 16, 16, false)
	for i := range ta {
		if ta[i] != tb[i] {
			t.Fatal("same seed produced different textures")
		}
	}

	c, err := NewTileableNoise(tileSeed+1, tileXPer, tileYPer)
	if err != nil {
		t.Fatal(err)
	}
	tc := mustTexture(t, c, 16, 16, false)
	same := true
	for i := range ta {
		if ta[i] != tc[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different seeds produced identical textures")
	}
}
