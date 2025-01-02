package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"

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

	videoStore := dbstore.NewVideoStore(db)
	vp := videoprocessor.NewFFmpegInstance(logLevel)

	videos := filesystem.SearchDirs(&config)
	if len(videos) <= 1 {
		log.Println("No files found in directory exiting!")
	}
	dbVideos, err := videoStore.GetAllVideos(context.Background())
	if err != nil {
		log.Fatalf("Error getting videos from data, err: %v\n", err)
	}
	videosNotInDB := reconcileVideosWithDB(videos, dbVideos)
	for _, v := range videos {
		digest := xxhash.NewWithSeed(uint64(v.Size))
		if err := CalculateXXHash(digest, v); err != nil {
			v.XXHash = ""
			continue
		}
		v.XXHash = strconv.FormatUint(digest.Sum64(), 10)
		log.Println(v)
	}

	validVideos := make([]*models.Video, 0, len(videosNotInDB))
	for _, v := range videosNotInDB {
		err := ffprobe.GetVideoInfo(v)
		if err != nil {
			v.Corrupted = true
			log.Printf("Error getting video info, skipping file with path: %q, err: %v\n", v.Path, err)
			continue
		}
		validVideos = append(validVideos, v)
	}

	for _, v := range validVideos {
		log.Println(v)
		pHash, screenshots, err := phash.Create(vp, v)
		if err != nil {
			log.Printf("Error, trying to generate pHash, fileName: %q, err: %v", v.FileName, err)
			continue
		}
		if strings.EqualFold(pHash.HashValue, "8000000000000000") || strings.EqualFold(pHash.HashValue, "0000000000000000") {
			log.Printf("Skipping video: %q, phash is entirely one colour: %q", v.Path, pHash.HashValue)
		}

		if err := videoStore.CreateVideo(context.Background(), v, pHash, screenshots); err != nil {
			log.Printf("FAILED to create video in DB, skipping video: %v", err)
			continue
		}
		log.Println(v)
	}

	fVideos, err := videoStore.GetAllVideos(context.Background())
	if err != nil {
		log.Println(err)
	}
	fHashes, err := videoStore.GetAllVideoHashes(context.Background())
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
	dupeVideoIndexes, dupeVideos, err := duplicate.FindVideoDuplicates(fHashes)
	if err != nil {
		log.Fatalf("Error trying to determine duplicates, err: %v", err)
	}

	videoStore.BulkUpdateVideohashes(context.Background(), dupeVideos)

	log.Println(dupeVideoIndexes)
	log.Println("Printing duplicate video groups:")
	for i := 0; i < len(dupeVideoIndexes); i++ {
		log.Printf("Video group #%d", i)
		for _, k := range dupeVideoIndexes[i] {
			j := k - 1 // sqlite3 primary key auto increment start at 1
			log.Printf("Filename: %q, path: %q", fVideos[j].FileName, fVideos[j].Path)
		}
	}

	duplicateVideoData, err := videoStore.GetDuplicateVideoData(context.Background())
	if err != nil {
		log.Println(err)
	}
	log.Println(len(duplicateVideoData))
	ui.CreateUI(duplicateVideoData)
}

// writeDuplicatesToJSON(dupeVideoIndexes, fVideos, "dups.json")

// Compare dbvideos to filesearch videos
// If a video from filesearch == to a dbvideo
// Then reuse dbvideo and don't calculate hash/sc
// If the video path == dbvideo path but the rest of the video's
// Other stats are different then delete dbvideo and calc hash/sc

// Or...calculate md5 hash for each video in addition to the
// Phash
// After running filesearch compare each new video with a dbvideo
// and use the md5 hash to identify videos that are = to a previous
// video even if their path's are different

// The implication of the above is that videos will be stored in the DB//
// forever so this check can be done. Possibly put them in a seperate
// DB table.

// Compares videos from searching fs with videos from DB
// If the path are = and inode/device then use the db video and don't
// calculate phash for the video
// If the size is = then calculate the xxhash, if hashes are = then
// use the db video
func reconcileVideosWithDB(v []*models.Video, dbVideos []*models.Video) []*models.Video {
	// map[video struct field]models.Video quickly check if video exists in DB
	dbPathToVideo := make(map[string]models.Video, len(dbVideos))
	dbSizeToVideoSlice := make(map[int64][]models.Video, len(v))
	videosToCalc := make([]*models.Video, 0, len(v))
	for _, video := range dbVideos {
		dbPathToVideo[video.Path] = *video
		dbSizeToVideoSlice[video.Size] = append(dbSizeToVideoSlice[video.Size], *video)
	}

	h := xxhash.New()
	for _, video := range v {
		// check if videos path exists in db already
		if match, exists := dbPathToVideo[video.Path]; exists {
			if video.Size == match.Size && video.Inode == match.Inode && video.Device == match.Device {
				log.Printf("Skipping video from filesearch found video in DB with the same: size1: %d = size2: %d, inode1: %d = inode2: %d, device1: %d = device2: %d", video.Size, match.Size, video.Inode, match.Inode, video.Device, match.Device)
				continue
			} // check if videos size exists in db already, if so calc xxhash
		} else if matches, e := dbSizeToVideoSlice[video.Size]; e {
			foundAMatch := false
			for _, m := range matches {
				h.ResetWithSeed(uint64(video.Size))
				CalculateXXHash(h, video)
				hStr := strconv.FormatUint(h.Sum64(), 10)
				if hStr == m.XXHash {
					log.Printf("Skipping video from filsearch found video in DB with the same: XXHash1: %q = %q XXHash2", hStr, m.XXHash)
					foundAMatch = true
				}
				video.XXHash = hStr
			}
			if foundAMatch {
				continue
			}
		}
		videosToCalc = append(videosToCalc, video)
	}
	return videosToCalc
}

func CalculateXXHash(h *xxhash.Digest, v *models.Video) error {
	f, err := os.Open(v.Path)
	if err != nil {
		return fmt.Errorf("error opening file, err: %v", err)
	}

	// 1024 bytes * 64 (2^16)
	offset := int64(0)
	bufferSize := 65536

	buf := make([]byte, bufferSize)
	eof := false
	for {
		n, err := f.ReadAt(buf, offset)
		if errors.Is(err, io.EOF) {
			buf = buf[:n]
			eof = true
		} else if err != nil {
			return fmt.Errorf("error reading from file into buffer, err: %v", err)
		}

		// always returns len(b), nil
		h.Write(buf)
		/*if err != nil {
			return nil, fmt.Errorf("error writing buffer contents to hash, err: %v", err)
		} else if w != len(buf) {
			return nil, fmt.Errorf("error writing the contents of the buffer, len(buf): %d != %d bytes written", len(buf), w)
		}*/
		if eof {
			break
		}
		offset += int64(bufferSize) + 1
	}
	return nil
}

func writeDuplicatesToJSON(dupeVideoIndexes [][]int, fVideos []*models.Video, outputPath string) error {
	// Create a structure to hold duplicate groups
	duplicateGroups := make([][]models.Video, len(dupeVideoIndexes))

	// Populate the structure
	for i, group := range dupeVideoIndexes {
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
