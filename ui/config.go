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
// doesn't contain all of config.Config
// used for convience to simplify creating UI elements
// complex UI fields are not included here

type formStruct struct {
	DatabasePath        string
	LogFilePath         string
	IgnoreStr           string // []string
	IncludeStr          string // []string
	IgnoreExt           string // []string
	IncludeExt          string // []string
	FilesizeCutoff      int64
	SaveSC              bool
	AbsPath             bool
	FollowSymbolicLinks bool
	SkipSymbolicLinks   bool
	SilentFFmpeg        bool
}

// also correspond to config.Config fields
// seperate UI section

type samplerComparerForm struct {
	SamplerType binding.String

	SlowSkipPercent binding.Float
	SlowFPS         binding.Float

	PairHammingDistance binding.Int

	FastSkipPercent binding.Float

	LCSHammingDistance binding.Int
}

func newSamplerComparerForm(cfg *config.Config) *samplerComparerForm {
	f := &samplerComparerForm{
		SamplerType:         binding.NewString(),
		SlowSkipPercent:     binding.NewFloat(),
		SlowFPS:             binding.NewFloat(),
		PairHammingDistance: binding.NewInt(),
		FastSkipPercent:     binding.NewFloat(),
		LCSHammingDistance:  binding.NewInt(),
	}

	_ = f.SamplerType.Set("slow")

	_ = f.SlowSkipPercent.Set(cfg.Slow.SkipPercent)
	_ = f.SlowFPS.Set(cfg.Slow.FPS)

	_ = f.PairHammingDistance.Set(cfg.Pair.HammingDistance)

	_ = f.FastSkipPercent.Set(cfg.Fast.SkipPercent)

	_ = f.LCSHammingDistance.Set(cfg.LCS.HammingDistance)

	return f
}

func buildSamplerAndComparerUI(cfg *config.Config) (fyne.CanvasObject, *samplerComparerForm) {
	// create separate binding struct
	scForm := newSamplerComparerForm(cfg)

	samplerOptions := []string{"slow", "fast"}
	samplerSelect := widget.NewSelect(samplerOptions, nil)
	samplerSelect.PlaceHolder = "Choose Sampler & Comparer"

	// WithData doesn't exist for select
	samplerSelect.OnChanged = func(selected string) {
		_ = scForm.SamplerType.Set(selected)
	}
	// set the initial selection
	currentSamplerType, _ := scForm.SamplerType.Get()
	samplerSelect.SetSelected(currentSamplerType)

	// -- slow sampler --
	slowSkipSlider := widget.NewSliderWithData(0, 40, scForm.SlowSkipPercent)
	slowSkipSlider.Step = 1
	slowSkipLabel := widget.NewLabelWithData(
		binding.FloatToString(scForm.SlowSkipPercent),
	)

	slowFPS := widget.NewEntryWithData(binding.FloatToString(scForm.SlowFPS))
	slowFPS.SetPlaceHolder("FPS (>1)")

	slowSamplerBox := container.NewVBox(
		widget.NewLabel("Slow Sampler"),
		widget.NewForm(
			widget.NewFormItem(
				"Skip Percent (0–40)",
				container.NewBorder(nil, nil, nil, slowSkipLabel, slowSkipSlider),
			),
			widget.NewFormItem("FPS (>1)", slowFPS),
		),
	)

	// -- Pair comparer --
	pairHammingSlider := widget.NewSliderWithData(0, 16, binding.IntToFloat(scForm.PairHammingDistance))
	pairHammingSlider.Step = 1
	pairHammingLabel := widget.NewLabelWithData(binding.IntToString(scForm.PairHammingDistance))

	bottomPadding := widget.NewLabel("")

	slowComparerBox := container.NewBorder(nil, bottomPadding, nil, nil,
		container.NewVBox(
			widget.NewLabel("Pair Comparer"),
			widget.NewForm(
				widget.NewFormItem(
					"Hamming Distance (0–16)",
					container.NewBorder(nil, nil, nil, pairHammingLabel, pairHammingSlider),
				),
			),
		),
	)

	slowContainer := container.NewVBox(slowSamplerBox, slowComparerBox)

	// -- Fast sampler --
	fastFramesLabel := widget.NewLabel("4 (fixed)") // not editable
	fastSkipSlider := widget.NewSliderWithData(0, 40, scForm.FastSkipPercent)
	fastSkipSlider.Step = 1
	fastSkipLabel := widget.NewLabelWithData(binding.FloatToString(scForm.FastSkipPercent))

	fastSamplerBox := container.NewVBox(
		widget.NewLabel("Fast Sampler"),
		widget.NewForm(
			widget.NewFormItem("Frames", fastFramesLabel),
			widget.NewFormItem(
				"Skip Percent (0–40)",
				container.NewBorder(nil, nil, nil, fastSkipLabel, fastSkipSlider),
			),
		),
	)

	// -- LCS comparer --
	lcsHamSlider := widget.NewSliderWithData(0, 16, binding.IntToFloat(scForm.LCSHammingDistance))
	lcsHamSlider.Step = 1
	lcsHamLabel := widget.NewLabelWithData(binding.IntToString(scForm.LCSHammingDistance))
	lcsSlidingWindowLabel := widget.NewLabel("5 (fixed)") // not editable

	fastComparerBox := container.NewVBox(
		widget.NewLabel("LCS Comparer"),
		widget.NewForm(
			widget.NewFormItem(
				"Hamming Distance (0–16)",
				container.NewBorder(nil, nil, nil, lcsHamLabel, lcsHamSlider),
			),
			widget.NewFormItem("Sliding Window", lcsSlidingWindowLabel),
		),
	)

	fastContainer := container.NewVBox(fastSamplerBox, fastComparerBox)

	// show/hide slow vs fast based on scForm.SamplerType
	if currentSamplerType == "slow" {
		slowContainer.Show()
		fastContainer.Hide()
	} else {
		slowContainer.Hide()
		fastContainer.Show()
	}

	samplerSelect.OnChanged = func(method string) {
		_ = scForm.SamplerType.Set(method)
		if method == "slow" {
			slowContainer.Show()
			fastContainer.Hide()
		} else {
			slowContainer.Hide()
			fastContainer.Show()
		}
	}

	combined := container.NewVBox(
		samplerSelect,
		slowContainer,
		fastContainer,
	)

	return combined, scForm
}

