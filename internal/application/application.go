package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"govdupes/internal/config"
	store "govdupes/internal/db"
	"govdupes/internal/duplicate"
	"govdupes/internal/filesystem"
	"govdupes/internal/hash"
	"govdupes/internal/models"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"
	"govdupes/internal/vm"
)

type App struct {
	Config         *config.Config
	VideoStore     store.VideoStore
	VideoProcessor *videoprocessor.FFmpegWrapper
}

func NewApplication(c *config.Config, vs store.VideoStore, vp *videoprocessor.FFmpegWrapper) *App {
	return &App{Config: c, VideoStore: vs, VideoProcessor: vp}
}

func (a *App) Search(vm vm.ViewModel) error {
	dbVideos, err := a.VideoStore.GetAllVideos(context.Background())
	if err != nil {
		slog.Error("Error getting videos from DB", slog.Any("error", err))
		os.Exit(1)
	}

	fsVideos := filesystem.SearchDirs(a.Config,
		func(a int) {
			vm.UpdateFileCount(fmt.Sprintf("%d files found...", a))
		},
		func(b int) {
			vm.UpdateAcceptedFiles(fmt.Sprintf("%d videos accepted...", b))
		},
	)

	if len(fsVideos) == 0 {
		slog.Info("No files found in directory. Exiting!")
		return nil
	}

	// Filter out any "files" that are already in DB (based on dev/inode and path)
	videosNotInDB := reconcileVideosWithDB(fsVideos, dbVideos)

	if len(videosNotInDB) != 0 {

		validVideos := GetFFprobeInfo(videosNotInDB, vm)
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
		generatePHashesParallel(videosToCreate, a, func(progress float64) {
			vm.UpdateGenPHashesProgress(progress)
		})
		slog.Info("Done generating pHashes!")
	}

	fVideos, err := a.VideoStore.GetAllVideos(context.Background())
	if err != nil {
		slog.Error("Error retrieving all videos", slog.Any("error", err))
		return err
	}
	for _, vid := range fVideos {
		slog.Info("Video details", "Path", vid.Path)
	}

	fHashes, err := a.VideoStore.GetAllVideoHashes(context.Background())
	if err != nil {
		slog.Error("Error retrieving all video hashes", slog.Any("error", err))
		return err
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
		return err
	}

	duplicateVideoData, err := a.VideoStore.GetDuplicateVideoData(context.Background())
	if err != nil {
		slog.Error("Error getting duplicate video data", slog.Any("error", err))
		return err
	}

	slog.Info("Number of duplicate video groups", slog.Int("count", len(duplicateVideoData)))

	if duplicateVideoData == nil {
		// err no data, etc...
		return nil
	}

	// Convert to a []interface{} to give to the UntypedList
	items := make([]any, len(duplicateVideoData))
	for i, grp := range duplicateVideoData {
		items[i] = grp // []*models.VideoData
	}
	vm.SetDuplicateGroups(items)
	return nil
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

func generatePHashesParallel(videosToCreate [][]*models.Video, a *App, UpdatePhashProgress func(progress float64)) {
	detectionMethod := a.Config.DetectionMethod
	const workerCount = 5
	const maxBatchSize = 10
	const maxRetries = 5
	const retryBaseDelay = 50 * time.Millisecond

	videoChan := make(chan []*models.Video, len(videosToCreate))
	progressChan := make(chan float64, len(videosToCreate))
	writeChan := make(chan *models.VideoData, maxBatchSize*workerCount)
	var wg sync.WaitGroup
	var writeWg sync.WaitGroup

	// Writer goroutine
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		var batch []*models.VideoData
		timer := time.NewTimer(1 * time.Second)
		defer timer.Stop()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}

			for retries := range maxRetries {
				if err := a.VideoStore.BatchCreateVideos(context.Background(), batch); err != nil {
					if isSQLiteBusyError(err) {
						time.Sleep(retryBaseDelay * time.Duration(1<<retries))
						continue
					}
					slog.Error("Failed to write batch to DB", slog.Any("error", err))
					return
				}
				break
			}

			batch = batch[:0]
		}

		for {
			select {
			case task, ok := <-writeChan:
				if !ok {
					flushBatch()
					return
				}

				batch = append(batch, task)
				if len(batch) >= maxBatchSize {
					flushBatch()
				}
			case <-timer.C:
				flushBatch()
				timer.Reset(1 * time.Second)
			}
		}
	}()

	// Worker goroutines
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range videoChan {
				if group[0].FKVideoVideohash != 0 {
					progressChan <- 1.0 / float64(len(videosToCreate))
					continue
				}

				pHash, screenshots, err := hash.Create(a.VideoProcessor, group[0], detectionMethod)
				if err != nil {
					slog.Warn("Skipping pHash generation", slog.String("path", group[0].Path), slog.Any("error", err))
					progressChan <- 1.0 / float64(len(videosToCreate))
					continue
				}

				// skip video if pHashes are all solid colours
				solidColor := true
				for i := 16; i < len(pHash.HashValue); i += 16 {
					if !strings.EqualFold(pHash.HashValue[i-16:i], "8000000000000000") && !strings.EqualFold(pHash.HashValue[i-16:i], "0000000000000000") {
						solidColor = false
						break
					}
				}

				if solidColor {
					slog.Warn("Skipping video with solid color pHash",
						slog.String("path", group[0].Path),
						slog.String("pHash", pHash.HashValue))
					progressChan <- 1.0 / float64(len(videosToCreate))
					continue
				}

				for _, video := range group {
					writeChan <- &models.VideoData{
						Video:      *video,
						Videohash:  *pHash,
						Screenshot: *screenshots,
					}
				}
				progressChan <- 1.0 / float64(len(videosToCreate))
			}
		}()
	}

	// Distribute work
	for _, group := range videosToCreate {
		videoChan <- group
	}
	close(videoChan)

	// Progress updater goroutine
	go func() {
		totalProgress := 0.0
		for progress := range progressChan {
			totalProgress += progress
			UpdatePhashProgress(totalProgress)
		}
	}()

	wg.Wait()
	close(writeChan)
	writeWg.Wait()
	close(progressChan)
	UpdatePhashProgress(1.0)
	slog.Info("All pHash generation workers completed.")
}

