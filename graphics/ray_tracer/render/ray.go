package render

// Ray is a half-line shot through the scene
type Ray struct {
	Origin, Dir Vec3
}

// WeightedRay is a ray annotated for wavefront tracing: it remembers which
// sample of the tile it contributes to and how much of that sample's final
// color it still carries (the weight shrinks with every reflection)
type WeightedRay struct {
	Ray
	SampleIndex int
	Weight      float64
}

// samplesPerAxis controls antialiasing: each pixel is sampled
// samplesPerAxis x samplesPerAxis times and the results are averaged
const samplesPerAxis = 2

// SamplesPerPixel is how many rays GenerateRays emits per pixel
const SamplesPerPixel = samplesPerAxis * samplesPerAxis

// GenerateRays builds the primary rays (weight 1) for the pixel rows
// [yFrom, yTo) of a width x height frame: SamplesPerPixel rays per pixel,
// in row-major order, with tile-local sample indexes
func GenerateRays(cam Camera, width, height, yFrom, yTo int) []WeightedRay {
	rays := make([]WeightedRay, 0, (yTo-yFrom)*width*SamplesPerPixel)

	sampleIndex := 0
	for y := yFrom; y < yTo; y++ {
		for x := range width {
			for subY := range samplesPerAxis {
				for subX := range samplesPerAxis {
					// Sample at the center of each sub-cell of the pixel:
					// for a 2x2 grid that is 0.25 and 0.75 of the pixel size
					sampleX := float64(x) + (float64(subX)+0.5)/samplesPerAxis
					sampleY := float64(y) + (float64(subY)+0.5)/samplesPerAxis

					u, v := pixelToNDC(sampleX, sampleY, width, height)
					rays = append(rays, WeightedRay{Ray: cam.ray(u, v), SampleIndex: sampleIndex, Weight: 1})
					sampleIndex++
				}
			}
		}
	}
	return rays
}

// pixelToNDC maps a point given in pixel coordinates (y grows downwards) to
// normalized device coordinates: u spans [-1, 1] left to right, v spans bottom
// to top and is scaled by the aspect ratio so pixels stay square on screen
func pixelToNDC(sampleX, sampleY float64, width, height int) (u, v float64) {
	aspect := float64(height) / float64(width)
	u = 2*sampleX/float64(width) - 1
	v = (1 - 2*sampleY/float64(height)) * aspect
	return u, v
}
