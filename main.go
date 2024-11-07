package main

import (
	"image/png"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

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

// var ignoreStr string = "git"
var (
	testFName string = "sh.mp4"
	scFolder  string = "./screenshots"
)

// future cli args
var fps int = 1

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Fatalln(wrongArgsMsg)
	}

	if _, err := os.Stat(scFolder); os.IsNotExist(err) {
		// scFolder does not exist
		mkdirErr := os.MkdirAll(scFolder, 0o755)
		if mkdirErr != nil {
			log.Fatalf("Error making screenshot folder, folder path: %q, error: %v", scFolder, mkdirErr)
		}
	}
	if _, err := os.Stat("./normalized_sh.mp4"); !os.IsNotExist(err) {
		os.Remove("./normalized_sh.mp4")
	}

	err := os.Chdir(args[1])
	if err != nil {
		log.Fatalf("Error changing working dir, error: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting cwd, error: %v", err)
	}
	log.Printf("Searching recursively starting from: %q\n", cwd)
	fileSystem := os.DirFS(cwd)
	filePaths := getFilePaths(fileSystem, "", "", []string{".mp4"}, false)
	log.Println("Printing all files found: ")
	for _, v := range filePaths {
		log.Println(v)
	}

	// normalize videos
	ffErr := ffmpeg.
		Input(filePaths[0]).
		Filter("scale", ffmpeg.Args{"64:64"}). // Resize to 64x64 pixels
		Filter("fps", ffmpeg.Args{"15"}).      // Set frame rate to 15 fps
		Output("normalized_"+filePaths[0],
			ffmpeg.KwArgs{
				"pix_fmt":  "rgb24",      // RGB24 color format
				"vcodec":   "libx264",    // Video codec
				"movflags": "+faststart", // MP4 format optimization
				"an":       "",           // Disable audio
							}).
		GlobalArgs("-loglevel", "verbose"). // Set verbose logging
		OverWriteOutput().
		ErrorToStdOut().
		Run()

	if ffErr != nil {
		log.Printf("Error using ffmpeg to generate normalized video, video: %q, err: %v", filePaths[0], ffErr)
	}

	//note look into proper usage of ffmpeg to extract frames, do not want
	//intermediate frames
	//https://superuser.com/questions/135117/how-to-extract-one-frame-of-a-video-every-n-seconds-to-an-image
	//myimage_%04d.png
	//%0xd > zero-padded int x digits long
	// 1 frame per second = 3600 for an hour
	// therefore, %05d is fine, 0-99999 = 27.7~ hours at 1 fps
	fNameNoExt := strings.TrimSuffix(testFName, path.Ext(testFName))
	strFps := strconv.Itoa(fps)
	ffmpegErr := ffmpeg.Input("normalized_"+filePaths[0]).
		Output(scFolder+"/"+fNameNoExt+"%05d"+".png", ffmpeg.KwArgs{"r": strFps}).
		OverWriteOutput().ErrorToStdOut().
		Run()
	if ffmpegErr != nil {
		log.Printf("Error, ffmpeg, err: %v", ffmpegErr)
	}

	log.Println(cwd + scFolder[1:])
	scFolderPath := cwd + scFolder[1:]
	scFS := os.DirFS(scFolderPath)
	log.Printf("Changing cwd to: %q", scFolderPath)
	chdirErr := os.Chdir(scFolderPath)
	if chdirErr != nil {
		log.Fatalf("Error changing directory to: %q, err: %v", scFolderPath, chdirErr)
	}

	scPaths := getFilePaths(scFS, "", "", []string{".png"}, true)
	log.Println("Printing all screenshots created: ")
	for _, v := range scPaths {
		log.Println(v)
		f, err := os.Open(v)
		if err != nil {
			log.Printf("Error opening image, path: %q, err: %v", v, err)
			continue
		}

		img, err := png.Decode(f)
		if err != nil {
			log.Printf("Error decoding image, path: %q, err: %v", v, err)
		}

		hash, err := goimagehash.PerceptionHash(img)
		if err != nil {
			log.Printf("Error generating perceptual hash, path: %q, err: %v", v, err)
		}
		log.Println(hash.ToString())

		f.Close()
	}

	cerr := os.Chdir(cwd)
	if cerr != nil {
		log.Fatalf("Error changing working dir, error: %v", cerr)
	}
	cleanup()
	log.Println("sneed")
}

func getFilePaths(fileSystem fs.FS, ignoreStr string, includeStr string, includeExt []string, absPath bool) []string {
	var filePaths []string = make([]string, 0)
	walkDirErr := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error, walking through filesystem, err: %v", err)
			return err
		}
		if d.IsDir() {
			log.Printf("Dir, Path: %q\n", path)
			return nil
		}

		if !strings.EqualFold(ignoreStr, "") {
			if strings.Contains(path, ignoreStr) {
				return nil
			}
		}
		if !strings.EqualFold(includeStr, "") {
			if !strings.Contains(path, includeStr) {
				return nil
			}
		}

		extMatched := true
		for _, v := range includeExt {
			v = strings.ToLower(v)
			if pathExt := strings.ToLower(filepath.Ext(path)); strings.EqualFold(pathExt, v) {
				extMatched = true
				break
			} else {
				extMatched = false
			}
		}
		if !extMatched {
			return nil
		}

		log.Printf("File, Path: %q\n", path)
		if absPath {
			fPath, err := filepath.Abs(path)
			if err != nil {
				log.Printf("Error creating absolute path, path: %q, err: %v", path, err)
			}
			filePaths = append(filePaths, fPath)
			return nil
		}
		filePaths = append(filePaths, path)
		return nil
	})
	if walkDirErr != nil {
		log.Println(walkDirErr)
	}

	return filePaths
}

func cleanup() {
	log.Println(scFolder)
	cwd, _ := os.Getwd()
	log.Println(cwd)
	finfo, err := os.Stat(scFolder)
	if os.IsNotExist(err) {
		// scFolder does not exist
		log.Println("qui?")
	}
	log.Println(finfo)
	er := os.RemoveAll(scFolder)
	if er != nil {
		log.Printf("Error removing sc folder: %q", scFolder)
	}
}
