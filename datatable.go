package datatable

import (
	"cmp"
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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type sortDir uint

// sort directions for columns
const (
	sortOff sortDir = iota
	sortAsc
	sortDesc
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

// DataTable is a Fyne widget representing a table for showing data.
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
	sortCols        []sortDir
	widths          []float32

	mu            sync.RWMutex
	layout        columnsLayout
	cells         []row
	cellsFiltered []row
}

// Config is a struct for configuring a DataTable widget.
// The Header field is mandatory. All other fields are optional.
type Config struct {
	// ColumnWidths sets the width of each column.
	// A column with width 0 will be auto-sized to fit the data in that column.
	// The width will be adjusted to fit the column header.
	ColumnWidths []float32

	// Whether to hide the footer.
	FooterHidden bool

	// Header sets the content of the header columns MANDATORY.
	Header []string

	// Whether to hide the header.
	HeaderHidden bool

	// Whether to hide the search bar.
	SearchBarHidden bool
}

// New returns a new DataTable widget.
// The widgets is configured with a [Config] struct.
func New(cfg Config) (*DataTable, error) {
	if len(cfg.Header) == 0 {
		return nil, fmt.Errorf("headers must be defined")
	}
	numCols := len(cfg.Header)
	dt := &DataTable{
		footer:          widget.NewLabel(""),
		footerHidden:    cfg.FooterHidden,
		headerCells:     cfg.Header,
		headerHidden:    cfg.HeaderHidden,
		numCols:         numCols,
		searchBarHidden: cfg.SearchBarHidden,
		sortCols:        make([]sortDir, numCols),
	}
	w, err := defineWidths(cfg.ColumnWidths, numCols)
	if err != nil {
		return nil, err
	}
	dt.widths = w
	dt.ExtendBaseWidget(dt)
	dt.body = dt.makeBody()
	dt.header = dt.makeHeader()
	dt.searchBar = dt.makeSearchBar()
	dt.sortCols[0] = sortAsc
	return dt, nil
}

func defineWidths(w []float32, numCols int) ([]float32, error) {
	if len(w) == 0 {
		return make([]float32, numCols), nil
	}
	if len(w) != numCols {
		return nil, fmt.Errorf("need to provide widths for exactly %d columns", numCols)
	}
	return slices.Clone(w), nil
}

func (dt *DataTable) makeSearchBar() *widget.Entry {
	e := widget.NewEntry()
	e.ActionItem = widget.NewIcon(theme.SearchIcon())
	e.OnChanged = func(s string) {
		dt.applyFilterAndSort(s)
	}
	return e
}

func (dt *DataTable) applyFilterAndSort(search string) {
	func() {
		dt.mu.Lock()
		defer dt.mu.Unlock()
		dt.applySort()
		var selection []row
		s2 := strings.ToLower(search)
		for _, row := range dt.cells {
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
		dt.cellsFiltered = selection
	}()
	dt.updateFooter()
	dt.body.Refresh()
}

func (dt *DataTable) applySort() {
	for i, x := range dt.header {
		t := dt.headerCells[i]
		var t2 string
		switch dt.sortCols[i] {
		case sortOff:
			t2 = t
		case sortAsc:
			t2 = t + characterSortAsc
		case sortDesc:
			t2 = t + characterSortDesc
		}
		l := x.(*tappableLabel)
		l.SetText(t2)
	}
	for i, c := range dt.sortCols {
		switch c {
		case sortAsc:
			slices.SortFunc(dt.cells, func(a, b row) int {
				return cmp.Compare(a.columns[i], b.columns[i])
			})
		case sortDesc:
			slices.SortFunc(dt.cells, func(a, b row) int {
				return cmp.Compare(b.columns[i], a.columns[i])
			})
		}
	}
}

func (dt *DataTable) makeHeader() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, dt.numCols)
	for i, s := range dt.headerCells {
		o := newTappableLabel(s, nil)
		o.TextStyle.Bold = true
		o.OnTapped = func() {
			for j := range dt.numCols {
				if j == i {
					if dt.sortCols[i] == sortDesc {
						dt.sortCols[i] = sortAsc
					} else {
						dt.sortCols[i]++
					}
				} else {
					dt.sortCols[j] = sortOff
				}
			}
			dt.applyFilterAndSort(dt.searchBar.Text)
		}
		objects[i] = o
	}
	return objects
}

