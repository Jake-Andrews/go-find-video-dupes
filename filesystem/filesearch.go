package filesystem

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"govdupes/config"
	"govdupes/models"
)

func SearchDirs(c *config.Config) *[]models.Video {
	videos := make([]models.Video, 0)

	for _, dir := range c.StartingDirs.Values {
		log.Printf("Searching recursively starting from: %q\n", dir)
		fileSystem := os.DirFS(dir)
		videos = append(videos, getVideosFromFS(fileSystem, c)...)
		log.Println("Printing all files found: ")
		for _, v := range videos {
			log.Println(v)
		}
	}

	if len(videos) == 0 {
		log.Fatalf("Quitting, no files were found!\n")
	}
	return &videos
}

// check if file ext is in ignoreext, if so ignore
// check if file ext is in includeext, if so consider the file
// check if file name is in ignorestr, if so ignore
// check if filename is in includestr, if so include consider the file
// if both includeext/includestr agree then include the file
func getVideosFromFS(fileSystem fs.FS, c *config.Config) []models.Video {
	videos := make([]models.Video, 0)

	walkDirErr := fs.WalkDir(
		fileSystem,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error walking through filesystem, err: %v", err)
				return err
			}
			if d.IsDir() {
				log.Printf("Dir, Path: %q\n", path)
				return nil
			}
			fileName := d.Name()
			fileExt := strings.ToLower(filepath.Ext(path))
			if len(fileExt) > 0 {
				fileExt = fileExt[1:]
			}
			fileInfo, err := d.Info()
			if err != nil {
				log.Printf("Error getting the fs.DirEntry.Info(), err: %v", err)
			}

			for _, v := range c.IgnoreExt.Values {
				v = strings.ToLower(v)
				if strings.EqualFold(fileExt, v) {
					log.Printf("Ignoring file, name: %q, ignoreext: %q\n", fileName, c.IgnoreExt.Values)
					return nil
				}
			}
			for _, s := range c.IgnoreStr.Values {
				s = strings.ToLower(s)
				if strings.Contains(fileName, s) {
					log.Printf("Ignoring file ignorestr, name: %q, ignorestr: %q\n", fileName, c.IgnoreStr.Values)
					return nil
				}
			}

			matchExt := false
			for _, v := range c.IncludeExt.Values {
				v = strings.ToLower(v)
				if strings.EqualFold(fileExt, v) {
					log.Printf("IncludeExt, fileExt: %q, includeext: %q\n", fileExt, c.IncludeExt.Values)
					matchExt = true
					break
				}
			}
			if !matchExt {
				log.Printf("IncludeExt does not match, fileExt: %q, includeext: %q\n", fileExt, c.IncludeExt.Values)
				return nil
			}

			if !checkIncludeStr(fileName, c.IncludeStr.Values) {
				return nil
			}

			if c.AbsPath {
				path, err := filepath.Abs(path)
				if err != nil {
					log.Printf("Error creating absolute path, path: %q, err: %v", path, err)
					return err
				}
			}

			video := models.Video{
				Path:       path,
				FileName:   fileInfo.Name(),
				ModifiedAt: fileInfo.ModTime(),
				Size:       fileInfo.Size(),
			}
			videos = append(videos, video)
			return nil
		},
	)

	if walkDirErr != nil {
		log.Println(walkDirErr)
	}
	return videos
}

func createVideo(path string, fileInfo os.FileInfo) models.Video {
	// Fileinfo.Sys to get OS specific data on file including the
	// modification time/creation

	video := models.Video{
		Path:       path,
		FileName:   fileInfo.Name(),
		ModifiedAt: fileInfo.ModTime(),
		Size:       fileInfo.Size(),
	}
	return video
}

func checkIncludeStr(fileName string, includeStrs []string) bool {
	if len(includeStrs) == 0 {
		// No inclusion criteria, so always match
		return true
	}

	fileNameLower := strings.ToLower(fileName)
	for _, s := range includeStrs {
		if strings.Contains(fileNameLower, strings.ToLower(s)) {
			log.Printf("IncludeStr matches, filename: %q, includestr: %q\n", fileName, includeStrs)
			return true
		}
	}

	log.Printf("IncludeStr does not match, filename: %q, includestr: %q\n", fileName, includeStrs)
	return false
}
