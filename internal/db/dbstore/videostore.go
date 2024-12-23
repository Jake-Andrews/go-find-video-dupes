package dbstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"strings"

	"govdupes/internal/models"
	"govdupes/internal/utils"

	"github.com/georgysavva/scany/v2/sqlscan"
	"golang.org/x/image/bmp"

	store "govdupes/internal/db"
)

type videoRepo struct {
	db *sql.DB
}

// NewVideoStore creates a new video repository.
func NewVideoStore(DB *sql.DB) store.VideoStore {
	return &videoRepo{
		db: DB,
	}
}

// GetDuplicateVideoData retrieves data for duplicate videos.
func (r *videoRepo) GetDuplicateVideoData(ctx context.Context) ([][]*models.VideoData, error) {
	// Step 1: Fetch all videohashes
	var videohashes []*models.Videohash
	query := `
        SELECT id, FK_videohash_video, hashValue, hashType, duration, neighbours, bucket
        FROM videohash
        WHERE bucket != -1;
    `
	err := sqlscan.Select(ctx, r.db, &videohashes, query)
	if err != nil {
		return nil, fmt.Errorf("querying videohashes: %w", err)
	}

	// Extract all video IDs from the fetched videohashes
	videoIDs := getUniqueIDsFromVideohashes(videohashes)

	// Step 2: Fetch related videos
	videos, err := r.getVideosByIDs(ctx, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related videos: %w", err)
	}

	// Step 3: Fetch screenshots for the videohashes
	screenshots, err := r.getScreenshotsByVideohashIDs(ctx, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related screenshots: %w", err)
	}

	// Step 4: Combine data into VideoData
	videoMap := map[int64]*models.Video{} // Map video IDs to videos for easy lookup

	// Map videos by their IDs for quick lookup
	for _, video := range videos {
		videoMap[video.ID] = video
	}

	// Build VideoData list
	groupedVideoData := map[int][]*models.VideoData{} // Group by bucket (or another criterion)

	for _, vh := range videohashes {
		videoData := &models.VideoData{
			Videohash: *vh,
		}

		// Attach the corresponding video
		if video, exists := videoMap[vh.FKVideohashVideo]; exists {
			videoData.Video = *video
		}

		// Attach the corresponding screenshots
		if sc, exists := screenshots[vh.ID]; exists {
			videoData.Screenshot = models.Screenshots{
				ID:                    sc.ID,
				Screenshots:           sc.Screenshots,
				FKScreenshotVideohash: sc.FKScreenshotVideohash,
			}
		}

		// Group by bucket or other criteria
		groupedVideoData[vh.Bucket] = append(groupedVideoData[vh.Bucket], videoData)
	}

	// Convert grouped data into a slice of slices
	result := make([][]*models.VideoData, 0, len(groupedVideoData))
	for _, group := range groupedVideoData {
		result = append(result, group)
	}

	return result, nil
}

// GetVideosWithValidHashes retrieves videos that have valid hashes.
func (r *videoRepo) GetVideosWithValidHashes(ctx context.Context) ([]models.Video, error) {
	query := `
		SELECT DISTINCT v.*
		FROM video v
		INNER JOIN videohash vh ON vh.FK_videohash_video = v.id
		WHERE vh.bucket != ?;
	`

	var videos []models.Video
	if err := sqlscan.Select(ctx, r.db, &videos, query, -1); err != nil {
		return nil, fmt.Errorf("querying videos: %w", err)
	}

	return videos, nil
}

// GetScreenshotsForValidHashes retrieves screenshots for valid video hashes.
func (r *videoRepo) GetScreenshotsForValidHashes(ctx context.Context) (map[int64]models.Screenshots, error) {
	// Step 1: Query video IDs with valid hashes
	query := `
		SELECT DISTINCT v.id
		FROM video v
		INNER JOIN videohash vh ON vh.FK_videohash_video = v.id
		WHERE vh.bucket != ?;
	`

	var videoIDs []int64
	if err := sqlscan.Select(ctx, r.db, &videoIDs, query, -1); err != nil {
		return nil, fmt.Errorf("querying valid video IDs: %w", err)
	}

	if len(videoIDs) == 0 {
		return make(map[int64]models.Screenshots), nil // No valid video IDs
	}

	// Step 2: Fetch screenshots for the valid video hashes
	placeholders := strings.Repeat("?,", len(videoIDs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query = fmt.Sprintf(`
		SELECT FK_screenshot_videohash, screenshots
		FROM screenshot
		WHERE FK_screenshot_videohash IN (%s);
	`, placeholders)

	var rawScreenshots []struct {
		VideohashID int64  `db:"FK_screenshot_videohash"`
		Screenshots string `db:"screenshots"`
	}
	if err := sqlscan.Select(ctx, r.db, &rawScreenshots, query, utils.ToInterfaceSlice(videoIDs)...); err != nil {
		return nil, fmt.Errorf("query raw screenshots: %w", err)
	}

	// Step 3: Decode screenshots and store them in the map
	screenshotsMap := make(map[int64]models.Screenshots)
	for _, row := range rawScreenshots {
		var base64Strings []string

		// Unmarshal the JSON array of Base64 strings
		if err := json.Unmarshal([]byte(row.Screenshots), &base64Strings); err != nil {
			return nil, fmt.Errorf("unmarshaling screenshots for videohash ID %d: %w", row.VideohashID, err)
		}

		// Decode Base64 strings into images
		var images []image.Image
		for _, b64 := range base64Strings {
			data, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return nil, fmt.Errorf("decoding Base64 for videohash ID %d: %w", row.VideohashID, err)
			}

			img, err := bmp.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("decoding BMP for videohash ID %d: %w", row.VideohashID, err)
			}

			images = append(images, img)
		}

		// Add the screenshots to the map
		screenshotsMap[row.VideohashID] = models.Screenshots{
			ID:                    row.VideohashID,
			Screenshots:           images,
			FKScreenshotVideohash: row.VideohashID,
		}
	}

	return screenshotsMap, nil
}

