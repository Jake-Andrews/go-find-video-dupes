package ui

import (
	"log/slog"
	"os"
	"strings"

	"govdupes/internal/config"
	"govdupes/internal/vm"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// temporary struct for binding config fields to Fyne form widgets.
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
	DetectionMethod  string
}

func buildHashType() fyne.CanvasObject {
	methodOptions := []string{"Slow", "Fast"}
	methodSelect := widget.NewSelect(methodOptions, nil)
	methodSelect.PlaceHolder = "Choose a hash method"

	// slow
	percentToSkip := 0.1
	data := binding.BindFloat(&percentToSkip)
	label := widget.NewLabel("% of start & end to skip: ")
	entry := widget.NewEntryWithData(binding.FloatToString(data))
	floats := container.NewGridWithColumns(2, label, entry)

	slide := widget.NewSliderWithData(0, 1, data)
	slide.Step = 0.01

	slowFramesEntry := widget.NewEntry()
	slowFramesEntry.SetPlaceHolder("Number of frames per second to hash (>=1)")

	slowSkipEntry := widget.NewEntry()
	slowSkipEntry.SetPlaceHolder("% of start/end to skip")

	slowContainer := container.NewVBox(
		widget.NewLabel("Slow Method Settings"),
		widget.NewForm(
			widget.NewFormItem("Number of Frames", slowFramesEntry),
			widget.NewFormItem("Skip (Start/End)", slowSkipEntry),
		),
	)

	// fast
	fastFPSEntry := widget.NewEntry()
	fastFPSEntry.SetPlaceHolder("FPS to hash")

	fastSkipEntry := widget.NewEntry()
	fastSkipEntry.SetPlaceHolder("% of start/end to skip")

	fastContainer := container.NewVBox(
		widget.NewLabel("Fast Method Settings"),
		widget.NewForm(
			widget.NewFormItem("FPS to Hash", fastFPSEntry),
			widget.NewFormItem("Skip (Start/End)", fastSkipEntry),
		),
	)

	// hide containers until the user selects a method
	slowContainer.Hide()
	fastContainer.Hide()

	methodSelect.OnChanged = func(method string) {
		switch method {
		case "Slow":
			slowContainer.Show()
			fastContainer.Hide()
		case "Fast":
			slowContainer.Hide()
			fastContainer.Show()
		}
	}

	return container.NewVBox(
		widget.NewLabel("Hash Type"),
		methodSelect,
		slowContainer,
		fastContainer,
	)
}

func buildCompareMethod() fyne.CanvasObject {
	// Options for compare method
	compareOptions := []string{"Duplicate Detection", "LCS"}
	compareSelect := widget.NewSelect(compareOptions, nil)
	compareSelect.PlaceHolder = "Choose a compare method"

	// Duplicate Detection fields
	ddFramesMatchEntry := widget.NewEntry()
	ddFramesMatchEntry.SetPlaceHolder("Number of frames that need to match")

	hammingDistanceEntry := widget.NewEntry()
	hammingDistanceEntry.SetPlaceHolder("Hamming distance threshold")

	duplicateDetectionContainer := container.NewVBox(
		widget.NewLabel("(Slow) Standard Detection Settings"),
		widget.NewForm(
			widget.NewFormItem("Frames to Match", ddFramesMatchEntry),
			widget.NewFormItem("Hamming Distance", hammingDistanceEntry),
		),
	)

	// LCS fields
	lcsWindowSizeEntry := widget.NewEntry()
	lcsWindowSizeEntry.SetPlaceHolder("Sliding window size")

	lcsHammingDistanceEntry := widget.NewEntry()
	lcsHammingDistanceEntry.SetPlaceHolder("Hamming distance threshold")

	lcsLengthNeededEntry := widget.NewEntry()
	lcsLengthNeededEntry.SetPlaceHolder("LCS length needed for a match")

	partialFullMatchEntry := widget.NewEntry()
	partialFullMatchEntry.SetPlaceHolder("Partial/Full match thresholds")

	lcsContainer := container.NewVBox(
		widget.NewLabel("(Slow) LCS-Based Detection Settings"),
		widget.NewForm(
			widget.NewFormItem("Sliding Window Size", lcsWindowSizeEntry),
			widget.NewFormItem("Hamming Dist.", lcsHammingDistanceEntry),
			widget.NewFormItem("LCS Length", lcsLengthNeededEntry),
			widget.NewFormItem("Match Thresholds", partialFullMatchEntry),
		),
	)

	duplicateDetectionContainer.Hide()
	lcsContainer.Hide()

	compareSelect.OnChanged = func(method string) {
		switch method {
		case "Duplicate Detection":
			duplicateDetectionContainer.Show()
			lcsContainer.Hide()
		case "LCS":
			duplicateDetectionContainer.Hide()
			lcsContainer.Show()
		}
	}

	return container.NewVBox(
		widget.NewLabel("Compare Method"),
		compareSelect,
		duplicateDetectionContainer,
		lcsContainer,
	)
}

