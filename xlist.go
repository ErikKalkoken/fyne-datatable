package datatable

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func MakeTable() fyne.CanvasObject {
	var cells [][]string
	header := []string{"First", "Second", "Third"}
	cellsMaster := [][]string{
		{"Joker", "Peter Parker", "Superman"},
		{"Bruce Wayne", "Penguin", "Dr. Doom"},
		{"alpha", "bravo", "charlie"},
		{"Joker", "Peter Parker", "Superman"},
		{"Bruce Wayne", "Penguin", "Dr. Doom"},
		{"alpha", "bravo", "charlie"},
	}
	cells = slices.Clone(cellsMaster)
	colLayout := columnsLayout(maxColWidths(cells))
	list := widget.NewList(
		func() int {
			return len(cells)
		},
		func() fyne.CanvasObject {
			c := len(header)
			objects := make([]fyne.CanvasObject, c)
			for i := range c {
				objects[i] = widget.NewLabel("")
			}
			return container.New(colLayout, objects...)
		},
		func(id widget.ListItemID, co fyne.CanvasObject) {
			if id >= len(cells) {
				return
			}
			r := cells[id]
			c := co.(*fyne.Container)
			for i := range len(header) {
				o := c.Objects[i].(*widget.Label)
				o.SetText(r[i])
			}
		},
	)
	filterRows := func(filter []string) {
		var selection [][]string
		for _, row := range cellsMaster {
			match := true
			for i, c := range row {
				c2 := strings.ToLower(c)
				if filter[i] != "" && !strings.Contains(c2, strings.ToLower(filter[i])) {
					match = false
					break
				}
			}
			if match {
				selection = append(selection, row)
			}
		}
		cells = selection
		list.Refresh()
	}
	objects := make([]fyne.CanvasObject, len(header))
	for i, s := range header {
		o := widget.NewEntry()
		o.PlaceHolder = s
		o.OnChanged = func(s string) {
			filter := make([]string, len(header))
			for j, x := range objects {
				y := x.(*widget.Entry)
				filter[j] = y.Text
			}
			filter[i] = s
			filterRows(filter)
		}
		objects[i] = o
	}
	head := container.NewVBox(container.New(colLayout, objects...), widget.NewSeparator())
	return container.NewBorder(head, nil, nil, nil, list)
}

func maxColWidths(cells [][]string) []float32 {
	numRows := len(cells)
	numCols := len(cells[0])
	colWidths := make([]float32, numCols)
	for c := range numCols {
		for r := range numRows {
			s := cells[r][c]
			l := widget.NewLabel(s)
			w := l.MinSize().Width
			colWidths[c] = max(w, colWidths[c])
		}
	}
	return colWidths
}

type columnsLayout []float32

func (d columnsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	if len(objects) > 0 {
		h = objects[0].MinSize().Height
	}
	for i, x := range d {
		w += x
		if i < len(d) {
			w += theme.Padding()
		}
	}
	s := fyne.NewSize(w, h)
	return s
}

func (d columnsLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	var lastX float32
	pos := fyne.NewPos(0, 0)
	padding := theme.Padding()
	for i, o := range objects {
		size := o.MinSize()
		var w float32
		if i < len(objects)-1 || containerSize.Width < 0 {
			w = d[i]
		} else {
			w = containerSize.Width - pos.X - padding
		}
		o.Resize(fyne.Size{Width: w, Height: size.Height})
		o.Move(pos)
		var x float32
		if len(d) > i {
			x = w
			lastX = x
		} else {
			x = lastX
		}
		pos = pos.AddXY(x+padding, 0)
	}
}
