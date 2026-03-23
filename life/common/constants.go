package common

const (
	Type            Label = "type"
	Level           Label = "level"
	Bias            Label = "bias"
	Axis            Label = "axis"
	Region          Label = "region"
	TickCount       Label = "tick_count"
	SimTime         Label = "sim_time"
	SimWallTime     Label = "sim_wall_time"
	DeltaT          Label = "dt"
	TickMeta        Label = "tick_meta"
	Sympathetic     Label = "sympathetic"
	Parasympathetic Label = "parasympathetic"
	Noise           Label = "noise"
	Gain            Label = "gain"

	DamageLevel State = "damage_level"

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