func isSQLiteBusyError(err error) bool {
	return strings.Contains(err.Error(), "database is locked")
}

func (a *App) DeleteVideosByID(ids []int64) error {
	for _, id := range ids {
		if err := a.VideoStore.DeleteVideoByID(context.Background(), id); err != nil {
			return err
		}
	}
	return nil
}

func GetFFprobeInfo(videosNotInDB []*models.Video, vm vm.ViewModel) []*models.Video {
	workerCount := 10
	validVideos := make([]*models.Video, 0, len(videosNotInDB))
	inodeDeviceMap := make(map[string]*models.Video)
	inodeDeviceMutex := sync.Mutex{}

	l := len(videosNotInDB)
	progressChan := make(chan float64, l)
	resultChan := make(chan *models.Video, l)
	taskChan := make(chan *models.Video, l)
	var wg sync.WaitGroup

	// progress updater to UI
	go func() {
		totalProgress := 0.0
		for progress := range progressChan {
			totalProgress += progress
			vm.UpdateGetFileInfoProgress(totalProgress)
		}
	}()

	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vid := range taskChan {
				// unique key for inode and device
				inodeDeviceKey := fmt.Sprintf("%d:%d", vid.Inode, vid.Device)

				// check inode/dev combination has already been processed
				inodeDeviceMutex.Lock()
				existingVid, exists := inodeDeviceMap[inodeDeviceKey]
				inodeDeviceMutex.Unlock()

				if exists {
					// reuse info for vids with matching inode/dev
					vid.Size = existingVid.Size
					vid.Duration = existingVid.Duration
					vid.BitRate = existingVid.BitRate
					vid.VideoCodec = existingVid.VideoCodec
					vid.AudioCodec = existingVid.AudioCodec
					vid.Width = existingVid.Width
					vid.Height = existingVid.Height
					vid.SampleRateAvg = existingVid.SampleRateAvg
					vid.AvgFrameRate = existingVid.AvgFrameRate
					slog.Info("Reused video info", slog.String("path", vid.Path))
				} else {
					if err := ffprobe.GetVideoInfo(vid); err != nil {
						vid.Corrupted = true
						slog.Warn("Skipping corrupted file",
							slog.String("path", vid.Path),
							slog.Any("error", err))
						progressChan <- 1.0 / float64(l) // increment progress for skipped file
						continue
					}

					// store processed inode/device info
					inodeDeviceMutex.Lock()
					inodeDeviceMap[inodeDeviceKey] = vid
					inodeDeviceMutex.Unlock()
				}

				// send valid video
				resultChan <- vid
				progressChan <- 1.0 / float64(l)
			}
		}()
	}

	// distribute tasks to workers
	go func() {
		for _, vid := range videosNotInDB {
			taskChan <- vid
		}
		close(taskChan)
	}()

	wg.Wait()
	close(resultChan)
	close(progressChan)

	for vid := range resultChan {
		validVideos = append(validVideos, vid)
	}

	vm.UpdateGetFileInfoProgress(1.0)
	return validVideos
}

/*
func generatePHashes(videosToCreate [][]*models.Video, a *App, UpdatePhashProgress func(progress float64)) {
	videosToCreateLen := len(videosToCreate)
	for i, group := range videosToCreate {
		slog.Debug("Processing group", slog.Int("groupSize", len(group)))
		progress := float64(i) / float64(videosToCreateLen)
		UpdatePhashProgress(progress)

		// If the first in the group already has a hash, reuse it
		if group[0].FKVideoVideohash != 0 {
			continue
		}

		pHash, screenshots, err := hash.Create(a.VideoProcessor, group[0], a.Config.DetectionMethod)
		if err != nil {
			slog.Warn("Skipping pHash generation",
				slog.String("path", group[0].Path),
				slog.Any("error", err))
			continue
		}

		// If the phash is a solid color, skip
		// **Rework, patch fix for slowhash**
		solidColor := true
		for i := 16; i < len(pHash.HashValue); i += 16 {
			if !(strings.EqualFold(pHash.HashValue[i-16:i], "8000000000000000") ||
				strings.EqualFold(pHash.HashValue[i-16:i], "0000000000000000")) {
				solidColor = false
				break
			}
		}

		if solidColor {
			slog.Warn("Skipping video with solid color pHash",
				slog.String("path", group[0].Path),
				slog.String("pHash", pHash.HashValue))
			continue
		}

		for _, video := range group {
			if err := a.VideoStore.CreateVideo(context.Background(), video, pHash, screenshots); err != nil {
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
	UpdatePhashProgress(1.0)
}

func computeXXHashes(videos []*models.Video) []*models.Video {
	var wg sync.WaitGroup
	videoChan := make(chan *models.Video, len(videos))
	validVideosChan := make(chan *models.Video, len(videos))

	const workerCount = 16

	for range workerCount {
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


*/
