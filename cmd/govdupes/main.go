package main

import (
	"context"
	"log"

	"govdupes/internal/config"
	"govdupes/internal/db/dbstore"
	"govdupes/internal/db/sqlite"
	"govdupes/internal/duplicate"
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

	videos := filesystem.SearchDirs(&config)
	dbVideos, _, err := repo.GetVideos(context.Background())
	if err != nil {
		log.Fatalf("Error getting videos from data, err: %v\n", err)
	}
	// create a list of map[path (string)]models.Video from dbVideos
	// if map[path] exists and size/etc...match, don't add
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

	var pHashes []*models.Videohash
	for _, v := range validVideos {
		pHash, err := phash.Create(vp, &v)
		if err != nil {
			log.Printf("Error, trying to generate pHash, fileName: %q, err: %v", v.FileName, err)
			continue
		}
		pHashes = append(pHashes, pHash)

		tmp := []*models.Videohash{pHash}
		if err := repo.CreateVideo(context.Background(), &v, tmp); err != nil {
			log.Printf("FAILED to create video: %v", err)
			continue
		}
		log.Println(v)
	}
	//dbHashes = append(dbHashes, pHashes...)
	//log.Println(dbHashes)
	//for _, h := range dbHashes {
	//	log.Println(h)
	//}
	fVideos, fHashes, err := repo.GetVideos(context.Background())
	if err != nil {
		log.Println(err)
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

	log.Println(hashDuplicates)
}

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
