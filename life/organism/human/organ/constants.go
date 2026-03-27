package organ

import . "github.com/hovsep/fmesh-examples/life/unit"

const (
	// Organ damage
	criticalDamageLevel = 1.0 * DNCS
	defaultDamageLevel  = 0.01 * DNCS
	damageRampRate      = 3.5e-12 * DNCS //  ~90 years
)