// Although were using data binding the Config is not updated until the
// form.OnSubmit is called. This is because some fields in the config struct
// need to be converted (string in UI -> []string in config).
// **Should fix this later**
func buildConfigTab(cfg *config.Config, w fyne.Window, checkWidget *widget.Check, myViewModel vm.ViewModel) fyne.CanvasObject {
	formStruct := ConvertConfigToFormStruct(cfg)
	formData := binding.BindStruct(&formStruct)
	form := newFormWithData(formData)

	// append here because it adopts the form "look", looks out of place otherwise
	form.Append("check", checkWidget)

	// directories to search
	dirStr := binding.NewString()
	dirEntry := widget.NewEntryWithData(dirStr)

	startingValues := cfg.StartingDirs
	if len(startingValues) == 0 {
		startingValues = []string{"./"}
	}
	startingDirs := binding.BindStringList(&startingValues)

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

	dirList := widget.NewListWithData(
		startingDirs,
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil, nil, widget.NewLabel(""))
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			f := item.(binding.String)
			lbl := obj.(*fyne.Container).Objects[0].(*widget.Label)
			lbl.Bind(f)
		},
	)

	// JSON label + entry + button
	jsonLabel := widget.NewLabel("Export to json")
	jsonPathEntry := widget.NewEntry()
	cwd, _ := os.Getwd()
	cwd += "/duplicateVideosJSON.json"
	jsonPathEntry.PlaceHolder = cwd
	jsonPathEntry.Text = cwd

	jsonButton := widget.NewButton("Export", func() {
		exportPath := jsonPathEntry.Text
		if exportPath == "" {
			slog.Info("JSON export path is empty, defaulting to cwd", "cwd", cwd)
			exportPath = cwd
		}
		slog.Info("Exporting duplicates to JSON", "exportPath", exportPath)

		err := myViewModel.ExportToJSON(exportPath)
		if err != nil {
			slog.Error("Failed to export duplicates to JSON", "error", err)
		}
	})

	startingDirsLabel := widget.NewLabel("Directories to search:")
	btnsDirEntry := container.NewGridWithRows(
		7,
		jsonLabel,
		jsonPathEntry,
		jsonButton,
		startingDirsLabel,
		container.NewGridWithColumns(2, appendBtn, deleteBtn),
		dirEntry,
	)
	listPanel := container.NewBorder(btnsDirEntry, nil, nil, nil, dirList)

	hashType := buildHashType()
	compareMethod := buildCompareMethod()

	rightSide := container.NewVBox(
		form,
		widget.NewSeparator(),
		hashType,
		widget.NewSeparator(),
		compareMethod,
	)

	// places the directory list and the config form side by side
	content := container.NewGridWithColumns(2, listPanel, rightSide)

	// copy formStruct fields back into cfg
	form.OnSubmit = func() {
		slog.Info("Printing formStruct on Submit", "formStruct", formStruct)

		cfg.DatabasePath = formStruct.DatabasePath
		cfg.LogFilePath = formStruct.LogFilePath
		cfg.IgnoreStr = splitAndTrim(formStruct.IgnoreStr)
		cfg.IncludeStr = splitAndTrim(formStruct.IncludeStr)
		cfg.IgnoreExt = splitAndTrim(formStruct.IgnoreExt)
		cfg.IncludeExt = splitAndTrim(formStruct.IncludeExt)
		cfg.SaveSC = formStruct.SavedScreenshots
		cfg.AbsPath = formStruct.AbsPath
		cfg.FollowSymbolicLinks = formStruct.FollowSymbolic
		cfg.SkipSymbolicLinks = formStruct.SkipSymbolic
		cfg.SilentFFmpeg = formStruct.SilentFFmpeg
		cfg.FilesizeCutoff = formStruct.FilesizeCutoff
		cfg.DetectionMethod = formStruct.DetectionMethod

		// read out each directory from the binding
		length := startingDirs.Length()
		var dirs []string
		for i := range length {
			v, err := startingDirs.GetValue(i)
			if err == nil {
				dirs = append(dirs, v)
			}
		}
		cfg.StartingDirs = dirs

		err := config.ValidateStartingDirs(cfg)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		slog.Info("Updated real config.Config from UI", "cfg", cfg)
	}

	return content
}

