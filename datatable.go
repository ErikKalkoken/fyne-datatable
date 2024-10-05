// Package datatable provides a data-driven table widget for the Fyne GUI toolkit.
package datatable

import (
	"cmp"
	"errors"
	"fmt"
	"math"
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

// A Config configures a DataTable widget.
// All fields except for Columns are optional.
type Config struct {
	// Columns configures the columns of the table.
	Columns []Column // MANDATORY

	// Whether to hide the footer.
	FooterHidden bool

	// Whether to hide the header.
	HeaderHidden bool

	// Whether to hide the search bar.
	SearchBarHidden bool

	// Initially sorted column
	SortedColumnIndex int

	// Initial sort direction
	SortedColumnDirection sortDir
}

// A Column configures a column.
type Column struct {
	// Title is the title displayed in the header of a column.
	Title string

	// Widths sets the width of each column.
	// A column with width 0 will be auto-sized to fit the data in that column.
	// The width will be adjusted to fit the column header.
	Width float32
}

type sortDir uint

// sort directions for columns
const (
	sortOff sortDir = iota
	SortAsc
	SortDesc
)

// characters for showing sort direction
const (
	characterSortAsc  = "↑"
	characterSortDesc = "↓"
)

// row represents a row in a DataTable
type row struct {
	idx     int // index of this row in the original data
	columns []string
}

// DataTable is a Fyne widget representing a data focused table.
// It is safe for concurrent use by multiple goroutines.
type DataTable struct {

	// Callback runs when an entry is selected.
	// index refers to the row in the original data.
	OnSelected func(index int)

	widget.BaseWidget
	body            *widget.List
	footer          *widget.Label
	footerHidden    bool
	header          []fyne.CanvasObject
	headerCells     []string
	headerHidden    bool
	numCols         int
	searchBar       *widget.Entry
	searchBarHidden bool
	widths          []float32

	mu            sync.RWMutex
	layout        columnsLayout
	cells         []row
	cellsFiltered []row
	sortCols      []sortDir
}

// New returns a new DataTable widget.
// The widgets is configured with a [Config] struct.
// Returns an error if the validation of the config value failed.
func New(config Config) (*DataTable, error) {
	if len(config.Columns) == 0 {
		return nil, fmt.Errorf("no headers defined")
	}
	numCols := len(config.Columns)
	headerCells := make([]string, numCols)
	for i, c := range config.Columns {
		headerCells[i] = c.Title
	}
	w := &DataTable{
		footer:          widget.NewLabel(""),
		footerHidden:    config.FooterHidden,
		headerCells:     headerCells,
		headerHidden:    config.HeaderHidden,
		numCols:         numCols,
		searchBarHidden: config.SearchBarHidden,
		sortCols:        make([]sortDir, numCols),
	}

	// column widths
	widths := make([]float32, numCols)
	for i, c := range config.Columns {
		widths[i] = c.Width
	}
	widths, err := defineWidths(widths, numCols)
	if err != nil {
		return nil, err
	}
	w.widths = widths
	c := [][]string{headersForWidthsCalc(w.headerCells)}
	w.layout = columnsLayout(minimalColumnWidths(c, w.widths))

	// sorting
	if config.SortedColumnDirection == sortOff {
		config.SortedColumnDirection = SortAsc
	}
	if config.SortedColumnIndex < 0 || config.SortedColumnIndex >= numCols {
		return nil, errors.New("invalid index for initial sort column")
	}
	w.sortCols[config.SortedColumnIndex] = config.SortedColumnDirection

	w.ExtendBaseWidget(w)
	w.body = w.makeBody()
	w.header = w.makeHeader()
	w.searchBar = w.makeSearchBar()
	return w, nil
}

func defineWidths(widths []float32, numCols int) ([]float32, error) {
	if len(widths) == 0 {
		return make([]float32, numCols), nil
	}
	if len(widths) != numCols {
		return nil, fmt.Errorf("need to provide widths for exactly %d columns", numCols)
	}
	return slices.Clone(widths), nil
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
		case SortAsc:
			t2 = t + characterSortAsc
		case SortDesc:
			t2 = t + characterSortDesc
		}
		l := x.(*tappableLabel)
		l.SetText(t2)
	}
	for i, c := range w.sortCols {
		switch c {
		case SortAsc:
			slices.SortFunc(w.cells, func(a, b row) int {
				return cmp.Compare(a.columns[i], b.columns[i])
			})
		case SortDesc:
			slices.SortFunc(w.cells, func(a, b row) int {
				return cmp.Compare(b.columns[i], a.columns[i])
			})
		}
	}
}

