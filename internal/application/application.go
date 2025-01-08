package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"govdupes/internal/config"
	store "govdupes/internal/db"
	"govdupes/internal/db/dbstore"
	"govdupes/internal/db/sqlite"
	"govdupes/internal/duplicate"
	"govdupes/internal/filesystem"
	"govdupes/internal/hash"
	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"

	"fyne.io/fyne/v2/data/binding"
	"github.com/cespare/xxhash/v2"
)

type App struct {
	Config         *config.Config
	VideoStore     store.VideoStore
	VideoProcessor *videoprocessor.FFmpegWrapper
}

func NewApplication(c *config.Config, vs store.VideoStore, vp *videoprocessor.FFmpegWrapper) *App {
	return &App{Config: c, VideoStore: vs, VideoProcessor: vp}
}

func Setup() (*App, *sql.DB) {
	slog.Info("Starting...")

	var cfg config.Config
	cfg.SetDefaults()

	logger := config.SetupLogger(cfg.LogFilePath)
	slog.SetDefault(logger)

	db := sqlite.InitDB(cfg.DatabasePath)

	vs := dbstore.NewVideoStore(db)
	vp := videoprocessor.NewFFmpegInstance(&cfg)

	return NewApplication(&cfg, vs, vp), db
}

func (a *App) Search(f *models.FilesearchUI) [][]*models.VideoData {
	dbVideos, err := a.VideoStore.GetAllVideos(context.Background())
	if err != nil {
		slog.Error("Error getting videos from DB", slog.Any("error", err))
		os.Exit(1)
	}

	fsVideos := filesystem.SearchDirs(a.Config, func(a int, b int) {
		err := f.FileCount.Set(fmt.Sprintf("%d files found...", a))
		if err != nil {
			slog.Warn("Issue setting fileCount", "fileCount", a)
		}
		err = f.AcceptedFiles.Set(fmt.Sprintf("%d files found...", b))
		if err != nil {
			slog.Warn("Issue setting AcceptedFiles", "AcceptedFiles", b)
		}
	})
	if len(fsVideos) == 0 {
		slog.Info("No files found in directory. Exiting!")
		return nil
	}

	// Filter out any "files" that are already in DB (based on dev/inode and path)
	videosNotInDB := reconcileVideosWithDB(fsVideos, dbVideos)

	if len(videosNotInDB) != 0 {

		validVideos := make([]*models.Video, 0, len(videosNotInDB))
		l := len(videosNotInDB)
		for i, vid := range videosNotInDB {
			// UI
			progress := float64(i) / float64(l)
			if err := f.GetFileInfoProgress.Set(progress); err != nil {
				slog.Error("failed to set file info progress",
					"video", vid,
					"progress", progress,
					"error", err,
				)
			}

			if err := ffprobe.GetVideoInfo(vid); err != nil {
				vid.Corrupted = true
				slog.Warn("Skipping corrupted file",
					slog.String("path", vid.Path),
					slog.Any("error", err))
				continue
			}
			validVideos = append(validVideos, vid)
		}
		f.GetFileInfoProgress.Set(1.0)

		// Build DB lookups for device/inode and size/xxhash
		deviceInodeToDBVideo := make(map[[2]uint64]*models.Video, len(dbVideos))
		sizeHashToDBVideo := make(map[[2]string]*models.Video, len(dbVideos))

		for _, v := range dbVideos {
			keyDevIno := [2]uint64{v.Device, v.Inode}
			deviceInodeToDBVideo[keyDevIno] = v

			if v.Size > 0 && v.XXHash != "" {
				keySizeHash := [2]string{strconv.FormatInt(v.Size, 10), v.XXHash}
				sizeHashToDBVideo[keySizeHash] = v
			}
		}

		// Decide if a video matches an existing DB video or is truly new.
		// If it matches (hardlink or exact duplicate), reuse that video’s existing phash info.
		var videosReuseHash []*models.Video
		var vNotRelatedToDB []*models.Video

		for _, vid := range validVideos {
			// Check device+inode in DB
			devInoKey := [2]uint64{vid.Device, vid.Inode}
			if existingDBVid, ok := deviceInodeToDBVideo[devInoKey]; ok {
				vid.FKVideoVideohash = existingDBVid.FKVideoVideohash
				videosReuseHash = append(videosReuseHash, vid)
				continue
			}

			// Check size+xxhash in DB
			sizeHashKey := [2]string{strconv.FormatInt(vid.Size, 10), vid.XXHash}
			if existingDBVid, ok := sizeHashToDBVideo[sizeHashKey]; ok {
				vid.FKVideoVideohash = existingDBVid.FKVideoVideohash
				videosReuseHash = append(videosReuseHash, vid)
				continue
			}

			vNotRelatedToDB = append(vNotRelatedToDB, vid)
		}

		// For new videos that don't match anything in DB by dev/inode
		// or size/xxhash, if their dev & inode or size & xxhash are =
		// then group them together so later we can generate one phash
		// for the group then propogate it to the rest
		// Assumption: dev & inode = exact dupe, size & xxhash = exact dupe
		var videosToCreate [][]*models.Video
		deviceInodeToIndex := make(map[[2]uint64]int)
		sizeHashToIndex := make(map[[2]string]int)

		for _, vid := range vNotRelatedToDB {
			devInoKey := [2]uint64{vid.Device, vid.Inode}
			if i, ok := deviceInodeToIndex[devInoKey]; ok {
				videosToCreate[i] = append(videosToCreate[i], vid)
				continue
			}

			sizeHashKey := [2]string{strconv.FormatInt(vid.Size, 10), vid.XXHash}
			if i, ok := sizeHashToIndex[sizeHashKey]; ok {
				videosToCreate[i] = append(videosToCreate[i], vid)
				continue
			}

			vid.FKVideoVideohash = 0
			index := len(videosToCreate)
			deviceInodeToIndex[devInoKey] = index
			sizeHashToIndex[sizeHashKey] = index
			videosToCreate = append(videosToCreate, []*models.Video{vid})
		}

		// Also append those that matched an existing DB videohash
		for _, v := range videosReuseHash {
			videosToCreate = append(videosToCreate, []*models.Video{v})
		}

		slog.Info("Starting to generate pHashes!")
		generatePHashes(videosToCreate, a.VideoProcessor, a.VideoStore, f.GenPHashesProgress)
		slog.Info("Done generating pHashes!")
	}

	fVideos, err := a.VideoStore.GetAllVideos(context.Background())
	if err != nil {
		slog.Error("Error retrieving all videos", slog.Any("error", err))
		return nil
	}
	for _, vid := range fVideos {
		slog.Info("Video details", "Path", vid.Path)
	}

	fHashes, err := a.VideoStore.GetAllVideoHashes(context.Background())
	if err != nil {
		slog.Error("Error retrieving all video hashes", slog.Any("error", err))
		return nil
	}
	for _, vhash := range fHashes {
		slog.Info("Videohash", "vhash.ID", vhash.ID, "vhash.bucket", vhash.Bucket)
	}

	if len(fVideos) != len(fHashes) {
		slog.Warn("Mismatch in number of videos and video hashes",
			slog.Int("videosCount", len(fVideos)),
			slog.Int("hashesCount", len(fHashes)))
	}

	slog.Info("Starting to match hashes")
	err = duplicate.FindVideoDuplicates(fHashes)
	for _, vhash := range fHashes {
		slog.Info("Videohash", "vhash.ID", vhash.ID, "vhash.bucket", vhash.Bucket)
	}
	if err != nil {
		slog.Error("Error determining duplicates", slog.Any("error", err))
		os.Exit(1)
	}

	if err := a.VideoStore.BulkUpdateVideohashes(context.Background(), fHashes); err != nil {
		slog.Error("Error in BulkUpdateVideohashes", slog.Any("error", err))
	}

	duplicateVideoData, err := a.VideoStore.GetDuplicateVideoData(context.Background())
	if err != nil {
		slog.Error("Error getting duplicate video data", slog.Any("error", err))
		return nil
	}
	slog.Info("Number of duplicate video groups", slog.Int("count", len(duplicateVideoData)))
	return duplicateVideoData
}

