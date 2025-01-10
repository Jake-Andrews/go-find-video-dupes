package config

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// modify ConvertConfigToFormStruct / config UI as well
type Config struct {
	DatabasePath        string
	LogFilePath         string
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
}

// "3gp", "3g2", "mpeg", "mpg", "ts", "m2ts", "mts", "vob", "rm", "rmvb", "asf", "ogv", "ogm", "mxf", "divx", "dv", "xvid", "f4v"
func (c *Config) SetDefaults() {
	slog.Info("Setting default config options")
	c.StartingDirs = []string{"."}
	c.DatabasePath = "./videos.db"
	c.LogFilePath = "app.log"
	c.IgnoreStr = []string{}
	c.IncludeStr = []string{}
	c.IgnoreExt = []string{}
	c.IncludeExt = []string{
		"mp4", "m4a", "m4v", "webm", "mkv", "mov", "avi",
		"wmv", "flv",
	}
	c.SaveSC = true
	c.AbsPath = true
	c.FollowSymbolicLinks = true
	c.SkipSymbolicLinks = true
	c.SilentFFmpeg = true
	c.FilesizeCutoff = 0
	ValidateStartingDirs(c)
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

/*
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
	ValidateStartingDirs(c)
}
*/
