package views

// A full-width panel is drawn as styles.PanelStyle.Width(width-4): the rounded
// border sits outside that width and the panel's own padding inside it, so a
// row has six cells fewer than the view was given.
//
// Rows sized to anything else either wrap -- which is how the Body view's status
// word ended up on a line of its own -- or stop short, which puts their bars out
// of step with the panel next door.
func panelWidth(width int) int { return width - 4 }

// rowWidth is what a row may occupy inside a panel of the given panel width.
func rowWidth(panel int) int { return panel - 2 }
