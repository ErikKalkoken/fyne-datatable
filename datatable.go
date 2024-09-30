package datatable

import (
	"slices"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type DataTable struct {
	// Whether the footer is shown
	FooterEnabled bool

	widget.BaseWidget
	bottomLabel *widget.Label
	header      []fyne.CanvasObject
	headerCells []string
	list        *widget.List
	numCols     int

	mu            sync.RWMutex
	layout        columnsLayout
	cells         [][]string
	cellsFiltered [][]string
}

func NewDataTable(headerCells []string) *DataTable {
	w := &DataTable{
		bottomLabel: widget.NewLabel(""),
		headerCells: headerCells,
		numCols:     len(headerCells),
	}
	w.ExtendBaseWidget(w)
	w.list = w.makeList()
	w.header = w.makeHeader()
	return w
}

func (w *DataTable) makeList() *widget.List {
	list := widget.NewList(
		func() int {
			w.mu.RLock()
			defer w.mu.RUnlock()
			return len(w.cellsFiltered)
		},
		func() fyne.CanvasObject {
			w.mu.RLock()
			defer w.mu.RUnlock()
			objects := make([]fyne.CanvasObject, w.numCols)
			for i := range w.numCols {
				objects[i] = widget.NewLabel("")
			}
			return container.New(w.layout, objects...)
		},
		func(id widget.ListItemID, co fyne.CanvasObject) {
			w.mu.RLock()
			defer w.mu.RUnlock()
			if id >= len(w.cellsFiltered) {
				return // safeguard
			}
			r := w.cellsFiltered[id]
			c := co.(*fyne.Container)
			for i := range w.numCols {
				o := c.Objects[i].(*widget.Label)
				o.SetText(r[i])
			}
		},
	)
	return list
}

func (w *DataTable) makeHeader() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, w.numCols)
	for i, s := range w.headerCells {
		o := widget.NewEntry()
		o.PlaceHolder = s
		o.OnChanged = func(s string) {
			filter := make([]string, w.numCols)
			for j, x := range objects {
				y := x.(*widget.Entry)
				filter[j] = y.Text
			}
			filter[i] = s
			w.filterRows(filter)
			w.list.Refresh()
		}
		objects[i] = o
	}
	return objects
}

func (w *DataTable) filterRows(filter []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var selection [][]string
	for _, row := range w.cells {
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
	w.cellsFiltered = selection
	w.updateFooter()
}

func (w *DataTable) SetCells(cells [][]string) {
	defer w.list.Refresh()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cells = slices.Clone(cells)
	w.cellsFiltered = slices.Clone(cells)
	w.layout = columnsLayout(maxColWidths(slices.Concat([][]string{w.headerCells}, cells)))
	w.updateFooter()
}

func (w *DataTable) updateFooter() {
	var s string
	p := message.NewPrinter(language.English)
	if len(w.cellsFiltered) < len(w.cells) {
		s = p.Sprintf("%d of %d entries (filtered)", len(w.cellsFiltered), len(w.cells))
	} else {
		s = p.Sprintf("%d entries", len(w.cells))
	}
	w.bottomLabel.SetText(s)
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

func (w *DataTable) CreateRenderer() fyne.WidgetRenderer {
	w.mu.RLock()
	defer w.mu.RUnlock()
	header := container.NewVBox(container.New(w.layout, w.header...), widget.NewSeparator())
	var footer fyne.CanvasObject
	if w.FooterEnabled {
		footer = container.NewVBox(widget.NewSeparator(), w.bottomLabel)
	}
	c := container.NewBorder(header, footer, nil, nil, w.list)
	return widget.NewSimpleRenderer(c)
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