// CreateVideo adds a new video and associated hashes and screenshots to the database.
func (r *videoRepo) CreateVideo(ctx context.Context, video *models.Video, hash *models.Videohash, sc *models.Screenshots) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("rollback error: %v", rollbackErr)
			}
		}
	}()

	// Insert video
	result, err := tx.ExecContext(ctx, `
		INSERT INTO video (
			path, fileName, createdAt, modifiedAt, frameRate, videoCodec, 
			audioCodec, width, height, duration, size, bitRate, 
			numHardLinks, symbolicLink, isSymbolicLink, isHardLink, inode, device
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, video.Path, video.FileName, video.CreatedAt, video.ModifiedAt,
		video.FrameRate, video.VideoCodec, video.AudioCodec, video.Width,
		video.Height, video.Duration, video.Size, video.BitRate,
		video.NumHardLinks, video.SymbolicLink, video.IsSymbolicLink,
		video.IsHardLink, video.Inode, video.Device,
	)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}

	videoID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("retrieve video ID: %w", err)
	}

	neighboursJSON, err := json.Marshal(hash.Neighbours)
	if err != nil {
		return fmt.Errorf("marshal neighbours: %w", err)
	}

	hashResult, err := tx.ExecContext(ctx, `
        INSERT INTO videohash (
            FK_videohash_video, hashType, hashValue, duration, neighbours, bucket
        ) VALUES (?, ?, ?, ?, ?, ?);
    `, videoID, hash.HashType, hash.HashValue, hash.Duration, string(neighboursJSON), hash.Bucket)
	if err != nil {
		return fmt.Errorf("insert hash: %w", err)
	}

	hashID, err := hashResult.LastInsertId()
	if err != nil {
		return fmt.Errorf("retrieve hash ID: %w", err)
	}

	// Insert screenshots
	base64Images, err := sc.EncodeImages()
	if err != nil {
		return fmt.Errorf("Error encoding images to base64, err: %v", err)
	}
	jsonImages, err := json.Marshal(base64Images)
	if err != nil {
		return fmt.Errorf("marshal base64 images: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO screenshot (FK_screenshot_videohash, screenshots) 
        VALUES (?, ?);
    `, hashID, string(jsonImages))
	if err != nil {
		return fmt.Errorf("insert screenshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *videoRepo) GetVideo(ctx context.Context, videoPath string) (*models.Video, *models.Videohash, error) {
	var video models.Video
	err := sqlscan.Get(ctx, r.db, &video, `
		SELECT *
		FROM video
		WHERE path = ?;
	`, videoPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("video not found for path: %s", videoPath)
		}
		return nil, nil, fmt.Errorf("error retrieving video: %w", err)
	}

	var hash models.Videohash
	err = sqlscan.Select(ctx, r.db, &hash, `
		SELECT *
		FROM videohash
		WHERE FK_videohash_video = ?;
	`, video.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("error retrieving video hashes: %w", err)
	}

	return &video, &hash, nil
}

func (r *videoRepo) GetAllVideos(ctx context.Context) ([]*models.Video, error) {
	var videos []*models.Video
	// Fetch all videos from the database
	err := sqlscan.Select(ctx, r.db, &videos, `
		SELECT *
		FROM video
		ORDER BY id;
	`)
	if err != nil {
		return nil, fmt.Errorf("error retrieving all videos: %w", err)
	}

	return videos, nil
}

func (r *videoRepo) GetAllVideoHashes(ctx context.Context) ([]*models.Videohash, error) {
	var hashes []*models.Videohash
	err := sqlscan.Select(ctx, r.db, &hashes, `
		SELECT *
		FROM videohash
		ORDER BY id;
	`)
	if err != nil {
		return nil, fmt.Errorf("error retrieving all video hashes: %w", err)
	}

	return hashes, nil
}

