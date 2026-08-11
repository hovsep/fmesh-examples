package render

import "math"

// sphere is the only solid primitive in the scene
type sphere struct {
	center       Vec3
	radius       float64
	color        Vec3
	reflectivity float64
}

// Geometry is the solid content of the scene:
// spheres, the ground plane and the 3D lettering
type Geometry struct {
	spheres []sphere
	text    textCluster
}

// Scene bundles everything needed to render. It is a plain value: the mesh
// derives per-concern config signals from it (geometry, light) instead of
// sharing a pointer between components
type Scene struct {
	Geometry Geometry
	Light    Vec3 // point light position
}

// hit describes the closest intersection of a ray with the scene
type hit struct {
	t            float64
	point        Vec3
	normal       Vec3
	color        Vec3
	reflectivity float64
}

// HitResult pairs a traced ray with what it hit (if anything).
// The fields are internal to the render package: for the mesh this is
// an opaque value flowing between the tracing stages
type HitResult struct {
	ray   WeightedRay
	hit   hit
	found bool
	lit   bool // set by CastShadows
}

// NewScene returns the demo scene: a few spheres and the blocky
// "F-MESH" lettering on a checkerboard floor
func NewScene() Scene {
	return Scene{
		Geometry: Geometry{
			spheres: []sphere{
				// Big mirror sphere in the center
				{center: Vec3{0, 1, 0}, radius: 1, color: Vec3{0.95, 0.95, 0.95}, reflectivity: 0.95},
				// Matte red
				{center: Vec3{-1.9, 0.6, 1.2}, radius: 0.6, color: Vec3{0.95, 0.25, 0.2}},
				// Glossy blue
				{center: Vec3{1.8, 0.5, 0.8}, radius: 0.5, color: Vec3{0.2, 0.4, 0.95}, reflectivity: 0.3},
				// Small green
				{center: Vec3{0.7, 0.3, 2.1}, radius: 0.3, color: Vec3{0.25, 0.85, 0.35}},
			},
			// Golden 3D lettering floating above the spheres
			text: fmeshLettering(0.26, 0.28, 1.9, -2.6, Vec3{0.9, 0.7, 0.25}, 0.25),
		},
		Light: Vec3{5, 8, 5},
	}
}

// Clone returns a deep copy of the geometry, so every consumer can own
// a fully independent instance — no memory shared between components
func (g Geometry) Clone() Geometry {
	spheres := make([]sphere, len(g.spheres))
	copy(spheres, g.spheres)
	boxes := make([]box, len(g.text.boxes))
	copy(boxes, g.text.boxes)
	return Geometry{spheres: spheres, text: textCluster{bound: g.text.bound, boxes: boxes}}
}

// IntersectAll finds the closest intersection for a whole wave of rays
func (g Geometry) IntersectAll(rays []WeightedRay) []HitResult {
	results := make([]HitResult, len(rays))
	for i, wr := range rays {
		h, found := g.intersect(wr.Ray)
		results[i] = HitResult{ray: wr, hit: h, found: found}
	}
	return results
}

// intersect finds the closest hit of the ray with the geometry (spheres + ground plane)
func (g Geometry) intersect(r Ray) (hit, bool) {
	const eps = 1e-4
	closest := hit{t: math.Inf(1)}
	found := false

	for _, sp := range g.spheres {
		oc := r.Origin.sub(sp.center)
		b := oc.dot(r.Dir)
		c := oc.dot(oc) - sp.radius*sp.radius
		disc := b*b - c
		if disc < 0 {
			continue
		}
		t := -b - math.Sqrt(disc)
		if t < eps || t >= closest.t {
			continue
		}
		p := r.Origin.add(r.Dir.scale(t))
		closest = hit{
			t:            t,
			point:        p,
			normal:       p.sub(sp.center).norm(),
			color:        sp.color,
			reflectivity: sp.reflectivity,
		}
		found = true
	}

	// The lettering: rays are tested against the individual letter boxes
	// only when they pass through the text's bounding volume
	if entry, ok := g.text.bound.hitsVolume(r); ok && entry < closest.t {
		for _, bx := range g.text.boxes {
			t, normal, ok := bx.intersect(r)
			if !ok || t >= closest.t {
				continue
			}
			closest = hit{
				t:            t,
				point:        r.Origin.add(r.Dir.scale(t)),
				normal:       normal,
				color:        bx.color,
				reflectivity: bx.reflectivity,
			}
			found = true
		}
	}

	// Ground plane y=0 with a checkerboard pattern
	if r.Dir.Y < 0 {
		t := -r.Origin.Y / r.Dir.Y
		if t > eps && t < closest.t {
			p := r.Origin.add(r.Dir.scale(t))
			color := Vec3{0.9, 0.9, 0.9}
			if (int(math.Floor(p.X))+int(math.Floor(p.Z)))%2 != 0 {
				color = Vec3{0.25, 0.3, 0.35}
			}
			closest = hit{
				t:            t,
				point:        p,
				normal:       Vec3{0, 1, 0},
				color:        color,
				reflectivity: 0.1,
			}
			found = true
		}
	}

	return closest, found
}

// occluded reports whether anything blocks the segment from p towards the light
func (g Geometry) occluded(p, lightDir Vec3, maxDist float64) bool {
	h, ok := g.intersect(Ray{Origin: p, Dir: lightDir})
	return ok && h.t < maxDist
}
