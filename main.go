package main

import (
	"io/fs"
	"log"
	"os"
	"strings"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var wrongArgsMsg string = "Error, your input must include a filedirectory path"
var ignoreStr string = "git"
var testFName string = "sh.webm"

func main()  {
    args := os.Args
    if len(args) != 2 {
        log.Fatalln(wrongArgsMsg)
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

    ffmpegErr := ffmpeg.Input(testFName).
        Output("./sh_out.mp4", ffmpeg.KwArgs{"vf": "scale=w=64:h=64"}).
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
