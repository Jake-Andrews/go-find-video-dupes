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
	"reflect"
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

func NewVideoStore(DB *sql.DB) store.VideoStore {
	return &videoRepo{
		db: DB,
	}
}

// GetDuplicateVideoData returns video-hash groups that share the same bucket (bucket != -1).
// This reflects "duplicate groups" or related items.
func (r *videoRepo) GetDuplicateVideoData(ctx context.Context) ([][]*models.VideoData, error) {
	// 1) Fetch all videohashes with a valid bucket (bucket != -1)
	var videohashes []*models.Videohash
	query := `
        SELECT id, hashValue, hashType, duration, neighbours, bucket
        FROM videohash
        WHERE bucket != -1;
    `
	if err := sqlscan.Select(ctx, r.db, &videohashes, query); err != nil {
		return nil, fmt.Errorf("querying videohashes: %w", err)
	}

	if len(videohashes) == 0 {
		// No matching videohashes, so return empty
		return nil, nil
	}

	// 2) For each videohash, get all videos referencing it
	hashIDs := make([]int64, 0, len(videohashes))
	for _, vh := range videohashes {
		hashIDs = append(hashIDs, vh.ID)
	}

	// Retrieve videos grouped by videohash ID
	videosByHashID, err := r.getVideosByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related videos: %w", err)
	}

	// 3) Fetch screenshots for these videohash IDs
	screenshots, err := r.getScreenshotsByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related screenshots: %w", err)
	}

	// 4) Combine data into a set of groups keyed by bucket
	groupedVideoData := map[int][]*models.VideoData{}

	for _, vh := range videohashes {
		// For each videohash, find the associated videos
		videosForHash := videosByHashID[vh.ID]

		// If no screenshots found, we create an empty record
		sc, hasScreenshot := screenshots[vh.ID]
		if !hasScreenshot {
			sc = &models.Screenshots{}
		}

		// For each video referencing this hash, build a VideoData
		for _, vid := range videosForHash {
			videoData := &models.VideoData{
				Video:      *vid,
				Videohash:  *vh,
				Screenshot: *sc,
			}
			groupedVideoData[vh.Bucket] = append(groupedVideoData[vh.Bucket], videoData)
		}

		// If no videos found at all, at least store the hash + screenshot info
		if len(videosForHash) == 0 {
			videoData := &models.VideoData{
				Videohash:  *vh,
				Screenshot: *sc,
			}
			groupedVideoData[vh.Bucket] = append(groupedVideoData[vh.Bucket], videoData)
		}
	}

	// Convert grouped map into a slice of slices
	result := make([][]*models.VideoData, 0, len(groupedVideoData))
	for _, group := range groupedVideoData {
		result = append(result, group)
	}
	return result, nil
}

// GetVideosWithValidHashes retrieves videos that have valid hashes (bucket != -1).
// Because now video references videohash, we JOIN on video.FK_video_videohash = videohash.id.
func (r *videoRepo) GetVideosWithValidHashes(ctx context.Context) ([]models.Video, error) {
	query := `
		SELECT DISTINCT v.*
		FROM video v
		INNER JOIN videohash vh ON v.FK_video_videohash = vh.id
		WHERE vh.bucket != ?;
	`
	var videos []models.Video
	if err := sqlscan.Select(ctx, r.db, &videos, query, -1); err != nil {
		return nil, fmt.Errorf("querying videos: %w", err)
	}

	return videos, nil
}

// GetScreenshotsForValidHashes retrieves screenshots for valid (bucket != -1) video hashes.
func (r *videoRepo) GetScreenshotsForValidHashes(ctx context.Context) (map[int64]models.Screenshots, error) {
	// Step 1: Query all *videohash IDs* that are valid
	query := `
		SELECT vh.id
		FROM videohash vh
		WHERE vh.bucket != ?;
	`
	var hashIDs []int64
	if err := sqlscan.Select(ctx, r.db, &hashIDs, query, -1); err != nil {
		return nil, fmt.Errorf("querying valid videohash IDs: %w", err)
	}

	if len(hashIDs) == 0 {
		return make(map[int64]models.Screenshots), nil
	}

	// Step 2: Fetch screenshots by these videohash IDs
	placeholders := strings.Repeat("?,", len(hashIDs))
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
	if err := sqlscan.Select(ctx, r.db, &rawScreenshots, query, utils.ToInterfaceSlice(hashIDs)...); err != nil {
		return nil, fmt.Errorf("query raw screenshots: %w", err)
	}

	// Step 3: Decode screenshots into a map keyed by videohash ID
	screenshotsMap := make(map[int64]models.Screenshots)
	for _, row := range rawScreenshots {
		var base64Strings []string

		if err := json.Unmarshal([]byte(row.Screenshots), &base64Strings); err != nil {
			return nil, fmt.Errorf("unmarshaling screenshots for videohash ID %d: %w", row.VideohashID, err)
		}

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

		screenshotsMap[row.VideohashID] = models.Screenshots{
			ID:                    row.VideohashID,
			Screenshots:           images,
			FKScreenshotVideohash: row.VideohashID,
		}
	}

	return screenshotsMap, nil
}

