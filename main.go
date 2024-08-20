package main

import (
	"io/fs"
	"log"
	"os"
	"strings"
)

var wrongArgsMsg string = "Error, your input must include a filedirectory path"

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

    var filePaths []string = make([]string, 0)
    walkDirErr := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            log.Fatal(err)
        }
        // TESTING, REMOVE LATER
        if strings.Contains(path, "git") {
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

    log.Println("Printing all file found: ")
    for _, v := range filePaths {
       log.Println(v)
    }
}