// reconcileVideosWithDB returns a subset of 'videosFromFS' that are not already
// in DB (based on path + device/inode/size checks).
func reconcileVideosWithDB(videosFromFS []*models.Video, dbVideos []*models.Video) []*models.Video {
	dbPathToVideo := make(map[string]models.Video, len(dbVideos))
	for _, dbv := range dbVideos {
		dbPathToVideo[dbv.Path] = *dbv
	}

	var results []*models.Video
	for _, fsVid := range videosFromFS {
		if match, exists := dbPathToVideo[fsVid.Path]; exists {
			sameInodeDevice := (fsVid.Inode == match.Inode) && (fsVid.Device == match.Device)
			sameSize := (fsVid.Size == match.Size)
			if sameInodeDevice && sameSize {
				slog.Info("Skipping filesystem video already in DB",
					slog.String("path", fsVid.Path))
				continue
			}
		}
		results = append(results, fsVid)
	}
	return results
}

func CalculateXXHash(h *xxhash.Digest, v *models.Video) error {
	f, err := os.Open(v.Path)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer f.Close()

	offset := int64(0)
	const bufferSize = 65536
	buf := make([]byte, bufferSize)
	eof := false

	for {
		n, readErr := f.ReadAt(buf, offset)
		if errors.Is(readErr, io.EOF) {
			buf = buf[:n]
			eof = true
		} else if readErr != nil {
			return fmt.Errorf("error reading file: %v", readErr)
		}
		h.Write(buf)
		if eof {
			break
		}
		offset += int64(bufferSize)
	}
	return nil
}

