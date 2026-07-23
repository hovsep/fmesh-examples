// Package common holds the configuration constants shared by the packages
// of the wavefront ray tracer example: frame parameters, port names and
// the names of the scalars (numeric signal metadata) used for routing.
package common

const (
	FrameWidth  = 320
	FrameHeight = 240
	TotalFrames = 36
	NumChains   = 4 // parallel raygen -> intersector -> shadow-caster -> shader chains
	GifDelay    = 5 // per-frame delay in 1/100s of a second
	OutputFile  = "out.gif"

	PortNextFrame    = "i_next_frame"
	PortGeometryIn   = "i_geometry"
	PortGeometryOut  = "o_geometry"
	PortLightIn      = "i_light"
	PortLightOut     = "o_light"
	PortStartOut     = "o_start"
	PortTileOut      = "o_tile"
	PortContribsOut  = "o_contribs"
	PortSecondaryOut = "o_secondary"
	PortFrameOut     = "o_frame"
	PortNextOut      = "o_next"
	PortProgressOut  = "o_progress"
	PortIn           = "in"
	PortOut          = "out"

	// Scalars: numeric metadata riding on signals (meta.Scalars)
	ScalarFrame  = "frame"
	ScalarYFrom  = "y_from"
	ScalarYTo    = "y_to"
	ScalarBounce = "bounce"
)
