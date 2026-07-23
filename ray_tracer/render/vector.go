package render

import "math"

// Vec3 is a minimal 3D vector, also used for RGB colors
type Vec3 struct {
	X, Y, Z float64
}

func (a Vec3) add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) length() float64      { return math.Sqrt(a.dot(a)) }

func (a Vec3) norm() Vec3 {
	l := a.length()
	if l == 0 {
		return a
	}
	return a.scale(1 / l)
}

func (a Vec3) cross(b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}

// reflect mirrors the vector around normal n
func (a Vec3) reflect(n Vec3) Vec3 {
	return a.sub(n.scale(2 * a.dot(n)))
}
