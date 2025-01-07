package ui

import (
	"log/slog"
	"strings"

	"govdupes/internal/config"
	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

type formStruct struct {
	DatabasePath     string
	LogFilePath      string
	IgnoreStr        string
	IncludeStr       string
	IgnoreExt        string
	IncludeExt       string
	SavedScreenshots bool
	AbsPath          bool
	FollowSymbolic   bool
	SkipSymbolic     bool
	SilentFFmpeg     bool
	FilesizeCutoff   int64
}

func buildConfigTab(cfg *config.Config, checkWidget *widget.Check) fyne.CanvasObject {
	// REST OF CONFIG (binding settings to widgets)
	formStruct := ConvertConfigToFormStruct(cfg)

	// intermediate struct
	formData := binding.BindStruct(&formStruct)
	form := newFormWithData(formData)

	form.Append("check", checkWidget)

	dirStr := binding.NewString()
	dirEntry := widget.NewEntryWithData(dirStr)

	startingDirs := binding.BindStringList(&[]string{"./"})

	appendBtn := widget.NewButton("Append", func() {
		startingDirs.Append(dirEntry.Text)
	})
	deleteBtn := widget.NewButton("Delete", func() {
		l := startingDirs.Length() - 1
		if l < 0 {
			return
		}
		str, err := startingDirs.GetValue(l)
		if err != nil {
			slog.Warn("failed getting value from startingDirs list", slog.Any("Error", err))
			return
		}
		err = startingDirs.Remove(str)
		if err != nil {
			slog.Warn("failed removing value from startingDirs list", slog.Any("Error", err))
		}
	})

	dirList := widget.NewListWithData(startingDirs,
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil, nil,
				widget.NewLabel(""))
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			f := item.(binding.String)
			text := obj.(*fyne.Container).Objects[0].(*widget.Label)
			text.Bind(f)
		})

	startingDirsLabel := widget.NewLabel("Directories to search:")

	btns := container.NewGridWithColumns(2, appendBtn, deleteBtn)
	btnsDirEntry := container.NewGridWithRows(3, startingDirsLabel, btns, dirEntry)
	listPanel := container.NewBorder(btnsDirEntry, nil, nil, nil, dirList)

	// MAIN LAYOUT
	content := container.NewGridWithColumns(2, listPanel, form)

	// Hook up form submission to update "real" cfg
	form.OnSubmit = func() {
		slog.Info("Printing formStruct on Submit", "formStruct", formStruct)

		cfg.DatabasePath = formStruct.DatabasePath

		cfg.IgnoreStr = splitAndTrim(formStruct.IgnoreStr)
		cfg.IncludeStr = splitAndTrim(formStruct.IncludeStr)
		cfg.IgnoreExt = splitAndTrim(formStruct.IgnoreExt)
		cfg.IncludeExt = splitAndTrim(formStruct.IncludeExt)

		cfg.LogFilePath = formStruct.LogFilePath
		cfg.SaveSC = formStruct.SavedScreenshots
		cfg.AbsPath = formStruct.AbsPath
		cfg.FollowSymbolicLinks = formStruct.FollowSymbolic
		cfg.SkipSymbolicLinks = formStruct.SkipSymbolic
		cfg.SilentFFmpeg = formStruct.SilentFFmpeg
		cfg.FilesizeCutoff = formStruct.FilesizeCutoff
		// read out each directory from the binding
		length := startingDirs.Length()
		var dirs []string
		for i := 0; i < length; i++ {
			v, err := startingDirs.GetValue(i)
			if err == nil {
				dirs = append(dirs, v)
			}
		}
		cfg.StartingDirs = dirs
		config.ValidateStartingDirs(cfg)

		slog.Info("Updated real config.Config from UI", "cfg", cfg)
	}

	return content
}

func buildFilter(duplicatesListWidget *DuplicatesList, originalVideoData [][]*models.VideoData) (*fyne.Container, *widget.Check) {
	// FILTER
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Enter path/file search...")

	filterButton := widget.NewButton("Apply", func() {
		slog.Info("Filter applied", slog.String("filter", filterEntry.Text))

		query := parseSearchQuery(filterEntry.Text)

		filteredData := applyFilter(originalVideoData, query)

		duplicatesListWidget.SetData(filteredData)
	})

	filterForm := container.NewVBox(
		widget.NewLabel("Filter:"),
		filterEntry,
		filterButton,
	)

	filterForm.Hide()
	filterVisible := false

	checkWidget := widget.NewCheck("Show Filter Box", func(checked bool) {
		filterVisible = checked
		if filterVisible {
			filterForm.Show()
		} else {
			filterForm.Hide()
		}
		filterForm.Refresh()
	})
	return filterForm, checkWidget
}

// Helper function that splits comma-separated strings into slices
func splitAndTrim(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func newFormWithData(data binding.DataMap) *widget.Form {
	keys := data.Keys()
	items := make([]*widget.FormItem, len(keys))
	for i, k := range keys {
		data, err := data.GetItem(k)
		if err != nil {
			items[i] = widget.NewFormItem(k, widget.NewLabel(err.Error()))
		}
		items[i] = widget.NewFormItem(k, createBoundItem(data))
	}

	return widget.NewForm(items...)
}

func createBoundItem(v binding.DataItem) fyne.CanvasObject {
	switch val := v.(type) {
	case binding.Bool:
		return widget.NewCheckWithData("", val)
	case binding.Int:
		return widget.NewEntryWithData(binding.IntToString(val))
	case binding.String:
		return widget.NewEntryWithData(val)
	default:
		return widget.NewLabel("")
	}
}

func ConvertConfigToFormStruct(config *config.Config) formStruct {
	return formStruct{
		DatabasePath:     config.DatabasePath,
		LogFilePath:      config.LogFilePath,
		IgnoreStr:        strings.Join(config.IgnoreStr, ","),
		IncludeStr:       strings.Join(config.IncludeStr, ","),
		IgnoreExt:        strings.Join(config.IgnoreExt, ","),
		IncludeExt:       strings.Join(config.IncludeExt, ","),
		SavedScreenshots: config.SaveSC,
		AbsPath:          config.AbsPath,
		FollowSymbolic:   config.FollowSymbolicLinks,
		SkipSymbolic:     config.SkipSymbolicLinks,
		SilentFFmpeg:     config.SilentFFmpeg,
		FilesizeCutoff:   config.FilesizeCutoff,
	}
}