func buildConfigTab(cfg *config.Config, w fyne.Window, checkWidget *widget.Check, myViewModel vm.ViewModel) fyne.CanvasObject {
	formStruct := ConvertConfigToFormStruct(cfg)
	formData := binding.BindStruct(&formStruct)
	form := newFormWithData(formData)

	// append the check widget to the form
	form.Append("check", checkWidget)

	// --- Directory Management Section ---
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

	// Make the directory list larger and scrollable
	dirList := widget.NewListWithData(
		startingDirs,
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(item binding.DataItem, obj fyne.CanvasObject) {
			f := item.(binding.String)
			lbl := obj.(*widget.Label)
			lbl.Bind(f)
		},
	)

	// Ensure the scroll container takes up space and grows within its parent
	scroll := container.NewScroll(dirList)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	// Ensure other UI components remain close to the directory list
	startingDirsLabel := widget.NewLabel("Directories to Search:")
	dirControls := container.NewGridWithColumns(2, appendBtn, deleteBtn)

	dirManagement := container.NewVBox(
		startingDirsLabel,
		container.NewVBox(dirEntry, dirControls),
		scroll,
	)

	// --- Export JSON Section ---
	jsonLabel := widget.NewLabel("Export to JSON")
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

	exportSection := container.NewVBox(
		jsonLabel,
		jsonPathEntry,
		jsonButton,
	)

	// --- Sampler & Comparer Section ---
	samplerAndComparer, scForm := buildSamplerAndComparerUI(cfg)

	samplerHeading := widget.NewLabelWithStyle(
		"Sampler & Comparer",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	leftSide := container.NewVBox(
		dirManagement,
		widget.NewSeparator(),
		exportSection,
		widget.NewSeparator(),
		samplerHeading,
		samplerAndComparer,
	)

	// --- Form Section ---
	rightSide := container.NewVBox(
		form,
	)

	// --- Combine Left and Right using a Responsive Split ---
	content := container.NewHSplit(leftSide, rightSide)
	content.SetOffset(0.4) // Adjust this to give the left side 40% of the width

	// --- Form Submission ---
	form.OnSubmit = func() {
		slog.Info("Printing formStruct on Submit", "formStruct", formStruct)

		// -- Copy formStruct values to cfg --
		cfg.DatabasePath = formStruct.DatabasePath
		cfg.LogFilePath = formStruct.LogFilePath
		cfg.IgnoreStr = splitAndTrim(formStruct.IgnoreStr)
		cfg.IncludeStr = splitAndTrim(formStruct.IncludeStr)
		cfg.IgnoreExt = splitAndTrim(formStruct.IgnoreExt)
		cfg.IncludeExt = splitAndTrim(formStruct.IncludeExt)
		cfg.SaveSC = formStruct.SaveSC
		cfg.AbsPath = formStruct.AbsPath
		cfg.FollowSymbolicLinks = formStruct.FollowSymbolicLinks
		cfg.SkipSymbolicLinks = formStruct.SkipSymbolicLinks
		cfg.SilentFFmpeg = formStruct.SilentFFmpeg
		cfg.FilesizeCutoff = formStruct.FilesizeCutoff

		// -- Copy starting directories --
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

		// -- Copy sampler and comparer data --
		samplerType, _ := scForm.SamplerType.Get()
		switch samplerType {
		case "slow":
			cfg.SamplerType = config.SlowSampler
		case "fast":
			cfg.SamplerType = config.FastSampler
		default:
			slog.Error("unknown samplerType on form submit", "type", samplerType)
		}

		slog.Info("Updated real config.Config from UI", "cfg", cfg)
	}

	return content
}

// copies config fields into a formStruct for binding
func ConvertConfigToFormStruct(cfg *config.Config) formStruct {
	return formStruct{
		DatabasePath:        cfg.DatabasePath,
		LogFilePath:         cfg.LogFilePath,
		IgnoreStr:           strings.Join(cfg.IgnoreStr, ","),
		IncludeStr:          strings.Join(cfg.IncludeStr, ","),
		IgnoreExt:           strings.Join(cfg.IgnoreExt, ","),
		IncludeExt:          strings.Join(cfg.IncludeExt, ","),
		SaveSC:              cfg.SaveSC,
		AbsPath:             cfg.AbsPath,
		FollowSymbolicLinks: cfg.FollowSymbolicLinks,
		SkipSymbolicLinks:   cfg.SkipSymbolicLinks,
		SilentFFmpeg:        cfg.SilentFFmpeg,
		FilesizeCutoff:      cfg.FilesizeCutoff,
	}
}

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
		items[i] = widget.NewFormItem(k, createBoundItem(sub))
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
