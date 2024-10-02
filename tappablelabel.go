package datatable

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// tappableLabel is a Label that can be tapped.
type tappableLabel struct {
	widget.Label

	// The function that is called when the label is tapped.
	OnTapped func()

	hovered bool
}

var _ fyne.Tappable = (*tappableLabel)(nil)
var _ desktop.Hoverable = (*tappableLabel)(nil)

// newTappableLabel returns a new TappableLabel instance.
func newTappableLabel(text string, tapped func()) *tappableLabel {
	l := &tappableLabel{OnTapped: tapped}
	l.ExtendBaseWidget(l)
	l.SetText(text)
	return l
}

func (l *tappableLabel) Tapped(_ *fyne.PointEvent) {
	if l.OnTapped != nil {
		l.OnTapped()
	}
}

// Cursor returns the cursor type of this widget
func (l *tappableLabel) Cursor() desktop.Cursor {
	if l.hovered {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

// MouseIn is a hook that is called if the mouse pointer enters the element.
func (l *tappableLabel) MouseIn(e *desktop.MouseEvent) {
	l.hovered = true
}

func (l *tappableLabel) MouseMoved(*desktop.MouseEvent) {
	// needed to satisfy the interface only
}

// MouseOut is a hook that is called if the mouse pointer leaves the element.
func (l *tappableLabel) MouseOut() {
	l.hovered = false
}
