// Package tracing provides the wavefront tracing components: the parallel
// raygen -> intersector -> shadow-caster -> shader chains and the accumulator
// summing the contribution waves. The reflection recursion of a classic ray
// tracer is expressed here as a mesh cycle: shaders loop the reflected wave
// back into their chain's intersector.
package tracing

import (
	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// State keys under which chain components keep their adopted config
const (
	stateGeometry = "geometry"
	stateLight    = "light"
)

// The scene source emits its config signals on the very first cycle, before
// any wave can reach the tracing components, so the adopted values below are
// always present when the first wave arrives.

// adoptGeometry stores a private deep copy of the one-off geometry config
// signal (if one arrived this cycle) in the component's state — after
// adoption the component owns its geometry outright, nothing is shared
func adoptGeometry(this *component.Component) {
	if sig := this.InputByName(common.PortGeometryIn).Signals().First(); sig != nil {
		this.State().Set(stateGeometry, sig.Payload().(render.Geometry).Clone())
	}
}

// geometryFromState returns the geometry previously adopted by the component
func geometryFromState(this *component.Component) render.Geometry {
	geometry, _ := this.State().Get(stateGeometry).(render.Geometry)
	return geometry
}

// adoptLight stores the one-off light config signal in the component's state.
// The light is a plain value, so reading it from the signal already copies it
func adoptLight(this *component.Component) {
	if sig := this.InputByName(common.PortLightIn).Signals().First(); sig != nil {
		this.State().Set(stateLight, sig.Payload())
	}
}

// lightFromState returns the light previously adopted by the component
func lightFromState(this *component.Component) render.Vec3 {
	light, _ := this.State().Get(stateLight).(render.Vec3)
	return light
}
