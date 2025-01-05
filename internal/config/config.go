package config

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

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
			continue // or os.Exit(1) if you want to fail immediately
		}

		abs, err := filepath.Abs(dir)
		if err != nil {
			slog.Error("Error getting the absolute path for dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			continue // or os.Exit(1)
		}
		c.StartingDirs.Values[i] = abs

		fsInfo, err := f.Stat()
		if errors.Is(err, os.ErrNotExist) {
			slog.Error("Directory does not exist",
				slog.String("dir", dir),
				slog.Any("error", err))
			continue
		} else if err != nil {
			slog.Error("Error calling stat on dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			continue
		}
		if !fsInfo.IsDir() {
			slog.Error("Path is not a valid directory", slog.String("dir", dir))
		}
	}
}

// SetupLogger creates a slog Logger that writes either to a file (JSON) or stdout (text).
// SetupLogger creates a slog Logger that writes to both a file and stdout/stderr.
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

