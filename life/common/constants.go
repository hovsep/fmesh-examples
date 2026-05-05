package common

const (
	Type            Label = "type"
	Level           Label = "level"
	Bias            Label = "bias"
	Axis            Label = "axis"
	Region          Label = "region"
	TickCount       Label = "tick_count"
	SimDuration     Label = "sim_duration"
	SimWallTime     Label = "sim_wall_time"
	DeltaT          Label = "dt"
	TickMeta        Label = "tick_meta"
	Sympathetic     Label = "sympathetic"
	Parasympathetic Label = "parasympathetic"
	Noise           Label = "noise"
	Gain            Label = "gain"

	DamageLevel State = "damage_level"
	Rate        State = "rate"
	Phase       State = "phase"

	Cardiac     System = "cardiac"
	Vascular    System = "vascular"
	Respiratory System = "respiratory"
	GI          System = "gi"

	Balanced Trend = "balanced"
	Rising   Trend = "rising"
	Falling  Trend = "falling"

	Left  Side = "left"
	Right Side = "right"
)
