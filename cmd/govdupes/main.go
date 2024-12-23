package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"govdupes/internal/config"
	"govdupes/internal/db/dbstore"
	"govdupes/internal/db/sqlite"
	"govdupes/internal/duplicate"
	"govdupes/internal/filesystem"
	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"
	"govdupes/ui"

	phash "govdupes/internal/hash"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"
	_ "modernc.org/sqlite"
)

var (
	wrongArgsMsg string = "Error, your input must include only one arg which contains the path to the filedirectory to scan."
	logLevel     string = "error"
)

func main() {
	var config config.Config
	config.ParseArgs()

	db := sqlite.InitDB(config.DatabasePath.String())
	defer db.Close()

	repo := dbstore.NewVideoStore(db)
	vp := videoprocessor.NewFFmpegInstance(logLevel)

	videos := filesystem.SearchDirs(&config)
	dbVideos, err := repo.GetAllVideos(context.Background())
	if err != nil {
		log.Fatalf("Error getting videos from data, err: %v\n", err)
	}
	videosNotInDB := videoExistsInDB(videos, dbVideos)

	validVideos := make([]models.Video, 0, len(videosNotInDB))
	for _, v := range videosNotInDB {
		err := ffprobe.GetVideoInfo(&v)
		if err != nil {
			v.Corrupted = true
			log.Printf("Error getting video info, skipping file with path: %q, err: %v\n", v.Path, err)
			continue
		}
		validVideos = append(validVideos, v)
	}

	for _, v := range validVideos {
		pHash, screenshots, err := phash.Create(vp, &v)
		if err != nil {
			log.Printf("Error, trying to generate pHash, fileName: %q, err: %v", v.FileName, err)
			continue
		}
		if strings.EqualFold(pHash.HashValue, "8000000000000000") || strings.EqualFold(pHash.HashValue, "0000000000000000") {
			log.Printf("Skipping video: %q, phash is entirely one colour: %q", v.Path, pHash.HashValue)
		}

		if err := repo.CreateVideo(context.Background(), &v, pHash, screenshots); err != nil {
			log.Printf("FAILED to create video in DB, skipping video: %v", err)
			continue
		}
		log.Println(v)
	}

	fVideos, err := repo.GetAllVideos(context.Background())
	if err != nil {
		log.Println(err)
	}
	fHashes, err := repo.GetAllVideoHashes(context.Background())
	if err != nil {
		log.Println(err)
	}
	if len(fVideos) != len(fHashes) {
		log.Fatalf("Error fVideos len: %d, fHashes:%d", len(fVideos), len(fHashes))
	}

	for _, h := range fHashes {
		log.Println("sneed")
		log.Println(*h)
	}
	for _, v := range fVideos {
		log.Println("feed")
		log.Println(*v)
	}

	log.Println("Starting to match hashes")
	hashDuplicates, err := duplicate.FindVideoDuplicates(fHashes)
	if err != nil {
		log.Fatalf("Error trying to determine duplicates, err: %v", err)
	}
	repo.BulkUpdateVideohashes(context.Background(), fHashes)

	log.Println(hashDuplicates)
	log.Println("Printing duplicate video groups:")
	for i := 0; i < len(hashDuplicates); i++ {
		log.Printf("Video group #%d", i)
		for _, k := range hashDuplicates[i] {
			j := k - 1 // sqlite3 primary key auto increment start at 1
			log.Printf("Filename: %q, path: %q", fVideos[j].FileName, fVideos[j].Path)
		}
	}

	duplicateVideoData, err := repo.GetDuplicateVideoData(context.Background())
	if err != nil {
		log.Println(err)
	}

	a := app.New()
	w := a.NewWindow("Duplicate Videos Demo")
	w.SetContent(ui.CreateUI(duplicateVideoData))

	// Optional: set a theme
	a.Settings().SetTheme(theme.LightTheme())

	// Show and run
	w.ShowAndRun()
}

// writeDuplicatesToJSON(hashDuplicates, fVideos, "dups.json")

func videoExistsInDB(v []models.Video, dbVideos []*models.Video) []models.Video {
	// map[filepath (string)]models.Video quickly check if video exists in DB
	dbPathToVideo := make(map[string]models.Video, len(dbVideos))
	trimmedVideos := make([]models.Video, 0, len(v))
	for _, video := range dbVideos {
		dbPathToVideo[video.Path] = *video
	}

	for _, video := range v {
		if matchingVideo, exists := dbPathToVideo[video.Path]; exists {
			log.Printf("Video found in DB with matching name: %+v\n", matchingVideo)
			// improve later, path + size is not wholely sufficient to
			// determine duplicates, quick hash (md5, etc...) hash or more file info
			if identicalVideoChecker(&video, &matchingVideo) {
				continue
			}
		}
		trimmedVideos = append(trimmedVideos, video)
	}
	return trimmedVideos
}

// at this point the file has been gathered from filesearch.go and
// must have a Path, FileName, ModifiedAt, Size
// ffprobe adds other information later on, could get slightly more from
// updating filesearch.go, but not much more to check if videos are =
// computing a simple md5 hash would be bulletproof, consider later.
func identicalVideoChecker(v *models.Video, dbV *models.Video) bool {
	if v.Size == dbV.Size {
		log.Printf("Video found in DB also has the same size, size1: %d size2: %d\n", v.Size, dbV.Size)
		return true
	}
	return false
}

func writeDuplicatesToJSON(hashDuplicates [][]int, fVideos []*models.Video, outputPath string) error {
	// Create a structure to hold duplicate groups
	duplicateGroups := make([][]models.Video, len(hashDuplicates))

	// Populate the structure
	for i, group := range hashDuplicates {
		duplicateGroups[i] = make([]models.Video, len(group))
		for j, index := range group {
			if index < 1 || index > len(fVideos) {
				log.Printf("Invalid index %d in group %d, skipping...", index, i)
				continue
			}
			duplicateGroups[i][j] = *fVideos[index-1] // Convert 1-based to 0-based index
		}
	}

	// Wrap groups in a top-level structure
	data := map[string]interface{}{
		"duplicateGroups": duplicateGroups,
	}

	// Create and write to the JSON file
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty-print JSON
	if err := encoder.Encode(data); err != nil {
		return err
	}

	log.Printf("Duplicate groups successfully written to %s", outputPath)
	return nil
}
