package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"log"

	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/image/bmp"
)

type DuplicateGroup struct {
	Videos []*models.VideoData
}

func CreateUI(duplicateGroups []DuplicateGroup) {
	myApp := app.New()
	myWindow := myApp.NewWindow("Duplicate Videos")

	rows := []fyne.CanvasObject{}

	for _, group := range duplicateGroups {
		// Add group label
		rows = append(rows, widget.NewLabel("Duplicate Group"))

		for _, video := range group.Videos {
			videoRow := container.NewHBox(
				widget.NewLabel(video.Video.FileName),
				widget.NewLabel(video.Video.Path),
				widget.NewLabel(fmt.Sprintf("Size: %d", video.Video.Size)),
				widget.NewLabel(fmt.Sprintf("Duration: %.2f", video.Video.Duration)),
			)

			rows = append(rows, videoRow)

			if len(video.Screenshot.Screenshots) > 0 {
				for _, img := range video.Screenshot.Screenshots {
					imageCanvas := canvas.NewImageFromImage(img)
					imageCanvas.SetMinSize(fyne.NewSize(100, 100))
					rows = append(rows, imageCanvas)
				}
			}
		}

		// Add a blank row to separate groups
		rows = append(rows, widget.NewLabel(""))
	}

	content := container.NewVBox(rows...) // Combine rows into a vertical layout
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.ShowAndRun()
}

func loadScreenshots(encodedScreenshots []string) []image.Image {
	var screenshots []image.Image
	for _, b64 := range encodedScreenshots {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			log.Printf("Error decoding base64 screenshot: %v", err)
			continue
		}
		img, err := bmp.Decode(bytes.NewReader(data))
		if err != nil {
			log.Printf("Error decoding PNG image: %v", err)
			continue
		}
		screenshots = append(screenshots, img)
	}
	return screenshots
}
