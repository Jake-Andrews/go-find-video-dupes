package main

import (
	"image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"govdupes/config"
	"govdupes/models"

	"github.com/corona10/goimagehash"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

/*
General overview
- Crawl fs, find video files
- Generate images from videos
- Apply hashing algorithm to images
- Compare hashes to find duplicates

Implementation details
Things to consider
  - Structuring code for Go routines/concurrency (obviously)
  - **Heavy considerations for cpu usage/memory, possibly user defined limit
    - FS crawling can be done in memory, rest shouldn't
  - Save hash results/duplicate list
  - Flag to not delete any files, only output duplicate filepaths to a list
  - Don't delete symbolic links/correctly identify them
FS
  - Skip/Include folders/files by name (regex)
  - Skip/Include certain file types
  - Soft/Hard link considerations
Image generation
  - Sample by seconds/frame count
  - Frequency of samples
Hashing algorithms
  - phash, ahash, dhash, wavelet (later)
Comparing hashes
  - hamming distance
*/

var wrongArgsMsg string = "Error, your input must include only one arg which contains the path to the filedirectory to scan."

//var ignoreStr string = "git"
//var (
//  testFName string = "sh.mp4"
//  scFolder  string = "./screenshots"
//

// future cli arg
var fps int = 1

func main() {
	var config config.Config
	config.ParseArgs()

	videos := make([]models.Video, 0)
	for _, dir := range config.StartingDirs {
		log.Printf("Searching recursively starting from: %q\n", dir)
		fileSystem := os.DirFS(dir)
		videos := getVideos(fileSystem, config.IgnoreStr, config.IncludeStr, config.IgnoreExt, config.IncludeExt, false)
		log.Println("Printing all files found: ")
		for _, v := range videos {
			log.Println(v)
		}
	}

	for _, v := range videos {
		videoReader, videoWriter := io.Pipe()
		go func() {
			ffErr := ffmpeg.
				Input(v.Path).
				Filter("scale", ffmpeg.Args{"64:64"}). // Resize to 64x64 pixels
				Filter("fps", ffmpeg.Args{"15"}).      // Set frame rate to 15 fps
				Output("pipe:",
					ffmpeg.KwArgs{
						"pix_fmt":  "rgb24",      // RGB24 color format
						"vcodec":   "libx264",    // Video codec
						"movflags": "+faststart", // MP4 format optimization
						"an":       "",           // Disable audio
					}).
				WithOutput(videoWriter).
				GlobalArgs("-loglevel", "verbose"). // Set verbose logging
				OverWriteOutput().                  // unsure
				ErrorToStdOut().
				Run()
			if ffErr != nil {
				log.Fatalf("Error using ffmpeg to generate normalized video, video: %v, err: %v", v, ffErr)
			}
		}()
		var screenshots []io.Reader
		for frameIdx := 0; ; frameIdx++ {
			screenshotReader, screenshotWriter := io.Pipe()

			go func() {
				defer screenshotWriter.Close()
				frameErr := ffmpeg.
					Input("pipe:").
					Output("pipe:", ffmpeg.KwArgs{
						"vf":      "select=eq(n\\," + strconv.Atoi(frameIdx)) + ")",
						"vframes": "1",
						"f":       "image2",
					}).
					WithInput(videoReader).
					WithOutput(screenshotWriter).
					OverWriteOutput().
					ErrorToStdOut().
					Run()
				if frameErr != nil {
					log.Printf("Error extracting frame, Frame: %d, Error: %v", frameIdx, frameErr)
				}
			}()
			screenshots = append(screenshots, screenshotReader)
		}

		for _, screenshot := range screenshots {
			img, err := png.Decode(screenshot)
			if err != nil {
				log.Printf("Error decoding image, err: %v", err)
				continue
			}

			hash, err := goimagehash.PerceptionHash(img)
			if err != nil {
				log.Printf("Error generating perceptual hash, err: %v", err)
			}
			log.Println(hash.ToString())
		}
	}
}

func getVideos(fileSystem fs.FS, ignoreStr []string, includeStr []string, ignoreExt []string, includeExt []string, absPath bool) []models.Video {
	videos := make([]models.Video, 0)
	walkDirErr := fs.WalkDir(
		fileSystem,
		".",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Printf("Error, walking through filesystem, err: %v", err)
				return err
			}
			if d.IsDir() {
				log.Printf("Dir, Path: %q\n", path)
				return nil
			}

			fName := d.Name()
			fNameExt := strings.ToLower(filepath.Ext(path))
			if !strings.EqualFold(ignoreStr[0], "") {
				for _, s := range ignoreStr {
					s = strings.ToLower(s)
					if strings.Contains(fName, s) {
						return nil
					}
				}
			}
			if !strings.EqualFold(includeStr[0], "") {
				for _, s := range includeStr {
					s = strings.ToLower(s)
					if !strings.Contains(fName, s) {
						return nil
					}
				}
			}

			for _, v := range includeExt {
				v = strings.ToLower(v)
				if !strings.EqualFold(fNameExt, v) {
					return nil
				}
			}

			for _, v := range ignoreExt {
				v = strings.ToLower(v)
				if strings.EqualFold(fNameExt, v) {
					return nil
				}
			}

			log.Printf("File, Path: %q\n", path)
			if absPath {
				path, err := filepath.Abs(path)
				if err != nil {
					log.Printf("Error creating absolute path, path: %q, err: %v", path, err)
					return err
				}
			}

			/*
			   type FileInfo interface {
			       Name() string       // base name of the file
			       Size() int64        // length in bytes for regular files; system-dependent for others
			       Mode() FileMode     // file mode bits
			       ModTime() time.Time // modification time
			       IsDir() bool        // abbreviation for Mode().IsDir()
			       Sys() any           // underlying data source (can return nil)
			   }
			*/

			// Fileinfo.Sys to get OS specific data on file including the
			// modification time/creation

			fileInfo, err := d.Info()
			if err != nil {
				log.Printf("Error, getting the FileInfo, fName: %q, err: %v", fName, err)
				return err
			}
			video := models.Video{
				Path:       path,
				FileName:   fName,
				ModifiedAt: fileInfo.ModTime(),
				Size:       fileInfo.Size(),
				Format:     fNameExt,
			}
			videos = append(videos, video)
			return nil
		},
	)

	if walkDirErr != nil {
		log.Println(walkDirErr)
	}
	// add videos to database
	return videos
}

/*
// temp for debugging
cwd, err := os.Getwd()
if err != nil {
    log.Fatalf("Error getting cwd, error: %v", err)
}

aerr := os.RemoveAll("tmp")
if aerr != nil {
    log.Fatalf("Error removing tmp dir, error: %v", aerr)
}
mkdirErr := os.MkdirAll("tmp", 0o755)
if mkdirErr != nil {
    log.Fatalf("Error making tmp folder, error: %v", mkdirErr)
}
verr := os.RemoveAll("tmpv")
if verr != nil {
    log.Fatalf("Error removing tmp dir, error: %v", verr)
}
merr := os.MkdirAll("tmpv", 0o755)
if merr != nil {
    log.Fatalf("Error making tmp folder, error: %v", merr)
}*/
