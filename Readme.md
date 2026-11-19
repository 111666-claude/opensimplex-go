OpenSimplex in Go
=================


[![GoDoc](https://godoc.org/github.com/ojrac/opensimplex-go?status.svg)](http://godoc.org/github.com/ojrac/opensimplex-go)
[![Build Status](https://travis-ci.org/ojrac/opensimplex-go.svg?branch=master)](https://travis-ci.org/ojrac/opensimplex-go)

OpenSimplex noise is a random noise algorithm by Kurt Spencer, made as a
patent-free alternative to Perlin and Simplex noise. This Go port is based on
Kurt's [Java implementation](https://gist.github.com/KdotJPG/b1270127455a94ac5d19).

For an introduction to OpenSimplex noise, see [Kurt Spencer's
post](http://uniblock.tumblr.com/post/97868843242/noise) announcing it. If
you're not familiar with random noise, the Wikipedia post on [Perlin
noise](https://en.wikipedia.org/wiki/Perlin_noise) is a good introduction.


Why not Perlin noise?
---------------------

As Kurt explains [in his
post](http://uniblock.tumblr.com/post/97868843242/noise), Perlin noise tends to
generate noise with noticeable axis-aligned artifacts. Simplex noise fixes
these artifacts, but it's patented. OpenSimplex noise is for people who don't
want to deal with Simplex's patent.

The difference between Perlin and OpenSimplex noise is easiest to see in
pictures. This is Perlin noise, with a noticeable bias towards vertical and
horizontal artifacts:

![Perlin Noise sample](http://67.media.tumblr.com/153d6efb739746f114c52f86391c3932/tumblr_inline_nc4swj5tX51seaucq.png)

Here's what OpenSimplex noise looks like:

![OpenSimplex Noise sample](http://67.media.tumblr.com/6186a25f7bafb258c30101ee3c0c87b4/tumblr_inline_ngubweRMTr1seaucq.png)


Spherical textures
-------------------

Wrapping an ordinary 2D noise texture onto a sphere produces two visible
artifacts: a seam where the first and last meridians meet, and smeared
"spaghetti" lines at the poles, where every longitude converges on one point.
The `Sphere` sampler avoids both by sampling in 3D: longitude and latitude are
mapped onto a point of the unit sphere, and the existing `Eval3` noise is
evaluated at that point scaled by a spatial frequency.

```go
// Returns width*height values in row-major order, in [0, 1). Column 0 and the
// virtual column "width" match exactly, and every value in a pole row is
// identical, so the texture wraps onto a sphere without seams.
sphere, err := opensimplex.NewNormalizedSphere(seed, 4.0)
if err != nil { ... }
texture, err := sphere.Texture(512, 256)
if err != nil { ... }
```

Longitude is periodic with period 2π and latitude is reflected across the
poles, so out-of-range angles simply sample their in-range equivalent.
`NewSphere` provides the same sampler with raw (un-normalized) output, and
`EvalLonLat` evaluates a single longitude/latitude pair. Non-finite
frequencies and angles, non-positive or oversized texture dimensions have
defined results (sentinel errors or NaN) instead of panics.


Tests
-----------
This implementation of OpenSimplex's tests verify its output against the output
of the reference Java implementation. I haven't run these tests on different
architectures, so results may vary.

License
-------
This code is under the same "license" as Kurt's OpenSimplex - the public domain
"unlicense."

Next Steps
----------
* More documentation
* Benchmarks
