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

// DuplicatesListRow represents a single row in the DuplicatesList. It can be:
//  1. The columns header row (shown once, if any videos exist)
//  2. A group header row
//  3. A video row
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
	// Columns header (aligned with columns below)
	//---------------------------------------------------------------------
	headerLabel1 := widget.NewLabel("Screenshots")
	headerLabel2 := widget.NewLabel("Path")
	headerLabel3 := widget.NewLabel("Stats")
	headerLabel4 := widget.NewLabel("Codecs")
	headerLabel5 := widget.NewLabel("Links")

	// Set text alignment for headers
	headerLabel1.Alignment = fyne.TextAlignLeading
	headerLabel2.Alignment = fyne.TextAlignLeading
	headerLabel3.Alignment = fyne.TextAlignLeading
	headerLabel4.Alignment = fyne.TextAlignLeading
	headerLabel5.Alignment = fyne.TextAlignLeading

	row.columnsHeaderContainer = container.NewHBox(
		leftAligned(headerLabel1, 532),
		leftAligned(headerLabel2, 200),
		leftAligned(headerLabel3, 120),
		leftAligned(headerLabel4, 100),
		leftAligned(headerLabel5, 120),
	)

	//---------------------------------------------------------------------
	// Group header (centered horizontally & vertically)
	//---------------------------------------------------------------------
	row.groupHeaderLabel = widget.NewLabel("")
	row.groupHeaderLabel.Alignment = fyne.TextAlignCenter
	groupHeader := centerBoth(row.groupHeaderLabel)
	row.groupHeaderContainer = container.NewVBox(groupHeader)

	//---------------------------------------------------------------------
	// Video row
	//---------------------------------------------------------------------
	row.screenshotContainer = container.NewWithoutLayout()
	row.pathLabel = widget.NewLabel("")
	row.statsLabel = widget.NewLabel("")
	row.codecsLabel = widget.NewLabel("")
	row.linksLabel = widget.NewLabel("")

	// For each column, align content to the left and center vertically
	col1 := leftAligned(row.screenshotContainer, 532)
	col2 := leftAligned(row.pathLabel, 200)
	col3 := leftAligned(row.statsLabel, 120)
	col4 := leftAligned(row.codecsLabel, 100)
	col5 := leftAligned(row.linksLabel, 120)

	row.videoLayout = container.NewHBox(col1, col2, col3, col4, col5)

	row.ExtendBaseWidget(row)
	return row
}

// centerBoth returns a container that centers an object both vertically and horizontally.
func centerBoth(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCenterLayout(), obj)
}

// leftAligned aligns an object vertically to the center and horizontally to the left.
func leftAligned(obj fyne.CanvasObject, width float32) fyne.CanvasObject {
	// Ensure text or object alignment is leading (left)
	if label, ok := obj.(*widget.Label); ok {
		label.Alignment = fyne.TextAlignLeading
	}

	// Use a VBox layout to center the object vertically
	return container.New(layout.NewVBoxLayout(),
		container.New(layout.NewGridWrapLayout(fyne.NewSize(width, 40)), obj),
	)
}

func (r *DuplicatesListRow) Tapped(_ *fyne.PointEvent) {
	// If it's the columns header or a group header, do nothing
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

	// We stack 3 possible containers:
	// 1) columnsHeaderContainer
	// 2) groupHeaderContainer
	// 3) videoLayout
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

// backgroundColor returns a highlight color if selected, else transparent.
func (r *DuplicatesListRow) backgroundColor() color.Color {
	if r.selected {
		// Use a gentle highlight color
		return color.RGBA{R: 173, G: 216, B: 230, A: 128}
	}
	return color.RGBA{0, 0, 0, 0}
}

// Update configures this row as either a columns header, group header, or video row.
func (r *DuplicatesListRow) Update(item duplicateListItem) {
	r.isColumnsHeader = item.IsColumnsHeader
	r.isGroupHeader = item.IsGroupHeader
	r.selected = item.Selected

	// Hide everything initially
	r.columnsHeaderContainer.Hide()
	r.groupHeaderContainer.Hide()
	r.videoLayout.Hide()

	switch {
	case r.isColumnsHeader:
		// Show the columns header row
		r.columnsHeaderContainer.Show()

	case r.isGroupHeader:
		// Show the group header
		r.groupHeaderLabel.SetText(item.HeaderText)
		r.groupHeaderContainer.Show()

	default:
		// Normal video row
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

	//---------------------------------------------------------------------
	// Screenshots: place them horizontally side-by-side, no margin
	//---------------------------------------------------------------------
	r.screenshotContainer.Objects = nil
	var xPos float32
	for _, img := range vd.Screenshot.Screenshots {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(100, 100))

		fImg.Move(fyne.NewPos(xPos, 0))
		xPos += fImg.MinSize().Width

		r.screenshotContainer.Add(fImg)
	}
	r.screenshotContainer.Resize(fyne.NewSize(xPos, 100))

	// Path
	r.pathLabel.SetText(vd.Video.Path)
	r.pathLabel.Alignment = fyne.TextAlignLeading

	// Stats
	statsString := fmt.Sprintf("%.2f MB\n%.2f Mbps\n%.2f fps\n%dx%d\n%02d:%02d:%02d",
		float64(vd.Video.Size)/(1024.0*1024.0),
		(float64(vd.Video.BitRate)/1024.0/1024.0)*8.0,
		vd.Video.FrameRate,
		vd.Video.Width,
		vd.Video.Height,
		int(vd.Video.Duration)/3600,
		(int(vd.Video.Duration)%3600)/60,
		int(vd.Video.Duration)%60,
	)
	r.statsLabel.SetText(statsString)
	r.statsLabel.Alignment = fyne.TextAlignLeading

	// Codecs
	r.codecsLabel.SetText(fmt.Sprintf("%s / %s", vd.Video.AudioCodec, vd.Video.VideoCodec))
	r.codecsLabel.Alignment = fyne.TextAlignLeading

	// Links
	linkString := fmt.Sprintf("Symbolic? %t\nLink: %q\nHard? %t\nCount: %d",
		vd.Video.IsSymbolicLink,
		vd.Video.SymbolicLink,
		vd.Video.IsHardLink,
		vd.Video.NumHardLinks-1,
	)
	r.linksLabel.SetText(linkString)
	r.linksLabel.Alignment = fyne.TextAlignLeading
}

type duplicatesListRowRenderer struct {
	row        *DuplicatesListRow
	background *canvas.Rectangle
	container  *fyne.Container
}

func (r *duplicatesListRowRenderer) Destroy() {}

func (r *duplicatesListRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)

	// Ensure the columns header, group header, and video layout are constrained
	if r.row.isColumnsHeader {
		r.row.columnsHeaderContainer.Resize(fyne.NewSize(size.Width, 40)) // Height for header row
		r.row.columnsHeaderContainer.Move(fyne.NewPos(0, 0))
	}

	if r.row.isGroupHeader {
		r.row.groupHeaderContainer.Resize(fyne.NewSize(size.Width, 40)) // Adjust height if needed
		r.row.groupHeaderContainer.Move(fyne.NewPos(0, 0))
	}

	if !r.row.isColumnsHeader && !r.row.isGroupHeader {
		r.row.videoLayout.Resize(size) // Ensure video layout fits the row size
		r.row.videoLayout.Move(fyne.NewPos(0, 0))
	}

	r.container.Resize(size)
}

func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(600, 120)
}

func (r *duplicatesListRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.container}
}

func (r *duplicatesListRowRenderer) Refresh() {
	r.background.FillColor = r.row.backgroundColor()
	r.background.Refresh()
	r.container.Refresh()
}