func writeDuplicatesToJSON(dupeVideoIndexes [][]int, fVideos []*models.Video, outputPath string) error {
	duplicateGroups := make([][]models.Video, len(dupeVideoIndexes))
	for i, group := range dupeVideoIndexes {
		duplicateGroups[i] = make([]models.Video, len(group))
		for j, index := range group {
			if index < 1 || index > len(fVideos) {
				slog.Warn("Invalid index in group, skipping...",
					slog.Int("index", index),
					slog.Int("group", i))
				continue
			}
			duplicateGroups[i][j] = *fVideos[index-1] // convert 1-based to 0-based index
		}
	}
	data := map[string]any{
		"duplicateGroups": duplicateGroups,
	}
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(data); err != nil {
		return err
	}
	slog.Info("Duplicate groups successfully written to JSON", slog.String("output", outputPath))
	return nil
}

// Helper to find matches by device+inode or size+xxhash
func findMatchingVideo(
	deviceInodeKey [2]uint64,
	sizeHashKey [2]string,
	deviceInodeMap map[[2]uint64]*models.Video,
	sizeHashMap map[[2]string]*models.Video,
) (*models.Video, bool) {
	if vid, ok := deviceInodeMap[deviceInodeKey]; ok {
		return vid, true
	}
	if vid, ok := sizeHashMap[sizeHashKey]; ok {
		return vid, true
	}
	return nil, false
}

func generatePHashes(videosToCreate [][]*models.Video, vp *videoprocessor.FFmpegWrapper, videoStore store.VideoStore, PHasheProgress binding.Float) {
	videosToCreateLen := len(videosToCreate)
	for i, group := range videosToCreate {
		slog.Debug("Processing group", slog.Int("groupSize", len(group)))
		progress := float64(i) / float64(videosToCreateLen)
		PHasheProgress.Set(progress)

		// If the first in the group already has a hash, reuse it
		if group[0].FKVideoVideohash != 0 {
			continue
		}

		pHash, screenshots, err := hash.Create(vp, group[0])
		if err != nil {
			slog.Warn("Skipping pHash generation",
				slog.String("path", group[0].Path),
				slog.Any("error", err))
			continue
		}

		// If the phash is a solid color, skip
		if strings.EqualFold(pHash.HashValue, "8000000000000000") ||
			strings.EqualFold(pHash.HashValue, "0000000000000000") {
			slog.Warn("Skipping video with solid color pHash",
				slog.String("path", group[0].Path),
				slog.String("pHash", pHash.HashValue))
			continue
		}

		for _, video := range group {
			if err := videoStore.CreateVideo(context.Background(), video, pHash, screenshots); err != nil {
				slog.Error("FAILED to create video in DB",
					slog.String("path", video.Path),
					slog.Any("error", err))
				continue
			}
			slog.Info("Created new video with pHash",
				slog.String("path", video.Path),
				slog.String("pHash", pHash.HashValue))
		}
	}
	PHasheProgress.Set(1.0)
}

func generatePHashesParallel(videosToCreate [][]*models.Video, vp *videoprocessor.FFmpegWrapper, videoStore store.VideoStore) {
	const workerCount = 4
	videoChan := make(chan []*models.Video, len(videosToCreate))
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range videoChan {
				if group[0].FKVideoVideohash != 0 {
					continue
				}

				pHash, screenshots, err := hash.Create(vp, group[0])
				if err != nil {
					slog.Warn("Skipping pHash generation",
						slog.String("path", group[0].Path),
						slog.Any("error", err))
					continue
				}

				if strings.EqualFold(pHash.HashValue, "8000000000000000") ||
					strings.EqualFold(pHash.HashValue, "0000000000000000") {
					slog.Warn("Skipping video with solid color pHash",
						slog.String("path", group[0].Path),
						slog.String("pHash", pHash.HashValue))
					continue
				}

				for _, video := range group {
					if err := videoStore.CreateVideo(context.Background(), video, pHash, screenshots); err != nil {
						slog.Error("FAILED to create video in DB",
							slog.String("path", video.Path),
							slog.Any("error", err))
						continue
					}
					slog.Info("Created new video with pHash",
						slog.String("path", video.Path),
						slog.String("pHash", pHash.HashValue))
				}
			}
		}()
	}

	for _, group := range videosToCreate {
		videoChan <- group
	}
	close(videoChan)

	wg.Wait()
	slog.Info("All pHash generation workers completed.")
}

func computeXXHashes(videos []*models.Video) []*models.Video {
	var wg sync.WaitGroup
	videoChan := make(chan *models.Video, len(videos))
	validVideosChan := make(chan *models.Video, len(videos))

	const workerCount = 16

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vid := range videoChan {
				digest := xxhash.NewWithSeed(uint64(vid.Size))
				if err := CalculateXXHash(digest, vid); err != nil {
					slog.Error("XXHash failure",
						slog.String("path", vid.Path),
						slog.Any("error", err))
					continue
				}
				vid.XXHash = strconv.FormatUint(digest.Sum64(), 10)
				validVideosChan <- vid
			}
		}()
	}

	for _, vid := range videos {
		videoChan <- vid
	}
	close(videoChan)

	wg.Wait()
	close(validVideosChan)

	var validVideos []*models.Video
	for vid := range validVideosChan {
		validVideos = append(validVideos, vid)
	}
	return validVideos
}
