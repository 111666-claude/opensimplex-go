package opensimplex

import "math/rand"

func Example() {
	noise := New(rand.Int63())

	w, h := 100, 100
	heightmap := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			xFloat := float64(x) / float64(w)
			yFloat := float64(y) / float64(h)
			heightmap[(y*w)+x] = noise.Eval2(xFloat, yFloat)
		}
	}
}

// Render an equirectangular texture that tiles seamlessly when wrapped onto a
// sphere: the prime meridian and both poles are continuous.
func ExampleSphere() {
	const seed = int64(1234)
	const width, height = 512, 256

	sphere, err := NewNormalizedSphere(seed, 4.0)
	if err != nil {
		panic(err)
	}
	texture, err := sphere.Texture(width, height)
	if err != nil {
		panic(err)
	}
	_ = texture
}
