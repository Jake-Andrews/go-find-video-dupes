package filesystem

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"govdupes/internal/config"
	"govdupes/internal/models"
)

func SearchDirs(c *config.Config) []*models.Video {
	slog.Info("Searching directories")
	videos := make([]*models.Video, 0)

	for _, dir := range c.StartingDirs.Values {
		dir = filepath.Clean(strings.TrimSuffix(dir, "/"))
		slog.Info("Searching recursively", slog.String("starting_dir", dir))
		info, err := os.Stat(dir)
		if err != nil {
			slog.Error("Error accessing directory", slog.String("dir", dir), slog.Any("error", err))
			continue
		}
		if !info.IsDir() {
			slog.Warn("Skipping because it's not a directory", slog.String("dir", dir))
			continue
		}
		fileSystem := os.DirFS(dir)
		videos = append(videos, getVideosFromFS(fileSystem, c, dir)...)
	}

	if len(videos) == 0 {
		slog.Error("No files were found! Exiting.")
		os.Exit(1)
	}
	return videos
}

// check if file ext is in ignoreext, if so ignore
// check if file ext is in includeext, if so consider the file
// check if file name is in ignorestr, if so ignore
// check if filename is in includestr, if so include consider the file
// if both includeext/includestr agree then include the file
func getVideosFromFS(fileSystem fs.FS, c *config.Config, root string) []*models.Video {
	slog.Info("Processing root directory", slog.String("root", root))
	videos := make([]*models.Video, 0)
	fileTracker := NewFileTracker()

	walkDirErr := fs.WalkDir(
		fileSystem,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				slog.Error("Error walking through filesystem", slog.Any("error", err))
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !validExt(path, c) || !validFileName(d, c) {
				return nil
			}

			path = filepath.Join(root, path)

			fileInfo, err := d.Info()
			if err != nil {
				slog.Error("Error getting file info", slog.String("path", path), slog.Any("error", err))
				return nil
			}

			if fileInfo.Size() < c.FilesizeCutoff {
				slog.Info("Skipping file due to size cutoff",
					slog.String("path", path),
					slog.Int64("size", fileInfo.Size()),
					slog.Int64("cutoff", c.FilesizeCutoff))
				return nil
			}

			fileID, err := fileTracker.FindFileLinks(path, *c)
			if err != nil {
				slog.Error("Error detecting symbolic/hard link", slog.String("path", path), slog.Any("error", err))
				return nil
			}
			if fileID.IsSymbolicLink && c.SkipSymbolicLinks {
				slog.Info("Skipping symbolic link", slog.String("path", path))
				return nil
			}

			if !checkValidVideo(path, fileInfo) {
				slog.Warn("Invalid video file", slog.String("path", path))
				return nil
			}

			video := createVideo(path, fileInfo, *fileID)
			videos = append(videos, &video)
			return nil
		},
	)

	if walkDirErr != nil {
		slog.Error("Error walking through directories", slog.Any("error", walkDirErr))
	}
	slog.Info("Finished searching directories")
	return videos
}

func createVideo(path string, fileInfo os.FileInfo, fileID FileIdentity) models.Video {
	video := models.Video{
		Path:           path,
		FileName:       fileInfo.Name(),
		ModifiedAt:     fileInfo.ModTime(),
		Size:           fileInfo.Size(),
		NumHardLinks:   fileID.NumHardLinks,
		SymbolicLink:   fileID.SymbolicLink,
		IsSymbolicLink: fileID.IsSymbolicLink,
		IsHardLink:     fileID.IsHardLink,
		Inode:          fileID.Inode,
		Device:         fileID.Device,
	}
	return video
}

func checkIncludeExt(fileExt string, includeExts []string) bool {
	if len(includeExts) == 0 {
		return true
	}

	fileExtLower := strings.ToLower(fileExt)
	for _, v := range includeExts {
		v = strings.ToLower(v)
		if strings.EqualFold(fileExtLower, v) {
			// log.Printf("IncludeExt, fileExtLower: %q, includeext:
			// %q\n", fileExt, includeExts)
			return true
		}
	}
	// log.Printf("IncludeExt does not match, fileExt: %q, includeext:
	// %q\n", fileExt, includeExts)
	return false
}

func checkIncludeStr(fileName string, includeStrs []string) bool {
	if len(includeStrs) == 0 {
		return true
	}

	fileNameLower := strings.ToLower(fileName)
	for _, s := range includeStrs {
		if strings.Contains(fileNameLower, strings.ToLower(s)) {
			return true
		}
	}

	// log.Printf("IncludeStr does not match, filename: %q, includestr: %q\n", fileName, includeStrs)
	return false
}

func checkValidVideo(path string, fileInfo os.FileInfo) bool {
	if path == "" || fileInfo.Name() == "" || fileInfo.Size() <= 0 {
		return false
	}
	return true
}

func validExt(path string, c *config.Config) bool {
	// 1) Extract and normalize the file extension
	fileExt := strings.ToLower(filepath.Ext(path))
	if len(fileExt) > 0 {
		fileExt = fileExt[1:]
	}

	// 2) Check if this extension is included
	if len(c.IncludeExt.Values) > 0 {
		included := false
		for _, inc := range c.IncludeExt.Values {
			if strings.EqualFold(fileExt, strings.ToLower(inc)) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// 3) Check if this extension is ignored
	for _, ig := range c.IgnoreExt.Values {
		if strings.EqualFold(fileExt, strings.ToLower(ig)) {
			return false
		}
	}
	return true
}

func validFileName(d fs.DirEntry, c *config.Config) bool {
	// 1) Convert file name to lowercase
	fileName := strings.ToLower(d.Name())

	// 2) Check if file name is ignored
	for _, ig := range c.IgnoreStr.Values {
		if strings.Contains(fileName, strings.ToLower(ig)) {
			return false
		}
	}

	// 3) Check if file name is included (if includes are provided)
	if len(c.IncludeStr.Values) > 0 {
		for _, inc := range c.IncludeStr.Values {
			if strings.Contains(fileName, strings.ToLower(inc)) {
				// Found an include match, so we can allow it
				return true
			}
		}
		// No includes matched -> not valid
		return false
	}

	// If no include strings are specified, default to true
	return true
}
