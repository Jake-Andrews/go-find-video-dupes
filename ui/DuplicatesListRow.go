package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// DuplicatesListRow represents a single row in the DuplicatesList.
type DuplicatesListRow struct {
	widget.BaseWidget

	// "header" row for columns
	columnsHeaderContainer *fyne.Container

	// group header row
	groupHeaderContainer *fyne.Container
	groupHeaderLabel     *widget.Label

	// video row
	screenshotContainer *fyne.Container
	pathLabel           *widget.Label
	statsLabel          *widget.Label
	codecsLabel         *widget.Label
	linksLabel          *widget.Label

	videoLayout *fyne.Container

	isColumnsHeader bool
	isGroupHeader   bool
	selected        bool

	onTapped func(itemID int, selected bool)
	itemID   int
}

// NewDuplicatesListRow constructs a row with sub-elements for each usage scenario.
func NewDuplicatesListRow(onTapped func(itemID int, selected bool)) *DuplicatesListRow {
	row := &DuplicatesListRow{
		onTapped: onTapped,
	}

	//---------------------------------------------------------------------
	// 1) Columns header row
	//---------------------------------------------------------------------
	headerLabel1 := newCenteredTruncatedLabel("Screenshots")
	headerLabel2 := newCenteredTruncatedLabel("Path")
	headerLabel3 := newCenteredTruncatedLabel("Stats")
	headerLabel4 := newCenteredTruncatedLabel("Codecs")
	headerLabel5 := newCenteredTruncatedLabel("Links")

	// Each header column uses grid-wrap with these widths & uniform height:
	col1Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(532, 40)), headerLabel1),
		color.RGBA{255, 0, 0, 255}, // Red border for debugging
	)
	col2Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(200, 40)), headerLabel2),
		color.RGBA{0, 255, 0, 255}, // Green border for debugging
	)
	col3Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 40)), headerLabel3),
		color.RGBA{0, 0, 255, 255}, // Blue border for debugging
	)
	col4Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(100, 40)), headerLabel4),
		color.RGBA{255, 255, 0, 255}, // Yellow border for debugging
	)
	col5Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 40)), headerLabel5),
		color.RGBA{255, 0, 255, 255}, // Magenta border for debugging
	)

	row.columnsHeaderContainer = wrapWithBorder(
		container.NewHBox(col1Header, col2Header, col3Header, col4Header, col5Header),
		color.RGBA{128, 128, 128, 255}, // Gray border for entire header row
	)

	//---------------------------------------------------------------------
	// 2) Group header row
	//---------------------------------------------------------------------
	row.groupHeaderLabel = widget.NewLabel("")
	row.groupHeaderLabel.Alignment = fyne.TextAlignCenter
	row.groupHeaderLabel.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)

	groupCol := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(1024, 40)), row.groupHeaderLabel),
		color.RGBA{128, 128, 128, 255}, // Gray border for debugging
	)
	row.groupHeaderContainer = wrapWithBorder(container.NewStack(groupCol), color.RGBA{200, 200, 200, 255})

	//---------------------------------------------------------------------
	// 3) Video row
	//---------------------------------------------------------------------
	row.screenshotContainer = wrapWithBorder(container.NewCenter(container.NewHBox()), color.RGBA{0, 255, 255, 255}) // Cyan border for debugging

	row.pathLabel = newCenteredTruncatedLabel("")
	row.statsLabel = newCenteredTruncatedLabel("")
	row.codecsLabel = newCenteredTruncatedLabel("")
	row.linksLabel = newCenteredTruncatedLabel("")

	col1 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(532, 120)), row.screenshotContainer),
		color.RGBA{255, 165, 0, 255}, // Orange border for debugging
	)
	col2 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(200, 120)), newLeftAlignedContainer(row.pathLabel)),
		color.RGBA{0, 128, 128, 255}, // Teal border for debugging
	)
	col3 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 120)), newLeftAlignedContainer(row.statsLabel)),
		color.RGBA{75, 0, 130, 255}, // Indigo border for debugging
	)
	col4 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(100, 120)), newLeftAlignedContainer(row.codecsLabel)),
		color.RGBA{240, 230, 140, 255}, // Khaki border for debugging
	)
	col5 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 120)), newLeftAlignedContainer(row.linksLabel)),
		color.RGBA{255, 20, 147, 255}, // Pink border for debugging
	)

	row.videoLayout = wrapWithBorder(container.NewHBox(col1, col2, col3, col4, col5), color.RGBA{0, 0, 0, 255})

	row.ExtendBaseWidget(row)
	return row
}

// newCenteredTruncatedLabel returns a label with the given text, center-aligned and truncated
func newCenteredTruncatedLabel(text string) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.Alignment = fyne.TextAlignCenter
	lbl.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	return lbl
}

// Helper function to create a left-aligned, vertically centered container
func newLeftAlignedContainer(obj fyne.CanvasObject) *fyne.Container {
	return container.NewVBox(layout.NewSpacer(), obj, layout.NewSpacer())
}

