package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"govdupes/internal/comparer"
	"govdupes/internal/comparer/compare"
	"govdupes/internal/hasher"
	"govdupes/internal/hasher/hash"
	"govdupes/internal/sampler"
	"govdupes/internal/sampler/sample"
)

// changes here also have to be done to ConvertConfigToFormStruct / config UI
// **go back and properly implement SaveSC/and others**
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
	// ----
	SamplerType SamplerType
	Slow        sample.SlowSampler
	Fast        sample.FastSampler
	//
	HasherType HasherType
	Phash      hash.Phasher
	//
	CompareType ComparerType
	Pair        compare.PairComparer
	LCS         compare.LCSComparer
}

type SamplerType int

const (
	SlowSampler SamplerType = iota
	FastSampler
)

func BuildSamplerFromConfig(cfg Config) sampler.Sampler {
	switch cfg.SamplerType {
	case SlowSampler:
		return &cfg.Slow
	case FastSampler:
		return &cfg.Fast
	default:
		return nil
	}
}

type HasherType int

const (
	Phash HasherType = iota
)

func BuildHasherFromConfig(cfg Config) hasher.Hasher {
	switch cfg.HasherType {
	case Phash:
		return &cfg.Phash
	default:
		return nil
	}
}

type ComparerType int

const (
	Pair ComparerType = iota
	LCS
)

func BuildComparerFromConfig(cfg Config) comparer.Comparer {
	switch cfg.CompareType {
	case Pair:
		return &cfg.Pair
	case LCS:
		return &cfg.LCS
	default:
		return nil
	}
}

// "3gp", "3g2", "mpeg", "mpg", "ts", "m2ts", "mts", "vob", "rm", "rmvb", "asf", "ogv", "ogm", "mxf", "divx", "dv", "xvid", "f4v"
func (c *Config) SetDefaults() {
	slog.Info("Setting default config options")
	c.DatabasePath = "./videos.db"
	c.LogFilePath = "app.log"
	c.StartingDirs = []string{"."}
	c.IgnoreStr = []string{}
	c.IncludeStr = []string{}
	c.IgnoreExt = []string{}
	c.IncludeExt = []string{
		"mp4", "m4a", "m4v", "webm", "mkv", "mov", "avi",
		"wmv", "flv",
	}
	c.FilesizeCutoff = 0
	c.SaveSC = true
	c.AbsPath = true
	c.FollowSymbolicLinks = true
	c.SkipSymbolicLinks = true
	c.SilentFFmpeg = true
	ValidateStartingDirs(c) // on the random chance that "." is fucked

	// -- --

	c.SamplerType = FastSampler
	c.Slow = sample.SlowSampler{SkipPercent: 0.1, FPS: 1.0}
	c.Fast = sample.FastSampler{SkipPercent: 0.1, Frames: 4}
	// -- --
	c.HasherType = Phash
	c.Phash = hash.Phasher{}
	// -- --
	c.CompareType = Pair
	c.Pair = compare.PairComparer{}
	c.LCS = compare.LCSComparer{}
}

func ValidateStartingDirs(c *Config) error {
	for i, dir := range c.StartingDirs {
		f, err := os.Open(dir)
		if err != nil {
			slog.Error("Error opening dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			return fmt.Errorf("failed to open directory %s: %w", dir, err)
		}
		defer f.Close()

		abs, err := filepath.Abs(dir)
		if err != nil {
			slog.Error("Error getting the absolute path for dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			return fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
		}
		c.StartingDirs[i] = abs

		fsInfo, err := f.Stat()
		if errors.Is(err, os.ErrNotExist) {
			slog.Error("Directory does not exist",
				slog.String("dir", dir),
				slog.Any("error", err))
			return fmt.Errorf("directory does not exist: %s", dir)
		} else if err != nil {
			slog.Error("Error calling stat on dir",
				slog.String("dir", dir),
				slog.Any("error", err))
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
		if !fsInfo.IsDir() {
			slog.Error("Path is not a valid directory", slog.String("dir", dir))
			return fmt.Errorf("path is not a valid directory: %s", dir)
		}
	}
	return nil
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
