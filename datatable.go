package datatable

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type sortDir uint

const (
	sortOff sortDir = iota
	sortAsc
	sortDesc
)

// row represents a row in a DataTable
type row struct {
	idx     int // index of this row in the original data
	columns []string
}

// DataTable is a Fyne widget representing a table for showing data.
type DataTable struct {
	// Whether the footer is shown
	FooterDisabled bool
	// Whether the header is shown
	HeaderDisabled bool
	// Callback runs when an entry is selected.
	// The index refers to the row in the original data.
	OnSelected func(index int)
	// Whether the search bar is shown
	SearchBarDisabled bool

	widget.BaseWidget
	body        *widget.List
	footer      *widget.Label
	header      []fyne.CanvasObject
	headerCells []string
	numCols     int
	searchBar   *widget.Entry
	sortCols    []sortDir

	mu            sync.RWMutex
	layout        columnsLayout
	cells         []row
	cellsFiltered []row
	widths        []float32
}

// NewDataTable returns a new DataTable with automatic width detection.
func NewDataTable(headers []string) *DataTable {
	w := makeWidget(headers)
	return w
}

// NewDataTable returns a new DataTable with fixed columns widths.
func NewDataTableWithFixedColumns(headers []string, widths []float32) (*DataTable, error) {
	w := makeWidget(headers)
	if len(widths) != len(headers) {
		return nil, fmt.Errorf("need to provide widths for exactly %d columns", w.numCols)
	}
	w.widths = widths
	w.layout = columnsLayout(w.widths)
	return w, nil
}

func makeWidget(headerCells []string) *DataTable {
	w := &DataTable{
		footer:      widget.NewLabel(""),
		headerCells: headerCells,
		numCols:     len(headerCells),
		sortCols:    make([]sortDir, len(headerCells)),
	}
	w.ExtendBaseWidget(w)
	w.body = w.makeBody()
	w.header = w.makeHeader()
	w.searchBar = w.makeSearchBar()
	w.sortCols[0] = sortAsc
	return w
}

func (w *DataTable) makeSearchBar() *widget.Entry {
	e := widget.NewEntry()
	e.ActionItem = widget.NewIcon(theme.SearchIcon())
	e.OnChanged = func(s string) {
		w.applyFilterAndSort(s)
	}
	return e
}

func (w *DataTable) applyFilterAndSort(search string) {
	func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.applySort()
		var selection []row
		s2 := strings.ToLower(search)
		for _, row := range w.cells {
			match := false
			for _, c := range row.columns {
				c2 := strings.ToLower(c)
				if strings.Contains(c2, s2) {
					match = true
					break
				}
			}
			if match {
				selection = append(selection, row)
			}
		}
		w.cellsFiltered = selection
	}()
	w.updateFooter()
	w.body.Refresh()
}

func (w *DataTable) applySort() {
	for i, x := range w.header {
		t := w.headerCells[i]
		var t2 string
		switch w.sortCols[i] {
		case sortOff:
			t2 = t
		case sortAsc:
			t2 = t + "↑"
		case sortDesc:
			t2 = t + "↓"
		}
		l := x.(*TappableLabel)
		l.SetText(t2)
	}
	for i, c := range w.sortCols {
		switch c {
		case sortAsc:
			slices.SortFunc(w.cells, func(a, b row) int {
				return cmp.Compare(a.columns[i], b.columns[i])
			})
		case sortDesc:
			slices.SortFunc(w.cells, func(a, b row) int {
				return cmp.Compare(b.columns[i], a.columns[i])
			})
		}
	}
}

func (w *DataTable) makeHeader() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, w.numCols)
	for i, s := range w.headerCells {
		o := NewTappableLabel(s, nil)
		o.TextStyle.Bold = true
		o.OnTapped = func() {
			for j := range w.numCols {
				if j == i {
					if w.sortCols[i] == sortDesc {
						w.sortCols[i] = sortAsc
					} else {
						w.sortCols[i]++
					}
				} else {
					w.sortCols[j] = sortOff
				}
			}
			w.applyFilterAndSort(w.searchBar.Text)
		}
		objects[i] = o
	}
	return objects
}

func (w *DataTable) makeBody() *widget.List {
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
				o.SetText(r.columns[i])
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if w.OnSelected == nil {
			list.UnselectAll()
			return
		}
		w.mu.RLock()
		defer w.mu.RUnlock()
		if id >= len(w.cellsFiltered) {
			return // safeguard
		}
		w.OnSelected(w.cellsFiltered[id].idx)
	}
	return list
}

// SetCells sets the content of all cells in the table.
// Returns an error if not all rows have the same number of columns as the header.
func (w *DataTable) SetCells(cells [][]string) error {
	defer w.body.Refresh()
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range cells {
		if len(r) != w.numCols {
			return fmt.Errorf("some rows do not have %d columns", w.numCols)
		}
	}
	w.cells = make([]row, len(cells))
	for i, r := range cells {
		w.cells[i] = row{idx: i, columns: r}
	}
	w.cellsFiltered = slices.Clone(w.cells)
	if len(w.widths) == 0 {
		w.layout = columnsLayout(maxColWidths(slices.Concat([][]string{w.headerCells}, cells)))
	}
	w.applySort()
	w.updateFooter()
	return nil
}

func (w *DataTable) updateFooter() {
	var s string
	p := message.NewPrinter(language.English)
	if len(w.cellsFiltered) < len(w.cells) {
		s = p.Sprintf("%d of %d entries (filtered)", len(w.cellsFiltered), len(w.cells))
	} else {
		s = p.Sprintf("%d entries", len(w.cells))
	}
	w.footer.SetText(s)
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
	var headerFrame, footerFrame fyne.CanvasObject
	header := container.NewStack(
		canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground)),
		container.New(w.layout, w.header...),
	)
	if !w.SearchBarDisabled && !w.HeaderDisabled {
		headerFrame = container.NewVBox(w.searchBar, header, widget.NewSeparator())
	} else if w.HeaderDisabled {
		headerFrame = container.NewVBox(w.searchBar, widget.NewSeparator())
	} else if w.SearchBarDisabled {
		headerFrame = container.NewVBox(header, widget.NewSeparator())
	}
	if !w.FooterDisabled {
		footerFrame = container.NewVBox(widget.NewSeparator(), w.footer)
	}
	c := container.NewBorder(headerFrame, footerFrame, nil, nil, w.body)
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
			w = max(containerSize.Width-pos.X-padding, d[i])
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
