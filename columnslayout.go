package datatable

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type columnsLayout []float32

func (cl columnsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	if len(objects) > 0 {
		h = objects[0].MinSize().Height
	}
	for i, x := range cl {
		w += x
		if i < len(cl) {
			w += theme.Padding()
		}
	}
	s := fyne.NewSize(w, h)
	return s
}

func (cl columnsLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	if len(cl) < len(objects) {
		panic(fmt.Sprintf("not enough columns defined. Need: %d, Have: %d", len(objects), len(cl))) // FIXME
	}
	var lastX float32
	pos := fyne.NewPos(0, 0)
	padding := theme.Padding()
	for i, o := range objects {
		size := o.MinSize()
		var w float32
		if i < len(objects)-1 || containerSize.Width < 0 {
			w = cl[i]
		} else {
			w = max(containerSize.Width-pos.X-padding, cl[i])
		}
		o.Resize(fyne.Size{Width: w, Height: size.Height})
		o.Move(pos)
		var x float32
		if len(cl) > i {
			x = w
			lastX = x
		} else {
			x = lastX
		}
		pos = pos.AddXY(x+padding, 0)
	}
}
