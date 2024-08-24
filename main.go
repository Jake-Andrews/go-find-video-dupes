package main

import (
	"io/fs"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

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
var wrongArgsMsg string = "Error, your input must include a filedirectory path"
var ignoreStr string = "git"
var testFName string = "sh.webm"
var scFolder string = "./screenshots/"

//future cli args
var fps int = 1

func main()  {
    args := os.Args
    if len(args) != 2 {
        log.Fatalln(wrongArgsMsg)
    }

	mkdirErr := os.MkdirAll(scFolder, 0644)
	if mkdirErr != nil {
		log.Fatalf("Error making screenshot folder, folder path: %q, error: %v", scFolder, mkdirErr)
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

    filePaths := getFilePaths(fileSystem, ignoreStr)
    log.Println("Printing all file found: ")
    for _, v := range filePaths {
       log.Println(v)
    }

	//https://superuser.com/questions/135117/how-to-extract-one-frame-of-a-video-every-n-seconds-to-an-image
	//myimage_%04d.png
	//%0xd > zero-padded int x digits long

	//1 frame per second = 3600 for an hour
	//therefore, %05d is fine, 0-99999 = 27.7~ hours at 1 fps
	fNameNoExt := strings.TrimSuffix(testFName, path.Ext(testFName))
	strFps := strconv.Itoa(fps)
	ffmpegErr := ffmpeg.Input(testFName).
		Output(fNameNoExt + "%05d" + ".png", ffmpeg.KwArgs{"r": strFps}).
		OverWriteOutput().ErrorToStdOut().
		Run()

    if ffmpegErr != nil {
        log.Printf("Error, ffmpeg, err: %v", ffmpegErr)
    }
}

func getFilePaths(fileSystem fs.FS, ignoreStr string) []string  {
    var filePaths []string = make([]string, 0)
    walkDirErr := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            log.Printf("Error, walking through filesystem, err: %v", err)
            return err
        }
        if strings.Contains(path, ignoreStr) {
            return nil
        }
        if d.IsDir() {
            log.Printf("Dir, Path: %q\n", path)
            return nil
        }
        log.Printf("File, Path: %q\n", path)
        filePaths = append(filePaths, path)
        return nil
    })
    if walkDirErr != nil {
        log.Println(walkDirErr)
    }

    return filePaths
}
