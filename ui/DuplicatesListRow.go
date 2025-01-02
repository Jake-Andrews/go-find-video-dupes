package ui

// DuplicatesListRow.go
import (
	"fmt"
	"image/color"
	"log"

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
	groupHeaderText      *canvas.Text

	// video row
	screenshotContainer *fyne.Container
	pathText            *widget.Label
	statsLabel          *fyne.Container
	codecsText          *canvas.Text
	linksLabel          *fyne.Container

	videoLayout *fyne.Container

	isColumnsHeader bool
	isGroupHeader   bool
	selected        bool

	onTapped func(itemID int, selected bool)
	itemID   int
}

// NewDuplicatesListRow constructs a row with sub-elements for each usage scenario.
func NewDuplicatesListRow(onTapped func(itemID int, selected bool)) *DuplicatesListRow {
	log.Println("Setting duplicatelistrow")
	row := &DuplicatesListRow{
		onTapped: onTapped,
	}

	//---------------------------------------------------------------------
	// 1) Columns header row
	//---------------------------------------------------------------------
	headerLabel1 := newCenteredTruncatedText("Screenshots")
	headerLabel2 := newCenteredTruncatedText("Path")
	headerLabel3 := newCenteredTruncatedText("Stats")
	headerLabel4 := newCenteredTruncatedText("Codecs")
	headerLabel5 := newCenteredTruncatedText("Links")

	col1Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(532, 50)), headerLabel1),
		color.RGBA{255, 0, 0, 255},
	)
	col2Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(200, 50)), headerLabel2),
		color.RGBA{0, 255, 0, 255},
	)
	col3Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 50)), headerLabel3),
		color.RGBA{0, 0, 255, 255},
	)
	col4Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(100, 50)), headerLabel4),
		color.RGBA{255, 255, 0, 255},
	)
	col5Header := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 50)), headerLabel5),
		color.RGBA{255, 0, 255, 255},
	)

	row.columnsHeaderContainer = wrapWithBorder(
		container.NewHBox(col1Header, col2Header, col3Header, col4Header, col5Header),
		color.RGBA{128, 128, 128, 255},
	)

	//---------------------------------------------------------------------
	// 2) Group header row
	//---------------------------------------------------------------------
	row.groupHeaderText = canvas.NewText("", color.White)
	row.groupHeaderText.Alignment = fyne.TextAlignCenter

	groupCol := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(1200, 50)), row.groupHeaderText),
		color.RGBA{128, 128, 128, 255},
	)
	row.groupHeaderContainer = wrapWithBorder(
		container.NewStack(groupCol),
		color.RGBA{200, 200, 200, 255},
	)

	//---------------------------------------------------------------------
	// 3) Video row
	//---------------------------------------------------------------------
	row.screenshotContainer = wrapWithBorder(
		container.NewCenter(container.NewHBox()),
		color.RGBA{0, 255, 255, 255},
	)
	row.pathText = widget.NewLabel("")
	row.pathText.Alignment = fyne.TextAlignLeading
	row.pathText.Wrapping = fyne.TextWrapBreak

	row.statsLabel = container.NewVBox()
	row.codecsText = canvas.NewText("", color.White)
	row.codecsText.Alignment = fyne.TextAlignLeading
	row.linksLabel = container.NewVBox()

	col1 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(532, 120)), row.screenshotContainer),
		color.RGBA{255, 165, 0, 255},
	)
	col2 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(200, 120)), newLeftAlignedContainer(row.pathText)),
		color.RGBA{0, 128, 128, 255},
	)
	col3 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 120)), row.statsLabel),
		color.RGBA{75, 0, 130, 255},
	)
	col4 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(100, 120)), newLeftAlignedContainer(row.codecsText)),
		color.RGBA{240, 230, 140, 255},
	)
	col5 := wrapWithBorder(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(120, 120)), row.linksLabel),
		color.RGBA{255, 20, 147, 255},
	)

	row.videoLayout = wrapWithBorder(
		container.NewHBox(col1, col2, col3, col4, col5),
		color.RGBA{0, 0, 0, 255},
	)

	row.ExtendBaseWidget(row)
	return row
}

// newCenteredTruncatedText creates a centered canvas.Text with truncation.
func newCenteredTruncatedText(text string) *canvas.Text {
	txt := canvas.NewText(text, color.White)
	txt.Alignment = fyne.TextAlignCenter
	return txt
}

// newLeftAlignedContainer is a helper for left-aligned content in a container
func newLeftAlignedContainer(obj fyne.CanvasObject) *fyne.Container {
	return container.NewVBox(layout.NewSpacer(), obj, layout.NewSpacer())
}

// wrapWithBorder adds a border to a container
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

	// Stack background at the bottom, then place actual content on top
	content := container.NewStack(
		bg,
		container.NewVBox(
			r.columnsHeaderContainer,
			r.groupHeaderContainer,
			r.videoLayout,
		),
	)

	return &duplicatesListRowRenderer{
		row:        r,
		background: bg,
		container:  content,
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
		r.groupHeaderText.Text = item.HeaderText
		r.groupHeaderContainer.Show()
		r.groupHeaderText.Refresh()

	default:
		r.videoLayout.Show()
		r.updateVideoRow(item)
	}

	r.Refresh()
}

