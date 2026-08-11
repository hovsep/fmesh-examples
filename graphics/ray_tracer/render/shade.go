package render

import "math"

// MaxBounces limits how many times a ray can bounce between reflective surfaces
const MaxBounces = 3

// surfaceEps offsets secondary ray origins off the surface to avoid self-hits
const surfaceEps = 1e-3

// Contribution is one weighted color addition to a sample of the tile
type Contribution struct {
	SampleIndex int
	Color       Vec3
}

// CastShadows fires a shadow ray from every hit towards the light
// and marks the hits which actually see it
func (g Geometry) CastShadows(light Vec3, hits []HitResult) []HitResult {
	out := make([]HitResult, len(hits))
	for i, hr := range hits {
		if hr.found {
			toLight := light.sub(hr.hit.point)
			origin := hr.hit.point.add(hr.hit.normal.scale(surfaceEps))
			hr.lit = !g.occluded(origin, toLight.norm(), toLight.length())
		}
		out[i] = hr
	}
	return out
}

// Shade converts a wave of hits into color contributions and, for reflective
// surfaces, spawns the next wave of rays. This is the recursion of a classic
// ray tracer unrolled into batches (wavefront tracing): the caller loops the
// secondary wave back through intersection and shading until MaxBounces
func Shade(light Vec3, hits []HitResult, bounce int) (contribs []Contribution, secondaries []WeightedRay) {
	contribs = make([]Contribution, 0, len(hits))

	for _, hr := range hits {
		wr := hr.ray

		if !hr.found {
			contribs = append(contribs, Contribution{wr.SampleIndex, sky(wr.Ray).scale(wr.Weight)})
			continue
		}

		local := localColor(light, hr)
		refl := hr.hit.reflectivity

		if refl > 0 && bounce < MaxBounces {
			// Split the ray's weight: (1-refl) is settled now with the local
			// color, refl continues into the scene with the bounced ray
			contribs = append(contribs, Contribution{wr.SampleIndex, local.scale(wr.Weight * (1 - refl))})
			secondaries = append(secondaries, WeightedRay{
				Ray: Ray{
					Origin: hr.hit.point.add(hr.hit.normal.scale(surfaceEps)),
					Dir:    wr.Dir.reflect(hr.hit.normal),
				},
				SampleIndex: wr.SampleIndex,
				Weight:      wr.Weight * refl,
			})
		} else {
			contribs = append(contribs, Contribution{wr.SampleIndex, local.scale(wr.Weight)})
		}
	}
	return contribs, secondaries
}

// Accumulate adds a wave's contributions into the per-sample color buffer.
// The buffer is allocated on the first (primary) wave, which touches every sample
func Accumulate(buf []Vec3, contribs []Contribution) []Vec3 {
	if buf == nil {
		buf = make([]Vec3, len(contribs))
	}
	for _, c := range contribs {
		buf[c.SampleIndex] = buf[c.SampleIndex].add(c.Color)
	}
	return buf
}

// localColor computes the directly visible color of a hit:
// ambient + diffuse (if the point sees the light) + specular highlight
func localColor(light Vec3, hr HitResult) Vec3 {
	lightDir := light.sub(hr.hit.point).norm()

	// Ambient light so shadows are not pitch black
	intensity := 0.15
	if hr.lit {
		intensity += 0.85 * math.Max(0, hr.hit.normal.dot(lightDir))
	}
	color := hr.hit.color.scale(intensity)

	// Specular highlight
	specDir := hr.ray.Dir.reflect(hr.hit.normal)
	spec := math.Pow(math.Max(0, specDir.dot(lightDir)), 32)
	return color.add(Vec3{1, 1, 1}.scale(0.4 * spec))
}

// sky is the vertical gradient seen by rays which hit nothing
func sky(r Ray) Vec3 {
	t := 0.5 * (r.Dir.Y + 1)
	return Vec3{1, 1, 1}.scale(1 - t).add(Vec3{0.4, 0.6, 0.95}.scale(t))
}