// CreateVideo inserts a new videohash, then a new video referencing that hash,
// then screenshots referencing that same hash.
func (r *videoRepo) CreateVideo(ctx context.Context, video *models.Video, hash *models.Videohash, sc *models.Screenshots) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1) Insert the videohash record
	neighboursJSON, err := json.Marshal(hash.Neighbours)
	if err != nil {
		return fmt.Errorf("marshal neighbours: %w", err)
	}

	hashInsert := `
		INSERT INTO videohash (hashValue, hashType, duration, neighbours, bucket)
		VALUES (?, ?, ?, ?, ?);
	`
	hashResult, err := tx.ExecContext(ctx, hashInsert,
		hash.HashValue,
		hash.HashType,
		hash.Duration,
		string(neighboursJSON),
		hash.Bucket,
	)
	if err != nil {
		return fmt.Errorf("insert hash: %w", err)
	}
	hashID, err := hashResult.LastInsertId()
	if err != nil {
		return fmt.Errorf("retrieve hash ID: %w", err)
	}

	// 2) Now attach that videohash to the video
	video.FKVideoVideohash = hashID

	// 3) Insert the video referencing the new videohash
	cols, placeholders, vals, err := buildInsertQueryAndValues(video)
	if err != nil {
		return fmt.Errorf("build insert data for video: %w", err)
	}
	insertVideoQuery := fmt.Sprintf(
		"INSERT INTO video (%s) VALUES (%s);",
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)
	result, err := tx.ExecContext(ctx, insertVideoQuery, vals...)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}
	videoID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("retrieve video ID: %w", err)
	}
	video.ID = videoID

	// 4) Insert screenshot referencing the same videohash
	base64Images, err := sc.EncodeImages()
	if err != nil {
		return fmt.Errorf("error encoding images to base64, err: %v", err)
	}
	jsonImages, err := json.Marshal(base64Images)
	if err != nil {
		return fmt.Errorf("marshal base64 images: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO screenshot (FK_screenshot_videohash, screenshots) VALUES (?, ?);`,
		hashID, string(jsonImages),
	); err != nil {
		return fmt.Errorf("insert screenshot: %w", err)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit transaction: %w", commitErr)
	}
	return nil
}

// GetVideo fetches a single video by file path, along with its (single) videohash.
// Because the video references the hash via FK_video_videohash, we look up the hash by that ID.
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

	if video.FKVideoVideohash == 0 {
		// No videohash assigned to this video
		return &video, nil, nil
	}

	var hash models.Videohash
	err = sqlscan.Get(ctx, r.db, &hash, `
		SELECT *
		FROM videohash
		WHERE id = ?;
	`, video.FKVideoVideohash)
	if err != nil {
		if err == sql.ErrNoRows {
			// No hash record found
			return &video, nil, nil
		}
		return nil, nil, fmt.Errorf("error retrieving video hash: %w", err)
	}

	return &video, &hash, nil
}