// Helper function to wrap a container with a border
func wrapWithBorder(obj fyne.CanvasObject, borderColor color.Color) *fyne.Container {
	border := canvas.NewRectangle(borderColor)
	border.StrokeColor = borderColor
	border.StrokeWidth = 2
	return container.NewBorder(border, border, border, border, obj)
}

func (r *DuplicatesListRow) Tapped(_ *fyne.PointEvent) {
	if r.isColumnsHeader || r.isGroupHeader {
		return
	}
	r.selected = !r.selected
	r.Refresh()
	if r.onTapped != nil {
		r.onTapped(r.itemID, r.selected)
	}
}

func (r *DuplicatesListRow) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(r.backgroundColor())

	overlay := container.NewWithoutLayout(bg)
	overlay.Add(r.columnsHeaderContainer)
	overlay.Add(r.groupHeaderContainer)
	overlay.Add(r.videoLayout)

	return &duplicatesListRowRenderer{
		row:        r,
		background: bg,
		container:  overlay,
	}
}

func (r *DuplicatesListRow) backgroundColor() color.Color {
	if r.selected {
		return color.RGBA{R: 173, G: 216, B: 230, A: 128}
	}
	return color.RGBA{0, 0, 0, 0}
}

func (r *DuplicatesListRow) Update(item duplicateListItem) {
	r.isColumnsHeader = item.IsColumnsHeader
	r.isGroupHeader = item.IsGroupHeader
	r.selected = item.Selected

	r.columnsHeaderContainer.Hide()
	r.groupHeaderContainer.Hide()
	r.videoLayout.Hide()

	switch {
	case r.isColumnsHeader:
		r.columnsHeaderContainer.Show()
	case r.isGroupHeader:
		r.groupHeaderLabel.SetText(item.HeaderText)
		r.groupHeaderContainer.Show()
	default:
		r.videoLayout.Show()
		r.updateVideoRow(item)
	}

	r.Refresh()
}

func (r *DuplicatesListRow) updateVideoRow(item duplicateListItem) {
	vd := item.VideoData
	if vd == nil {
		r.pathLabel.SetText("(no data)")
		return
	}

	r.screenshotContainer.Objects = nil

	// Create a grid layout for the screenshots
	cols := 4 // Adjust the number of columns based on your requirement

	grid := container.NewGridWithColumns(cols)
	for _, img := range vd.Screenshot.Screenshots {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(100, 100)) // Ensure uniform size
		grid.Add(fImg)
	}

	// Center the grid within the screenshot container
	r.screenshotContainer.Objects = []fyne.CanvasObject{
		container.NewCenter(grid),
	}
	r.screenshotContainer.Refresh()

	// Update the other video details
	r.pathLabel.SetText(vd.Video.Path)
	statsString := fmt.Sprintf("%.2f MB | %.2f Mbps | %.2f fps | %dx%d | %02d:%02d:%02d",
		float64(vd.Video.Size)/(1024.0*1024.0),
		(float64(vd.Video.BitRate)/1024.0/1024.0)*8.0,
		vd.Video.FrameRate,
		vd.Video.Width, vd.Video.Height,
		int(vd.Video.Duration)/3600,
		(int(vd.Video.Duration)%3600)/60,
		int(vd.Video.Duration)%60,
	)
	r.statsLabel.SetText(statsString)
	r.codecsLabel.SetText(fmt.Sprintf("%s / %s", vd.Video.AudioCodec, vd.Video.VideoCodec))
	linkString := fmt.Sprintf("Symbolic? %t | Hard? %t | Count: %d",
		vd.Video.IsSymbolicLink,
		vd.Video.IsHardLink,
		vd.Video.NumHardLinks-1,
	)
	r.linksLabel.SetText(linkString)
}

type duplicatesListRowRenderer struct {
	row        *DuplicatesListRow
	background *canvas.Rectangle
	container  *fyne.Container
}

// Destroy handles cleanup for the renderer.
func (r *duplicatesListRowRenderer) Destroy() {}

// Layout arranges the objects within the renderer.
func (r *duplicatesListRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)

	// Columns header row (if used)
	if r.row.isColumnsHeader {
		r.row.columnsHeaderContainer.Resize(fyne.NewSize(size.Width, 40))
		r.row.columnsHeaderContainer.Move(fyne.NewPos(0, 0))
	}

	// Group header row (if used)
	if r.row.isGroupHeader {
		r.row.groupHeaderContainer.Resize(fyne.NewSize(size.Width, 40))
		r.row.groupHeaderContainer.Move(fyne.NewPos(0, 0))
	}

	// Video row (if used)
	if !r.row.isColumnsHeader && !r.row.isGroupHeader {
		r.row.videoLayout.Resize(size)
		r.row.videoLayout.Move(fyne.NewPos(0, 0))
	}

	r.container.Resize(size)
}

// MinSize calculates the minimum size of the renderer.
func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(600, 148)
}

// Objects returns the objects to be drawn by the renderer.
func (r *duplicatesListRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.container}
}

// Refresh updates the renderer with the current state.
func (r *duplicatesListRowRenderer) Refresh() {
	r.background.FillColor = r.row.backgroundColor()
	r.background.Refresh()
	r.container.Refresh()
}
