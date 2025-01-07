package ui

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"govdupes/internal/config"
	"govdupes/internal/models"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func buildConfigTab(duplicatesListWidget *DuplicatesList, originalVideoData [][]*models.VideoData) (fyne.CanvasObject, *fyne.Container, config.Config) {
	// FILTER
	filterEntry := widget.NewEntry()
	filterEntry.SetPlaceHolder("Enter path/file search...")

	filterButton := widget.NewButton("Apply", func() {
		slog.Info("Filter applied", slog.String("filter", filterEntry.Text))

		// Parse the user filter string into a search query
		query := parseSearchQuery(filterEntry.Text)

		// Filter the data based on the query
		filteredData := applyFilter(originalVideoData, query)

		// Rebind to the duplicates list
		duplicatesListWidget.SetData(filteredData)
	})

	// Create a form layout for the filter box
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
		DatabasePath, LogFilePath, IgnoreStr, IncludeStr, IgnoreExt, IncludeExt         string
		SavedScreenshots, AbsPath, FollowSymbolicLinks, SkipSymbolicLinks, SilentFFmpeg bool
		FilesizeCutoff                                                                  int64
	}{
		DatabasePath:        "./videos.db",
		LogFilePath:         "app.log",
		IgnoreStr:           "",
		IncludeStr:          "",
		IgnoreExt:           "",
		IncludeExt:          "mp4,m4a,webm",
		SavedScreenshots:    true,
		AbsPath:             true,
		FollowSymbolicLinks: true,
		SkipSymbolicLinks:   true,
		SilentFFmpeg:        true,
		FilesizeCutoff:      0,
	}

	formData := binding.BindStruct(&formStruct)
	form := newFormWithData(formData)
	form.OnSubmit = func() {
		slog.Info("Printing form", "formStruct:", formStruct)
	}
	form.Append("check", checkWidget)
	// List forms
	dirStr := binding.NewString()
	dirEntry := widget.NewEntryWithData(dirStr)

	startingDirs := binding.BindStringList(&[]string{"./"})
	appendBtn := widget.NewButton("Append", func() {
		startingDirs.Append(dirEntry.Text)
	})
	deleteBtn := widget.NewButton("Delete", func() {
		l := startingDirs.Length() - 1
		str, err := startingDirs.GetValue(l)
		if err != nil {
			slog.Warn("failed getting value from startingDirs list", slog.Any("Error", err))
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

	// ignorestr includestr ignoreext includeext
	/*
		ignStrBind := bindingStringErr("")
		inclStrBind := bindingStringErr("")
		ignExtBind := bindingStringErr("")
		inclExtBind := bindingStringErr("")

		ignStr := widget.NewEntryWithData(ignStrBind)
		inclStr := widget.NewEntryWithData(inclStrBind)
		ignExt := widget.NewEntryWithData(ignExtBind)
		inclExt := widget.NewEntryWithData(inclExtBind)

		ignStrLabel := widget.NewLabel("Ignore String:")
		inclStrLabel := widget.NewLabel("Include String:")
		ignExtLabel := widget.NewLabel("Ignore Ext:")
		inclExtLabel := widget.NewLabel("Include Ext:")
	*/

	content := container.NewGridWithColumns(2, listPanel, form)

	return content, filterForm, config.Config{}
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

type StringSlice struct {
	Values      []string
	wipeDefault bool // Track whether the default value is currently in use
}

func (s *StringSlice) String() string {
	return strings.Join(s.Values, ", ")
}

func (s *StringSlice) Set(value string) error {
	if s.wipeDefault {
		s.Values = nil
		s.wipeDefault = false
	}
	values := strings.Split(value, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			s.Values = append(s.Values, v)
		}
	}
	return nil
}

type Config struct {
	DatabasePath        StringSlice
	StartingDirs        StringSlice
	IgnoreStr           StringSlice
	IncludeStr          StringSlice
	IgnoreExt           StringSlice
	IncludeExt          StringSlice
	FilesizeCutoff      int64 // in bytes
	SaveSC              bool
	AbsPath             bool
	FollowSymbolicLinks bool
	SkipSymbolicLinks   bool
	SilentFFmpeg        bool
	LogFilePath         string
}

func (c *Config) ParseArgs() {
	c.DatabasePath = StringSlice{Values: []string{"./videos.db"}, wipeDefault: true}
	c.StartingDirs = StringSlice{Values: []string{"."}, wipeDefault: true}
	c.IgnoreStr = StringSlice{}
	c.IncludeStr = StringSlice{}
	c.IgnoreExt = StringSlice{}
	c.IncludeExt = StringSlice{Values: []string{"mp4", "m4a", "webm"}, wipeDefault: false}
	c.SaveSC = false
	c.AbsPath = true
	c.FollowSymbolicLinks = false
	c.SkipSymbolicLinks = true

	flag.Var(&c.DatabasePath, "dp", `Specify database path (default "./videos.db").`)
	flag.Var(&c.StartingDirs, "sd", `Directory path(s) to search (default "."), multiple allowed.`)
	flag.Var(&c.IgnoreStr, "igs", "String(s) to ignore, multiple allowed.")
	flag.Var(&c.IncludeStr, "is", "String(s) to include, multiple allowed.")
	flag.Var(&c.IgnoreExt, "ige", "Extension(s) to ignore, multiple allowed.")
	flag.Var(&c.IncludeExt, "ie", "Extension(s) to include, multiple allowed.")

	fileSizeGiB := flag.Float64("fs", 0, "Max file size in GiB (default 0).")
	c.SaveSC = *flag.Bool("sc", true, "Flag to save screenshots, T/F (default False).")
	c.SilentFFmpeg = *flag.Bool("sf", true, "Flag that determines if FFmpeg is silent or not, T/F (default True).")
	c.FollowSymbolicLinks = *flag.Bool("fsl", true, "Follow symbolic links, T/F (default False).")
	c.LogFilePath = *flag.String("log", "app.log", "Path to log file (default = app.log).")

	flag.Parse()

	c.FilesizeCutoff = int64(*fileSizeGiB * 1024 * 1024 * 1024)
	validateStartingDirs(c)
}

// validateStartingDirs ensures starting directories exist and are actually dirs.
func validateStartingDirs(c *Config) {
	for i, dir := range c.StartingDirs.Values {
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
		c.StartingDirs.Values[i] = abs

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
