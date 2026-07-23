package render

import "math"

// Viewpoint is where the camera stands and what it looks at
type Viewpoint struct {
	Position, LookAt Vec3
}

// Camera is an orthonormal basis placed in the scene
type Camera struct {
	origin, forward, right, up Vec3
}

// OrbitPosition returns the viewpoint for the given animation frame:
// the camera circles around the scene center once per full animation
func OrbitPosition(frame, totalFrames int) Viewpoint {
	angle := 2 * math.Pi * float64(frame) / float64(totalFrames)
	return Viewpoint{
		Position: Vec3{5.5 * math.Sin(angle), 2.2, 5.5 * math.Cos(angle)},
		LookAt:   Vec3{0, 0.9, 0},
	}
}

// NewCamera builds a camera looking from pos to lookAt
func NewCamera(pos, lookAt Vec3) Camera {
	forward := lookAt.sub(pos).norm()
	right := forward.cross(Vec3{0, 1, 0}).norm()
	up := right.cross(forward)
	return Camera{origin: pos, forward: forward, right: right, up: up}
}

// ray builds a primary ray through the image plane point (u, v), both in [-1, 1]
func (c Camera) ray(u, v float64) Ray {
	const fov = 1.1 // half-width of the image plane at distance 1
	dir := c.forward.
		add(c.right.scale(u * fov)).
		add(c.up.scale(v * fov)).
		norm()
	return Ray{Origin: c.origin, Dir: dir}
}
