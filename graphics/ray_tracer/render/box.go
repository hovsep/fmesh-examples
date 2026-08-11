package render

import "math"

// box is an axis-aligned solid, the building block of the 3D lettering
type box struct {
	min, max     Vec3
	color        Vec3
	reflectivity float64
}

// intersect finds the entry point of the ray into the box using the slab
// method: the ray enters when it is inside the [min, max] interval of all
// three axes at once. Only hits from outside are reported
func (b box) intersect(r Ray) (t float64, normal Vec3, ok bool) {
	const eps = 1e-4

	origin := [3]float64{r.Origin.X, r.Origin.Y, r.Origin.Z}
	dir := [3]float64{r.Dir.X, r.Dir.Y, r.Dir.Z}
	bmin := [3]float64{b.min.X, b.min.Y, b.min.Z}
	bmax := [3]float64{b.max.X, b.max.Y, b.max.Z}

	tNear, tFar := math.Inf(-1), math.Inf(1)
	nearAxis, nearSign := 0, 1.0

	for axis := range 3 {
		// A zero direction component yields ±Inf bounds, which the
		// comparisons below handle naturally
		invD := 1 / dir[axis]
		t0 := (bmin[axis] - origin[axis]) * invD
		t1 := (bmax[axis] - origin[axis]) * invD
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		if t0 > tNear {
			tNear = t0
			nearAxis = axis
			// The ray enters through the face it points against
			if dir[axis] > 0 {
				nearSign = -1
			} else {
				nearSign = 1
			}
		}
		if t1 < tFar {
			tFar = t1
		}
		if tNear > tFar {
			return 0, Vec3{}, false
		}
	}

	if tNear < eps || tFar < eps {
		return 0, Vec3{}, false
	}

	switch nearAxis {
	case 0:
		normal = Vec3{nearSign, 0, 0}
	case 1:
		normal = Vec3{0, nearSign, 0}
	default:
		normal = Vec3{0, 0, nearSign}
	}
	return tNear, normal, true
}

// hitsVolume reports whether the ray passes through the box volume and how
// far away it enters it. Unlike intersect it also accepts rays starting
// inside the box — required for a bounding volume, since shadow rays may
// originate right under the lettering
func (b box) hitsVolume(r Ray) (entry float64, ok bool) {
	const eps = 1e-4

	origin := [3]float64{r.Origin.X, r.Origin.Y, r.Origin.Z}
	dir := [3]float64{r.Dir.X, r.Dir.Y, r.Dir.Z}
	bmin := [3]float64{b.min.X, b.min.Y, b.min.Z}
	bmax := [3]float64{b.max.X, b.max.Y, b.max.Z}

	tNear, tFar := math.Inf(-1), math.Inf(1)
	for axis := range 3 {
		invD := 1 / dir[axis]
		t0 := (bmin[axis] - origin[axis]) * invD
		t1 := (bmax[axis] - origin[axis]) * invD
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		tNear = math.Max(tNear, t0)
		tFar = math.Min(tFar, t1)
		if tNear > tFar {
			return 0, false
		}
	}
	if tFar < eps {
		return 0, false
	}
	return math.Max(tNear, 0), true
}
