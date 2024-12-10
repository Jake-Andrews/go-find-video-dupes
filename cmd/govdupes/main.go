package main

import (
	"context"
	"log"

	"govdupes/internal/config"
	"govdupes/internal/db/dbstore"
	"govdupes/internal/db/sqlite"
	"govdupes/internal/filesystem"
	phash "govdupes/internal/hash"
	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"

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

	// create a list of map[string]models.Video from dbVideos
	// if map[path] exists and size/etc...match, don't add
	videos := filesystem.SearchDirs(&config)
	dbVideos, _ := repo.GetVideos(context.Background())
	vNotInDB := videoExistsInDB(videos, dbVideos)

	validVideos := make([]models.Video, 0, len(vNotInDB))
	log.Println(vNotInDB)
	for _, v := range vNotInDB {
		err := ffprobe.GetVideoInfo(&v)
		if err != nil {
			log.Printf("Error getting video info, skipping file with path: %q, err: %v\n", v.Path, err)
			continue
		}
		validVideos = append(validVideos, v)
	}

	for _, v := range validVideos {
		pHash, err := phash.Create(vp, &v)
		if err != nil {
			log.Printf("Error, trying to generate pHash, fileName: %q, err: %v", v.FileName, err)
		}
		pHashes := []models.Videohash{*pHash}

		if err := repo.CreateVideo(context.Background(), &v, pHashes); err != nil {
			log.Printf("Failed to create video: %v", err)
			continue
		}
		log.Println(v)
	}
}

func videoExistsInDB(v []models.Video, dbVideos []models.Video) []models.Video {
	// map[filepath (string)]models.Video quickly check if video exists in DB
	dbPathToVideo := make(map[string]models.Video, len(dbVideos))
	trimmedVideos := make([]models.Video, 0, len(v))
	for _, video := range dbVideos {
		dbPathToVideo[video.Path] = video
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