// formatFileSize displays size in GB if >= 1GB, otherwise MB
func formatFileSize(sizeBytes int64) string {
	const (
		MB = 1024.0 * 1024.0
		GB = 1024.0 * 1024.0 * 1024.0
	)
	gbVal := float64(sizeBytes) / GB
	if gbVal >= 1.0 {
		return fmt.Sprintf("%.2f GB", gbVal)
	}
	mbVal := float64(sizeBytes) / MB
	return fmt.Sprintf("%.2f MB", mbVal)
}

// formatDuration returns a string hh:mm:ss from a float duration in seconds
func formatDuration(seconds float32) string {
	hours := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}

// Helper function to create a left-aligned, vertically centered container for canvas.Text
func newLeftAlignedCanvasText(text string, clr color.Color) *canvas.Text {
	txt := canvas.NewText(text, clr)
	txt.Alignment = fyne.TextAlignLeading
	return txt
}

// Populate the video row
func (r *DuplicatesListRow) updateVideoRow(item duplicateListItem) {
	vd := item.VideoData
	if vd == nil {
		r.pathText.Text = "(no data)"
		r.pathText.Refresh()
		return
	}

	// 1) Screenshots
	r.screenshotContainer.Objects = nil
	cols := 4
	grid := container.NewGridWithColumns(cols)
	for _, img := range vd.Screenshot.Screenshots {
		fImg := canvas.NewImageFromImage(img)
		fImg.FillMode = canvas.ImageFillContain
		fImg.SetMinSize(fyne.NewSize(100, 100))
		grid.Add(fImg)
	}
	r.screenshotContainer.Objects = []fyne.CanvasObject{
		container.NewCenter(grid),
	}
	r.screenshotContainer.Refresh()

	// 2) Path
	r.pathText.Text = vd.Video.Path
	r.pathText.Refresh()

	// 3) Stats column
	statsObjects := []fyne.CanvasObject{
		layout.NewSpacer(),
		newLeftAlignedCanvasText(formatFileSize(vd.Video.Size), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("%.2f Mbps", float64(vd.Video.BitRate)/1_048_576.0), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("%.2f fps", vd.Video.AvgFrameRate), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("%dx%d", vd.Video.Width, vd.Video.Height), color.White),
		newLeftAlignedCanvasText(formatDuration(vd.Video.Duration), color.White),
		layout.NewSpacer(),
	}
	r.statsLabel.Objects = statsObjects
	r.statsLabel.Refresh()

	// 4) Codecs column
	r.codecsText.Text = fmt.Sprintf("%s / %s", vd.Video.VideoCodec, vd.Video.AudioCodec)
	r.codecsText.Refresh()

	// 5) Links column
	linksObjects := []fyne.CanvasObject{
		layout.NewSpacer(),
		newLeftAlignedCanvasText(fmt.Sprintf("Symbolic: %t", vd.Video.IsSymbolicLink), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("Symbolic: %q", vd.Video.SymbolicLink), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("Hard: %t", vd.Video.IsHardLink), color.White),
		newLeftAlignedCanvasText(fmt.Sprintf("#Hard: %d", vd.Video.NumHardLinks), color.White),
		layout.NewSpacer(),
	}
	r.linksLabel.Objects = linksObjects
	r.linksLabel.Refresh()

	r.videoLayout.Refresh()
}

type duplicatesListRowRenderer struct {
	row        *DuplicatesListRow
	background *canvas.Rectangle
	container  *fyne.Container
}

func (r *duplicatesListRowRenderer) Layout(size fyne.Size) {
	// This is called by Fyne whenever the row is resized
	// log.Println("Row Layout: wanted height =", size.Height)
	// log.Println("pathText.MinSize().Height =", r.row.pathText.MinSize().Height)

	r.background.Resize(size)

	// Adjust position to center content vertically
	contentHeight := r.container.MinSize().Height
	offsetY := (size.Height - contentHeight) / 2

	r.container.Move(fyne.NewPos(0, offsetY))
	r.container.Resize(fyne.NewSize(size.Width, contentHeight))
}

func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
	// For column or group headers, fixed height 50
	if r.row.isGroupHeader {
		return fyne.NewSize(1200, 50)
	} else if r.row.isColumnsHeader {
		return fyne.NewSize(1200, 50)
	}

	// For video rows, ensure at least 148
	contentHeight := r.row.pathText.MinSize().Height
	return fyne.NewSize(1200, fyne.Max(148, contentHeight))
}

func (r *duplicatesListRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.container}
}

func (r *duplicatesListRowRenderer) Refresh() {
	r.background.FillColor = r.row.backgroundColor()
	r.background.Refresh()
	r.container.Refresh()
}

func (r *duplicatesListRowRenderer) Destroy() {}