// copies config fields into a formStruct for binding
func ConvertConfigToFormStruct(cfg *config.Config) formStruct {
	return formStruct{
		DatabasePath:     cfg.DatabasePath,
		LogFilePath:      cfg.LogFilePath,
		IgnoreStr:        strings.Join(cfg.IgnoreStr, ","),
		IncludeStr:       strings.Join(cfg.IncludeStr, ","),
		IgnoreExt:        strings.Join(cfg.IgnoreExt, ","),
		IncludeExt:       strings.Join(cfg.IncludeExt, ","),
		SavedScreenshots: cfg.SaveSC,
		AbsPath:          cfg.AbsPath,
		FollowSymbolic:   cfg.FollowSymbolicLinks,
		SkipSymbolic:     cfg.SkipSymbolicLinks,
		SilentFFmpeg:     cfg.SilentFFmpeg,
		FilesizeCutoff:   cfg.FilesizeCutoff,
		DetectionMethod:  cfg.DetectionMethod,
	}
}

// Both funcs below used together to create forms easily
// ____________________________________________________

// newFormWithData creates a fyne Form from a binding.DataMap.
func newFormWithData(data binding.DataMap) *widget.Form {
	keys := data.Keys()
	items := make([]*widget.FormItem, len(keys))
	for i, k := range keys {
		sub, err := data.GetItem(k)
		if err != nil {
			items[i] = widget.NewFormItem(k, widget.NewLabel(err.Error()))
			continue
		}
		if k == "DetectionMethod" {
			items[i] = widget.NewFormItem(k, createDetectionSelect(sub))
		} else {
			items[i] = widget.NewFormItem(k, createBoundItem(sub))
		}
	}

	return widget.NewForm(items...)
}

// createBoundItem creates the correct widget for the given DataItem (bool, int, string)
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

func createDetectionSelect(data binding.DataItem) fyne.CanvasObject {
	strBinding, ok := data.(binding.String)
	if !ok {
		return widget.NewLabel("Invalid binding")
	}

	selectWidget := widget.NewSelect([]string{"SlowPhash", "FastPhash"}, func(selected string) {
		strBinding.Set(selected)
	})

	// Bind initial value
	current, err := strBinding.Get()
	if err == nil {
		selectWidget.SetSelected(current)
	}

	return selectWidget
}

// splitAndTrim splits a string by commas, then trims each piece.
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
