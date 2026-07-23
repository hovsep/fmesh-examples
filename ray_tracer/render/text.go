package render

import "math"

// stroke is one filled rect of a glyph, in cell coordinates
// (x grows right, y grows up, letters are 5 cells tall)
type stroke struct {
	x0, y0, x1, y1 float64
}

// glyph is a blocky letter shape assembled from strokes
type glyph struct {
	width   float64
	strokes []stroke
}

// textCluster is 3D lettering: extruded letter boxes plus their common
// bounding box, so rays passing nowhere near the text pay a single test
type textCluster struct {
	bound box // bounding volume only, carries no material
	boxes []box
}

// fmeshLettering builds the extruded blocky "F-MESH" 3D lettering, centered
// on x, raised lift units above the floor, with its front face at the given z
func fmeshLettering(cell, depth, lift, z float64, color Vec3, reflectivity float64) textCluster {
	letters := []glyph{
		// F
		{width: 4, strokes: []stroke{{0, 0, 1, 5}, {1, 4, 4, 5}, {1, 2, 3, 3}}},
		// -
		{width: 3, strokes: []stroke{{0, 2, 3, 3}}},
		// M
		{width: 5, strokes: []stroke{{0, 0, 1, 5}, {4, 0, 5, 5}, {1, 4, 4, 5}, {2, 2.5, 3, 4}}},
		// E
		{width: 4, strokes: []stroke{{0, 0, 1, 5}, {1, 4, 4, 5}, {1, 2, 3, 3}, {1, 0, 4, 1}}},
		// S
		{width: 4, strokes: []stroke{{0, 4, 4, 5}, {0, 3, 1, 4}, {0, 2, 4, 3}, {3, 1, 4, 2}, {0, 0, 4, 1}}},
		// H
		{width: 4, strokes: []stroke{{0, 0, 1, 5}, {3, 0, 4, 5}, {1, 2, 3, 3}}},
	}

	// Total width in cells: glyph widths plus one cell between letters
	totalCells := float64(len(letters) - 1)
	for _, g := range letters {
		totalCells += g.width
	}

	var boxes []box
	x := -totalCells * cell / 2
	for _, g := range letters {
		for _, s := range g.strokes {
			boxes = append(boxes, box{
				min:          Vec3{x + s.x0*cell, lift + s.y0*cell, z},
				max:          Vec3{x + s.x1*cell, lift + s.y1*cell, z + depth},
				color:        color,
				reflectivity: reflectivity,
			})
		}
		x += (g.width + 1) * cell
	}

	return textCluster{bound: boundOf(boxes), boxes: boxes}
}

// boundOf returns the common bounding box of the given boxes
func boundOf(boxes []box) box {
	bound := box{
		min: Vec3{math.Inf(1), math.Inf(1), math.Inf(1)},
		max: Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)},
	}
	for _, b := range boxes {
		bound.min = Vec3{
			math.Min(bound.min.X, b.min.X),
			math.Min(bound.min.Y, b.min.Y),
			math.Min(bound.min.Z, b.min.Z),
		}
		bound.max = Vec3{
			math.Max(bound.max.X, b.max.X),
			math.Max(bound.max.Y, b.max.Y),
			math.Max(bound.max.Z, b.max.Z),
		}
	}
	return bound
}
