package ui

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func buildConfigTab(
	duplicatesListWidget *DuplicatesList,
	originalVideoData [][]*models.VideoData,
	cfg *Config,
) (fyne.CanvasObject, *fyne.Container) {
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

	// REST OF CONFIG (binding settings to widgets)
	formStruct := struct {
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
	}{
		DatabasePath:     "./videos.db",
		LogFilePath:      "app.log",
		IgnoreStr:        "",
		IncludeStr:       "",
		IgnoreExt:        "",
		IncludeExt:       "mp4,m4a,webm",
		SavedScreenshots: true,
		AbsPath:          true,
		FollowSymbolic:   true,
		SkipSymbolic:     true,
		SilentFFmpeg:     true,
		FilesizeCutoff:   0,
	}

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
		ValidateStartingDirs(cfg)

		slog.Info("Updated real config.Config from UI", "cfg", cfg)
	}

	return content, filterForm
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

type Config struct {
	DatabasePath        string
	StartingDirs        []string
	IgnoreStr           []string
	IncludeStr          []string
	IgnoreExt           []string
	IncludeExt          []string
	FilesizeCutoff      int64 // in bytes
	SaveSC              bool
	AbsPath             bool
	FollowSymbolicLinks bool
	SkipSymbolicLinks   bool
	SilentFFmpeg        bool
	LogFilePath         string
}

// validateStartingDirs ensures starting directories exist and are actually dirs.
func ValidateStartingDirs(c *Config) {
	for i, dir := range c.StartingDirs {
		f, err := os.Open(dir)
		if err != nil {
			slog.Error("Error opening dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			os.Exit(1)
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			slog.Error("Error getting the absolute path for dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			os.Exit(1)
		}
		c.StartingDirs[i] = abs

		fsInfo, err := f.Stat()
		if errors.Is(err, os.ErrNotExist) {
			slog.Error("Directory does not exist",
				slog.String("dir", dir),
				slog.Any("error", err))
			os.Exit(1)
		} else if err != nil {
			slog.Error("Error calling stat on dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			os.Exit(1)
		}
		if !fsInfo.IsDir() {
			slog.Error("Path is not a valid directory", slog.String("dir", dir))
			os.Exit(1)
		}
	}
}

func SetupLogger(logFilePath string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if logFilePath != "" {
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			slog.Error("Failed to open log file",
				slog.String("path", logFilePath),
				slog.Any("error", err))
			os.Exit(1)
		}
		writers = append(writers, file)
	}

	multiWriter := io.MultiWriter(writers...)

	return slog.New(slog.NewJSONHandler(multiWriter, opts))
}

func bindingStringErr(s string) binding.String {
	str := binding.NewString()
	if err := str.Set(s); err != nil {
		slog.Warn("binding.NewString()", slog.Any("err", err))
	}
	return str
}

func (c *Config) ParseArgs() {
	c.StartingDirs = []string{"."}
	c.DatabasePath = "./videos.db"
	c.LogFilePath = "app.log"
	c.IgnoreStr = []string{}
	c.IncludeStr = []string{}
	c.IgnoreExt = []string{}
	c.IncludeExt = []string{"mp4", "m4a", "webm"}
	c.SaveSC = true
	c.AbsPath = true
	c.FollowSymbolicLinks = true
	c.SkipSymbolicLinks = true
	c.SilentFFmpeg = true
	c.FilesizeCutoff = 0
	ValidateStartingDirs(c)
}