// GetAllVideos returns all videos in the DB.
func (r *videoRepo) GetAllVideos(ctx context.Context) ([]*models.Video, error) {
	var videos []*models.Video
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

// GetAllVideoHashes returns all videohashes in the DB.
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

// BulkUpdateVideohashes updates multiple videohash rows in a single transaction.
func (r *videoRepo) BulkUpdateVideohashes(ctx context.Context, updates []*models.Videohash) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error starting transaction: %v", err)
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Panic during transaction, rolling back: %v", p)
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			log.Printf("Error detected, rolling back transaction: %v", err)
			_ = tx.Rollback()
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

	for _, vh := range updates {
		neighboursJSON, err := json.Marshal(vh.Neighbours)
		if err != nil {
			log.Printf("Error marshalling neighbours for videohash ID %d: %v", vh.ID, err)
			_ = tx.Rollback()
			return fmt.Errorf("marshal neighbours for videohash ID %d: %w", vh.ID, err)
		}
		log.Printf("Updating videohash ID %d with hashType: %s, hashValue: %s, duration: %.2f, bucket: %d",
			vh.ID, vh.HashType, vh.HashValue, vh.Duration, vh.Bucket)

		_, err = stmt.ExecContext(ctx,
			vh.HashType,
			vh.HashValue,
			vh.Duration,
			vh.Bucket,
			string(neighboursJSON),
			vh.ID,
		)
		if err != nil {
			log.Printf("Error updating videohash ID %d: %v", vh.ID, err)
			_ = tx.Rollback()
			return fmt.Errorf("update videohash ID %d: %w", vh.ID, err)
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		log.Printf("Error committing transaction: %v", commitErr)
		return fmt.Errorf("commit transaction: %w", commitErr)
	}

	log.Println("Successfully updated videohash records.")
	return nil
}

// getVideosByVideohashIDs returns all videos that reference any of the given videohash IDs.
func (r *videoRepo) getVideosByVideohashIDs(ctx context.Context, hashIDs []int64) (map[int64][]*models.Video, error) {
	if len(hashIDs) == 0 {
		return make(map[int64][]*models.Video), nil
	}

	placeholders := strings.Repeat("?,", len(hashIDs))
	placeholders = placeholders[:len(placeholders)-1]

	// We fetch all videos referencing these hash IDs
	query := fmt.Sprintf(`
		SELECT *
		FROM video
		WHERE FK_video_videohash IN (%s);
	`, placeholders)

	var allVideos []*models.Video
	if err := sqlscan.Select(ctx, r.db, &allVideos, query, utils.ToInterfaceSlice(hashIDs)...); err != nil {
		return nil, fmt.Errorf("querying videos by videohash IDs: %w", err)
	}

	// Group the videos by their FK_video_videohash
	result := make(map[int64][]*models.Video)
	for _, vid := range allVideos {
		result[vid.FKVideoVideohash] = append(result[vid.FKVideoVideohash], vid)
	}
	return result, nil
}

// getScreenshotsByVideohashIDs returns screenshots keyed by videohash ID.
func (r *videoRepo) getScreenshotsByVideohashIDs(ctx context.Context, videohashIDs []int64) (map[int64]*models.Screenshots, error) {
	if len(videohashIDs) == 0 {
		return make(map[int64]*models.Screenshots), nil // No IDs to query
	}

	placeholders := strings.Repeat("?,", len(videohashIDs))
	placeholders = placeholders[:len(placeholders)-1]

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
	if err := sqlscan.Select(ctx, r.db, &rows, query, int64ToInterfaceSlice(videohashIDs)...); err != nil {
		return nil, fmt.Errorf("querying screenshots by videohash IDs: %w", err)
	}

	screenshotMap := make(map[int64]*models.Screenshots)
	for _, row := range rows {
		var base64Strings []string
		if err := json.Unmarshal([]byte(row.Screenshots), &base64Strings); err != nil {
			return nil, fmt.Errorf("unmarshaling screenshots for videohash ID %d: %w", row.FKScreenshotVideohash, err)
		}

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

		screenshotMap[row.FKScreenshotVideohash] = &models.Screenshots{
			ID:                    row.ID,
			Screenshots:           images,
			FKScreenshotVideohash: row.FKScreenshotVideohash,
		}
	}

	return screenshotMap, nil
}

// int64ToInterfaceSlice helps us build the query args for a variable list of IDs
func int64ToInterfaceSlice(ids []int64) []interface{} {
	result := make([]interface{}, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}

// buildInsertQueryAndValues generates an INSERT statement's column, placeholders, and values
// from a struct 'v' that has `db:"columnName"` tags. It skips empty db tags and the "id" field.
func buildInsertQueryAndValues(v interface{}) ([]string, []string, []interface{}, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, nil, nil, fmt.Errorf("expected a struct, got %T", v)
	}

	var (
		columns      []string
		placeholders []string
		values       []interface{}
	)

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		fieldType := rt.Field(i)
		dbTag := fieldType.Tag.Get("db")

		// Skip untagged fields
		if dbTag == "" {
			continue
		}
		// Skip the primary key if it's auto-increment
		if dbTag == "id" {
			continue
		}

		columns = append(columns, dbTag)
		placeholders = append(placeholders, "?")
		values = append(values, rv.Field(i).Interface())
	}
	return columns, placeholders, values, nil
}

