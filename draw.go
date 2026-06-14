package nirilayout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const drawingSize = 200

type placedOutput struct {
	name           string
	style          outputStyle
	xp, yp, wp, hp int
	output         *Output
}

// Niri repositions outputs from scratch every time the output configuration
// changes (which includes monitors disconnecting and connecting). The following
// algorithm is used for positioning outputs.
//   - Collect all connected monitors and their logical sizes.
//   - Sort them by their name. This makes it so the automatic positioning does
//     not depend on the order the monitors are connected. This is important
//     because the connection order is non-deterministic at compositor startup.
//   - Try to place every output with explicitly configured position, in order.
//     If the output overlaps previously placed outputs, place it to the right
//     of all previously placed outputs. In this case, niri will also print a
//     warning.
//   - Place every output without explicitly configured position by putting it
//     to the right of all previously placed outputs.
func placeOutputs(outputs []*Output) []placedOutput {
	slices.SortFunc(outputs, func(a, b *Output) int {
		return strings.Compare(a.Name, b.Name)
	})

	var placed []placedOutput

	autoX := 0

	// place outputs with explicitly configured position
	for _, output := range outputs {
		if output.Position == nil {
			continue
		}

		x, y, w, h := output.LogicalRect()

		overlap := false
		for _, placedOutput := range placed {
			xp, yp, wp, hp := placedOutput.xp, placedOutput.yp, placedOutput.wp, placedOutput.hp
			if x+w > xp && x < xp+wp && y+h > yp && y < yp+hp {
				overlap = true
				break
			}
		}
		if overlap {
			x = autoX
			y = 0
		}
		name := output.Name
		if output.NameOverride != "" {
			name = output.NameOverride
		}
		placed = append(placed, placedOutput{
			name:   name,
			style:  output.resolvedStyle,
			xp:     x,
			yp:     y,
			wp:     w,
			hp:     h,
			output: output,
		})
		autoX = max(autoX, x+w)
	}

	// place outputs without explicitly configured position
	for _, output := range outputs {
		if output.Position != nil {
			continue
		}

		_, _, w, h := output.LogicalRect()
		x := autoX
		y := 0
		name := output.Name
		if output.NameOverride != "" {
			name = output.NameOverride
		}
		placed = append(placed, placedOutput{
			name:   name,
			style:  output.resolvedStyle,
			xp:     x,
			yp:     y,
			wp:     w,
			hp:     h,
			output: output,
		})
		autoX = x + w
	}

	return placed
}

func drawLayout(layout Layout) *gtk.DrawingArea {
	outputs := placeOutputs(layout.Outputs)

	da := gtk.NewDrawingArea()
	layoutWidth := float64(drawingSize)
	layoutHeight := float64(drawingSize)
	for _, o := range outputs {
		layoutWidth = max(layoutWidth, float64(o.xp)+float64(o.wp))
		layoutHeight = max(layoutHeight, float64(o.yp)+float64(o.hp))
	}
	scale := min(drawingSize/layoutWidth, drawingSize/layoutHeight)
	da.SetSizeRequest(int(layoutWidth*scale), int(layoutHeight*scale))

	da.SetDrawFunc(func(drawingArea *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		cr.SetSourceRGBA(0, 0, 0, 0)
		cr.Paint()
		cr.MoveTo(0, 0)

		if len(outputs) == 0 {
			msg := T("No preview available")
			extents := cr.TextExtents(msg)
			cr.MoveTo(float64(width)/2-extents.Width/2-extents.XBearing, float64(height)/2-extents.Height/2-extents.YBearing)
			cr.SetSourceRGBA(rgba(gray400))
			cr.ShowText(msg)
			return
		}

		for _, o := range outputs {
			x, y, w, h := float64(o.xp)*scale, float64(o.yp)*scale,
				float64(o.wp)*scale, float64(o.hp)*scale
			borderWidth := float64(o.style.borderWidth)

			cr.Rectangle(x, y, w, h)
			cr.SetSourceRGBA(rgba(o.style.fillColor))
			cr.FillPreserve()
			cr.ClipPreserve()
			cr.SetLineWidth(borderWidth * 2)
			cr.SetSourceRGBA(rgba(o.style.borderColor))
			cr.Stroke()
			cr.ResetClip()

			cr.SelectFontFace(o.style.fontFamily, o.style.fontStyle, o.style.fontWeight)
			cr.SetFontSize(o.style.fontSize)
			cr.SetSourceRGBA(rgba(o.style.textColor))

			lines := []string{o.name}
			if !o.style.hideDetails {
				rw, rh := o.output.Resolution()
				scale := 1.0
				if o.output.Scale != 0 {
					scale = o.output.Scale
				}
				lines = append(lines,
					fmt.Sprintf("%dx%d", rw, rh),
					fmt.Sprintf("%.2g×", scale),
				)
			}
			extents := make([]cairo.TextExtents, len(lines))
			totalHeight := 0.0
			for i, line := range lines {
				ex := cr.TextExtents(line)
				extents[i] = ex
				totalHeight += ex.Height
				if i != len(lines)-1 {
					totalHeight += float64(o.style.lineSpacing)
				}
			}

			py := y + h/2 - totalHeight/2
			for i, line := range lines {
				ex := extents[i]
				cr.MoveTo(x+w/2-ex.Width/2-ex.XBearing, py-ex.YBearing)
				cr.ShowText(line)
				py += ex.Height
				py += float64(o.style.lineSpacing)
				if i == 0 {
					cr.SetSourceRGBA(rgba(applyBrightness(o.style.textColor, 0.8)))
				}
			}
		}
	})

	return da
}
