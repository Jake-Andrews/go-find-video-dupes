package ui

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

type DuplicatesListRow struct {
	widget.BaseWidget

	// Sub-objects for different row states
	headerLabel         *widget.Label
	videoLayout         *fyne.Container
	screenshotContainer *fyne.Container
	pathLabel           *widget.Label
	statsContainer      *fyne.Container
	codecsLabel         *widget.Label
	linksContainer      *fyne.Container

	isHeader bool
	selected bool
}

func NewDuplicatesListRow() *DuplicatesListRow {
	row := &DuplicatesListRow{
		headerLabel:         widget.NewLabel(""),
		screenshotContainer: container.NewWithoutLayout(), // will fill in dynamically
		pathLabel:           widget.NewLabel(""),
		statsContainer:      container.NewWithoutLayout(),
		codecsLabel:         widget.NewLabel(""),
		linksContainer:      container.NewWithoutLayout(),
	}

	// We'll dynamically fill each container with H or V boxes
	// The “videoLayout” has 5 columns side by side
	row.videoLayout = container.New(layout.NewGridLayout(5),
		row.screenshotContainer,
		row.pathLabel,
		row.statsContainer,
		row.codecsLabel,
		row.linksContainer,
	)

	row.ExtendBaseWidget(row)
	return row
}

func (r *DuplicatesListRow) CreateRenderer() fyne.WidgetRenderer {
	// We'll stack a colored rectangle behind everything
	bg := canvas.NewRectangle(r.backgroundColor())

	// Put headerLabel and videoLayout in a Max container,
	// showing one or the other. We can Hide/Show in Update().
	overlay := container.NewStack(r.headerLabel, r.videoLayout)
	// Alternatively, use container.NewStack if you want them layered in order.

	c := container.NewStack(bg, overlay)
	return &duplicatesListRowRenderer{
		row:          r,
		background:   bg,
		containerAll: c,
	}
}

func (r *DuplicatesListRow) backgroundColor() color.Color {
	if r.selected {
		// Light transparent blue for selected
		return color.RGBA{R: 173, G: 216, B: 230, A: 128}
	}
	// Transparent otherwise
	return color.RGBA{0, 0, 0, 0}
}

// Update updates the row’s fields based on the item data
func (r *DuplicatesListRow) Update(item duplicateListItem) {
	log.Printf("DuplicatesListRow.Update: header=%v, selected=%v", item.IsHeader, item.Selected)

	r.isHeader = item.IsHeader
	r.selected = item.Selected

	if item.IsHeader {
		// Show the header label, hide the video layout
		r.headerLabel.SetText(item.HeaderText)
		r.headerLabel.Show()
		r.videoLayout.Hide()
		return
	}

	// It's a video row. Hide the header, show the video layout
	r.headerLabel.Hide()
	r.videoLayout.Show()

	vd := item.VideoData
	if vd == nil {
		r.pathLabel.SetText("(no data)")
		return
	}

	// 1) Screenshots side by side, no extra vertical space
	// Clear the old contents
	r.screenshotContainer.Objects = nil

	if len(vd.Screenshot.Screenshots) == 0 {
		// Just show “No screenshots”
		label := widget.NewLabel("No screenshots")
		// Put it in a horizontal box. If you truly want zero space,
		// you can create a custom layout. We'll do a simple HBox:
		r.screenshotContainer.Add(container.NewHBox(label))
	} else {
		// If multiple screenshots, put them side by side in a horizontal layout
		hbox := container.NewHBox()
		for _, img := range vd.Screenshot.Screenshots {
			fImg := canvas.NewImageFromImage(img)
			fImg.FillMode = canvas.ImageFillContain
			fImg.SetMinSize(fyne.NewSize(100, 100))
			hbox.Add(fImg) // side by side
		}
		r.screenshotContainer.Add(hbox)
	}

	// 2) Path
	r.pathLabel.SetText(vd.Video.Path)

	// 3) Stats: each piece of info on its own line
	r.statsContainer.Objects = nil
	sizeMB := float64(vd.Video.Size) / (1024.0 * 1024.0)
	bitrateMbps := (float64(vd.Video.BitRate) / (1024.0 * 1024.0)) * 8.0
	dur := int(vd.Video.Duration)
	hh, mm, ss := dur/3600, (dur%3600)/60, dur%60

	// Create a vertical box of labels
	vbStats := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("%.2f MB", sizeMB)),
		widget.NewLabel(fmt.Sprintf("%.2f Mbps", bitrateMbps)),
		widget.NewLabel(fmt.Sprintf("%.2f fps", vd.Video.FrameRate)),
		widget.NewLabel(fmt.Sprintf("%dx%d", vd.Video.Width, vd.Video.Height)),
		widget.NewLabel(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)),
	)
	r.statsContainer.Add(vbStats)

	// 4) Audio/Video codecs
	r.codecsLabel.SetText(fmt.Sprintf("%s / %s", vd.Video.AudioCodec, vd.Video.VideoCodec))

	// 5) Link info: each line on its own
	r.linksContainer.Objects = nil
	numHard := vd.Video.NumHardLinks - 1
	vbLinks := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Symbolic? %t", vd.Video.IsSymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Link: %q", vd.Video.SymbolicLink)),
		widget.NewLabel(fmt.Sprintf("Hard? %t", vd.Video.IsHardLink)),
		widget.NewLabel(fmt.Sprintf("Count: %d", numHard)),
	)
	r.linksContainer.Add(vbLinks)

	// Force a refresh of this row
	r.Refresh()
}

type duplicatesListRowRenderer struct {
	row          *DuplicatesListRow
	background   *canvas.Rectangle
	containerAll *fyne.Container
}

func (r *duplicatesListRowRenderer) Destroy() {}

func (r *duplicatesListRowRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	r.containerAll.Resize(size)
}

//	func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
//		return r.containerAll.MinSize()
//	}
func (r *duplicatesListRowRenderer) MinSize() fyne.Size {
	// Get the current minimum size of the container
	minSize := r.containerAll.MinSize()

	// Adjust the height to be at least 120
	if minSize.Height < 120 {
		minSize.Height = 120
	}

	return minSize
}

func (r *duplicatesListRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.containerAll}
}

func (r *duplicatesListRowRenderer) Refresh() {
	r.background.FillColor = r.row.backgroundColor()
	r.background.Refresh()
	r.containerAll.Refresh()
}