func (w *DataTable) makeHeader() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, w.numCols)
	for col, s := range w.headerCells {
		col := col
		o := newTappableLabel(s, nil)
		o.TextStyle.Bold = true
		o.OnTapped = func() {
			for i := 0; i < w.numCols; i++ {
				if i == col {
					if w.sortCols[col] == SortDesc {
						w.sortCols[col] = SortAsc
					} else {
						w.sortCols[col]++
					}
				} else {
					w.sortCols[i] = sortOff
				}
			}
			w.applyFilterAndSort(w.searchBar.Text)
		}
		objects[col] = o
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
			for i := 0; i < w.numCols; i++ {
				l := widget.NewLabel("")
				l.Truncation = fyne.TextTruncateEllipsis
				objects[i] = l
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
			for i := 0; i < w.numCols; i++ {
				o := c.Objects[i].(*widget.Label)
				o.SetText(r.columns[i])
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		defer list.UnselectAll()
		if w.OnSelected == nil {
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

// SetData sets the content of all cells in the table.
// Returns an error if not all rows have the expected number of columns.
func (w *DataTable) SetData(cells [][]string) error {
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
	allCells := slices.Concat([][]string{headersForWidthsCalc(w.headerCells)}, cells)
	w.layout = columnsLayout(minimalColumnWidths(allCells, w.widths))
	w.applySort()
	w.updateFooter()
	return nil
}

func headersForWidthsCalc(header []string) []string {
	h2 := make([]string, len(header))
	for i, v := range header {
		h2[i] = v + characterSortAsc
	}
	return h2
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

// minimalColumnWidths returns the calculated widths for all columns.
// It assumes the first row in cells contains the headers.
func minimalColumnWidths(cells [][]string, widths []float32) []float32 {
	numCols := len(cells[0])
	colWidths := slices.Clone(widths)
	for x := 0; x < numCols; x++ {
		// for c := range numCols {
		var numRows int
		if colWidths[x] != 0 {
			numRows = 1 // only look at headers
		} else {
			numRows = len(cells)
		}
		for y := 0; y < numRows; y++ {
			// for r := range numRows {
			s := cells[y][x]
			l := widget.NewLabel(s)
			w := l.MinSize().Width
			colWidths[x] = float32(math.Ceil(float64(max(w, colWidths[x]))))
		}
	}
	return colWidths
}

func (w *DataTable) CreateRenderer() fyne.WidgetRenderer {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var headerFrame, footerFrame fyne.CanvasObject
	top := container.NewVBox()
	if !w.searchBarHidden {
		top.Add(w.searchBar)
	}
	if !w.headerHidden {
		top.Add(container.NewStack(
			canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground)),
			container.New(w.layout, w.header...),
		))
	}
	if len(top.Objects) > 0 {
		top.Add(widget.NewSeparator())
		headerFrame = top
	}
	if !w.footerHidden {
		footerFrame = container.NewVBox(widget.NewSeparator(), w.footer)
	}
	c := container.NewHScroll(container.NewBorder(headerFrame, footerFrame, nil, nil, w.body))
	return widget.NewSimpleRenderer(c)
}

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

func max(a, b float32) float32 {
	if a < b {
		return b
	}
	return a
}
