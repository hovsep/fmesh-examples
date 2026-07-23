package render

import "math"

// AveragePixels folds the per-pixel color samples back into one color per pixel:
// GenerateRays produces SamplesPerPixel rays per pixel, averaging the traced
// results smooths jagged edges (supersampling)
func AveragePixels(samples []Vec3) []Vec3 {
	pixels := make([]Vec3, 0, len(samples)/SamplesPerPixel)
	for i := 0; i < len(samples); i += SamplesPerPixel {
		var sum Vec3
		for _, sample := range samples[i : i+SamplesPerPixel] {
			sum = sum.add(sample)
		}
		pixels = append(pixels, sum.scale(1.0/SamplesPerPixel))
	}
	return pixels
}

// GammaCorrect clamps linear colors and applies gamma correction
func GammaCorrect(pixels []Vec3) []Vec3 {
	out := make([]Vec3, len(pixels))
	for i, p := range pixels {
		out[i] = Vec3{gamma(p.X), gamma(p.Y), gamma(p.Z)}
	}
	return out
}

func gamma(v float64) float64 {
	return math.Sqrt(math.Max(0, math.Min(1, v)))
}

// PackRGBA converts colors to tightly packed RGBA bytes (4 bytes per pixel)
func PackRGBA(pixels []Vec3) []byte {
	rgba := make([]byte, 0, len(pixels)*4)
	for _, p := range pixels {
		rgba = append(rgba, byte(p.X*255), byte(p.Y*255), byte(p.Z*255), 255)
	}
	return rgba
}