func (dt *DataTable) makeBody() *widget.List {
	list := widget.NewList(
		func() int {
			dt.mu.RLock()
			defer dt.mu.RUnlock()
			return len(dt.cellsFiltered)
		},
		func() fyne.CanvasObject {
			dt.mu.RLock()
			defer dt.mu.RUnlock()
			objects := make([]fyne.CanvasObject, dt.numCols)
			for i := range dt.numCols {
				l := widget.NewLabel("")
				l.Truncation = fyne.TextTruncateEllipsis
				objects[i] = l
			}
			return container.New(dt.layout, objects...)
		},
		func(id widget.ListItemID, co fyne.CanvasObject) {
			dt.mu.RLock()
			defer dt.mu.RUnlock()
			if id >= len(dt.cellsFiltered) {
				return // safeguard
			}
			r := dt.cellsFiltered[id]
			c := co.(*fyne.Container)
			for i := range dt.numCols {
				o := c.Objects[i].(*widget.Label)
				o.SetText(r.columns[i])
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		defer list.UnselectAll()
		if dt.OnSelected == nil {
			return
		}
		dt.mu.RLock()
		defer dt.mu.RUnlock()
		if id >= len(dt.cellsFiltered) {
			return // safeguard
		}
		dt.OnSelected(dt.cellsFiltered[id].idx)
	}
	return list
}

// SetData sets the content of all cells in the table.
// Returns an error if not all rows have the expected number of columns.
func (dt *DataTable) SetData(cells [][]string) error {
	defer dt.body.Refresh()
	dt.mu.Lock()
	defer dt.mu.Unlock()
	for _, r := range cells {
		if len(r) != dt.numCols {
			return fmt.Errorf("some rows do not have %d columns", dt.numCols)
		}
	}
	dt.cells = make([]row, len(cells))
	for i, r := range cells {
		dt.cells[i] = row{idx: i, columns: r}
	}
	dt.cellsFiltered = slices.Clone(dt.cells)
	allCells := slices.Concat([][]string{headersForWidthsCalc(dt.headerCells)}, cells)
	dt.layout = columnsLayout(minimalColumnWidths(allCells, dt.widths))
	dt.applySort()
	dt.updateFooter()
	return nil
}

func headersForWidthsCalc(header []string) []string {
	h2 := make([]string, len(header))
	for i, v := range header {
		h2[i] = v + characterSortAsc
	}
	return h2
}

func (dt *DataTable) updateFooter() {
	var s string
	p := message.NewPrinter(language.English)
	if len(dt.cellsFiltered) < len(dt.cells) {
		s = p.Sprintf("%d of %d entries (filtered)", len(dt.cellsFiltered), len(dt.cells))
	} else {
		s = p.Sprintf("%d entries", len(dt.cells))
	}
	dt.footer.SetText(s)
}

// minimalColumnWidths returns the calculated widths for all columns.
// It assumes the first row in cells contains the headers.
func minimalColumnWidths(cells [][]string, widths []float32) []float32 {
	numCols := len(cells[0])
	colWidths := slices.Clone(widths)
	for c := range numCols {
		var numRows int
		if colWidths[c] != 0 {
			numRows = 1 // only look at headers
		} else {
			numRows = len(cells)
		}
		for r := range numRows {
			s := cells[r][c]
			l := widget.NewLabel(s)
			w := l.MinSize().Width
			colWidths[c] = float32(math.Ceil(float64(max(w, colWidths[c]))))
		}
	}
	return colWidths
}

func (dt *DataTable) CreateRenderer() fyne.WidgetRenderer {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	var headerFrame, footerFrame fyne.CanvasObject
	top := container.NewVBox()
	if !dt.searchBarHidden {
		top.Add(container.NewGridWithColumns(3, layout.NewSpacer(), dt.searchBar, layout.NewSpacer()))
	}
	if !dt.headerHidden {
		top.Add(container.NewStack(
			canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground)),
			container.New(dt.layout, dt.header...),
		))
	}
	if len(top.Objects) > 0 {
		top.Add(widget.NewSeparator())
		headerFrame = top
	}
	if !dt.footerHidden {
		footerFrame = container.NewVBox(widget.NewSeparator(), dt.footer)
	}
	c := container.NewBorder(headerFrame, footerFrame, nil, nil, dt.body)
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
