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
	FooterHidden    bool
	HeaderHidden    bool
	SearchBarHidden bool

	// Callback runs when an entry is selected.
	// index refers to the row in the original data.
	OnSelected func(index int)

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

// New returns a new DataTable widget.
// By default all columns will be automatically sized to fit the data.
func New(header []string) *DataTable {
	dt := &DataTable{
		footer:      widget.NewLabel(""),
		headerCells: header,
		numCols:     len(header),
		sortCols:    make([]sortDir, len(header)),
		widths:      make([]float32, len(header)),
	}
	dt.ExtendBaseWidget(dt)
	dt.body = dt.makeBody()
	dt.header = dt.makeHeader()
	dt.searchBar = dt.makeSearchBar()
	dt.sortCols[0] = sortAsc
	return dt
}

// // NewDataTable returns a new DataTable with fixed columns widths.
// func NewDataTableWithFixedColumns(header []string, widths []float32) (*DataTable, error) {
// 	w := makeWidget(header)
// 	if len(widths) != len(header) {
// 		return nil, fmt.Errorf("need to provide widths for exactly %d columns", w.numCols)
// 	}
// 	// width of headers is minimum for each column
// 	hw := maxColWidths([][]string{headersForWidthsCalc(header)})
// 	w.widths = make([]float32, len(widths))
// 	for i := range len(widths) {
// 		w.widths[i] = max(widths[i], hw[i])
// 	}
// 	w.layout = columnsLayout(w.widths)
// 	return w, nil
// }

func (dt *DataTable) SetColumnWidths(widths []float32) error {
	if len(widths) != dt.numCols {
		return fmt.Errorf("need to provide widths for exactly %d columns", dt.numCols)
	}
	dt.widths = widths
	return nil
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

// SetCells sets the content of all cells in the table.
// Returns an error if not all rows have the same number of columns as the header.
func (dt *DataTable) SetCells(cells [][]string) error {
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
	dt.layout = columnsLayout(maxColWidths(allCells, dt.widths))
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

func maxColWidths(cells [][]string, widths []float32) []float32 {
	numRows := len(cells)
	numCols := len(cells[0])
	colWidths := slices.Clone(widths)
	for c := range numCols {
		if colWidths[c] != 0 {
			continue // only calculate if width is not set
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
	if !dt.SearchBarHidden {
		top.Add(container.NewGridWithColumns(3, layout.NewSpacer(), dt.searchBar, layout.NewSpacer()))
	}
	if !dt.HeaderHidden {
		top.Add(container.NewStack(
			canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground)),
			container.New(dt.layout, dt.header...),
		))
	}
	if len(top.Objects) > 0 {
		top.Add(widget.NewSeparator())
		headerFrame = top
	}
	if !dt.FooterHidden {
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