func (r *videoRepo) BulkUpdateVideohashes(ctx context.Context, updates []*models.Videohash) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Panic during transaction, rolling back: %v", p)
			tx.Rollback()
			panic(p)
		} else if err != nil {
			log.Printf("Error detected, rolling back transaction: %v", err)
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE videohash
		SET hashType = ?, hashValue = ?, duration = ?, bucket = ?, neighbours = ?
		WHERE id = ?;
	`)
	if err != nil {
		log.Printf("Error preparing update statement: %v", err)
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	// Iterate over all updates and execute them in the transaction
	for _, vh := range updates {
		neighboursJSON, err := json.Marshal(vh.Neighbours)
		if err != nil {
			log.Printf("Error marshalling neighbours for videohash ID %d: %v", vh.ID, err)
			tx.Rollback()
			return fmt.Errorf("marshal neighbours for videohash ID %d: %w", vh.ID, err)
		}

		log.Printf("Updating videohash ID %d with hashType: %s, hashValue: %s, duration: %.2f, bucket: %d", vh.ID, vh.HashType, vh.HashValue, vh.Duration, vh.Bucket)
		_, err = stmt.ExecContext(ctx, vh.HashType, vh.HashValue, vh.Duration, vh.Bucket, string(neighboursJSON), vh.ID)
		if err != nil {
			log.Printf("Error updating videohash ID %d: %v", vh.ID, err)
			tx.Rollback()
			return fmt.Errorf("update videohash ID %d: %w", vh.ID, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Error committing transaction: %v", err)
		return fmt.Errorf("commit transaction: %w", err)
	}

	log.Println("Successfully updated videohash records.")
	return nil
}

func (r *videoRepo) getVideosByIDs(ctx context.Context, videoIDs []int64) ([]*models.Video, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	// Generate placeholders for query
	placeholders := strings.Repeat("?,", len(videoIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
        SELECT id, path, fileName, createdAt, modifiedAt, frameRate, videoCodec, 
               audioCodec, width, height, duration, size, bitRate, numHardLinks, 
               symbolicLink, isSymbolicLink, isHardLink, inode, device
        FROM video
        WHERE id IN (%s);
    `, placeholders)

	var videos []*models.Video
	err := sqlscan.Select(ctx, r.db, &videos, query, utils.ToInterfaceSlice(videoIDs)...)
	if err != nil {
		return nil, fmt.Errorf("querying videos by IDs: %w", err)
	}

	return videos, nil
}

func (r *videoRepo) getScreenshotsByVideohashIDs(ctx context.Context, videohashIDs []int64) (map[int64]*models.Screenshots, error) {
	if len(videohashIDs) == 0 {
		return make(map[int64]*models.Screenshots), nil // No IDs to query
	}

	// Create placeholders for the query
	placeholders := strings.Repeat("?,", len(videohashIDs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query := fmt.Sprintf(`
        SELECT id, screenshots, FK_screenshot_videohash
        FROM screenshot
        WHERE FK_screenshot_videohash IN (%s);
    `, placeholders)

	var rows []struct {
		ID                    int64  `db:"id"`
		Screenshots           string `db:"screenshots"`
		FKScreenshotVideohash int64  `db:"FK_screenshot_videohash"`
	}

	err := sqlscan.Select(ctx, r.db, &rows, query, int64ToInterfaceSlice(videohashIDs)...)
	if err != nil {
		return nil, fmt.Errorf("querying screenshots by videohash IDs: %w", err)
	}

	// Map screenshots to their corresponding videohash IDs
	screenshotMap := make(map[int64]*models.Screenshots)
	for _, row := range rows {
		var base64Strings []string
		// Decode JSON into Base64 strings
		if err := json.Unmarshal([]byte(row.Screenshots), &base64Strings); err != nil {
			return nil, fmt.Errorf("unmarshaling screenshots for videohash ID %d: %w", row.FKScreenshotVideohash, err)
		}

		// Convert Base64 strings to []image.Image
		var images []image.Image
		for _, b64 := range base64Strings {
			data, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return nil, fmt.Errorf("decoding Base64 for videohash ID %d: %w", row.FKScreenshotVideohash, err)
			}
			img, err := bmp.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("decoding BMP for videohash ID %d: %w", row.FKScreenshotVideohash, err)
			}
			images = append(images, img)
		}

		// Add to the map
		screenshotMap[row.FKScreenshotVideohash] = &models.Screenshots{
			ID:                    row.ID,
			Screenshots:           images,
			FKScreenshotVideohash: row.FKScreenshotVideohash,
		}
	}

	return screenshotMap, nil
}

func uniqueInt64Slice(slice []int64) []int64 {
	m := make(map[int64]struct{})
	var result []int64
	for _, s := range slice {
		if _, exists := m[s]; !exists {
			m[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

func getUniqueIDsFromVideohashes(videohashes []*models.Videohash) []int64 {
	idSet := make(map[int64]struct{})
	for _, vh := range videohashes {
		idSet[vh.FKVideohashVideo] = struct{}{}
	}

	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	return ids
}

func int64ToInterfaceSlice(ids []int64) []interface{} {
	result := make([]interface{}, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}
