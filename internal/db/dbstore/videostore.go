package dbstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"log/slog"
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
// GetDuplicateVideoData now selects all videos,
// then loads each video’s videohash (if any), plus screenshots for that hash.
func (r *videoRepo) GetDuplicateVideoData(ctx context.Context) ([][]*models.VideoData, error) {
	// 1) Fetch all videohashes
	var videohashes []*models.Videohash
	query := `
        SELECT *
        FROM videohash
    `
	if err := sqlscan.Select(ctx, r.db, &videohashes, query); err != nil {
		return nil, fmt.Errorf("querying videohashes: %v", err)
	}
	if len(videohashes) == 0 {
		return nil, nil // no videohashes => empty
	}

	// Collect videohash IDs
	hashIDs := make([]int64, 0, len(videohashes))
	for _, vh := range videohashes {
		hashIDs = append(hashIDs, vh.ID)
	}

	// 2) Fetch related videos for each videohash
	videosByHashID, err := r.GetVideosByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related videos: %w", err)
	}

	// 3) Fetch screenshots keyed by videohash ID
	screenshotMap, err := r.getScreenshotsByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching screenshots: %w", err)
	}

	// 4) Group everything by bucket, ensuring all combinations are created
	groupedByBucket := make(map[int][]*models.VideoData)

	for _, vh := range videohashes {
		if vh.Bucket == -1 {
			continue
		}
		vids := videosByHashID[vh.ID]
		sc := screenshotMap[vh.ID]
		if sc == nil {
			sc = &models.Screenshots{} // placeholder if no screenshots
		}

		// Create VideoData for each video, ensuring redundancy
		if len(vids) > 0 {
			for _, vid := range vids {
				groupedByBucket[vh.Bucket] = append(groupedByBucket[vh.Bucket], &models.VideoData{
					Video:      *vid,
					Videohash:  *vh,
					Screenshot: *sc,
				})
			}
		} else {
			// If no videos reference this hash, still store the hash + screenshot
			groupedByBucket[vh.Bucket] = append(groupedByBucket[vh.Bucket], &models.VideoData{
				Videohash:  *vh,
				Screenshot: *sc,
			})
		}
	}

	// 5) Convert map => slice of slices
	//    each sub-slice = all videos that share the same bucket
	var result [][]*models.VideoData
	for _, group := range groupedByBucket {
		if len(group) >= 2 {
			result = append(result, group)
		}
	}

	return result, nil
}

func (r *videoRepo) GetVideosByVideohashIDs(ctx context.Context, hashIDs []int64) (map[int64][]*models.Video, error) {
	// Ensure there are hashIDs to query
	if len(hashIDs) == 0 {
		return nil, nil
	}

	// Generate placeholders for the IN clause
	placeholders := make([]string, len(hashIDs))
	args := make([]interface{}, len(hashIDs))
	for i, id := range hashIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	// Construct the query
	query := fmt.Sprintf(`
        SELECT *
        FROM video
        WHERE FK_video_videohash IN (%s)
    `, strings.Join(placeholders, ", "))

	// Execute the query
	var videos []*models.Video
	if err := sqlscan.Select(ctx, r.db, &videos, query, args...); err != nil {
		return nil, fmt.Errorf("querying videos by videohash IDs: %w", err)
	}

	// Group videos by their FK_video_videohash
	videosByHashID := make(map[int64][]*models.Video)
	for _, video := range videos {
		videosByHashID[video.FKVideoVideohash] = append(videosByHashID[video.FKVideoVideohash], video)
	}

	return videosByHashID, nil
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

	// Step 1: Check if the videohash already exists
	var existingHashID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM videohash
		WHERE hashValue = ? AND hashType = ? AND duration = ?;
	`, hash.HashValue, hash.HashType, hash.Duration).Scan(&existingHashID)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error checking for existing hash: %w", err)
	}

	if err == sql.ErrNoRows {
		// Step 2: Insert the videohash record if it doesn't exist
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
		existingHashID, err = hashResult.LastInsertId()
		if err != nil {
			return fmt.Errorf("retrieve hash ID: %w", err)
		}
	}

	// Step 3: Now attach the existing/new videohash to the video
	video.FKVideoVideohash = existingHashID

	// Step 4: Insert the video referencing the existing/new videohash
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

	// Step 5: Insert screenshot referencing the same videohash
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
		existingHashID, string(jsonImages),
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
	if len(updates) == 0 {
		slog.Info("No updates provided for videohash records")
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("Error starting transaction", slog.String("error", err.Error()))
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			slog.Error("Panic during transaction, rolling back", slog.Any("panic", p))
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
			slog.Error("Transaction rolled back due to error", slog.String("error", err.Error()))
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE videohash
		SET hashType = ?, hashValue = ?, duration = ?, bucket = ?, neighbours = ?
		WHERE id = ?;
	`)
	if err != nil {
		slog.Error("Error preparing update statement", slog.String("error", err.Error()))
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, vh := range updates {
		if vh == nil {
			slog.Warn("Skipping nil videohash record in updates")
			continue
		}

		neighboursJSON, err := json.Marshal(vh.Neighbours)
		if err != nil {
			slog.Error("Error marshalling neighbours", slog.Int64("videohashID", vh.ID), slog.String("error", err.Error()))
			return fmt.Errorf("marshal neighbours for videohash ID %d: %w", vh.ID, err)
		}

		slog.Debug("Updating videohash",
			slog.Int64("videohashID", vh.ID),
			slog.String("hashType", string(vh.HashType)),
			slog.String("hashValue", vh.HashValue),
			slog.Float64("duration", float64(vh.Duration)),
			slog.Int("bucket", vh.Bucket),
			slog.String("neighbours", string(neighboursJSON)),
		)

		_, err = stmt.ExecContext(ctx,
			vh.HashType,
			vh.HashValue,
			vh.Duration,
			vh.Bucket,
			string(neighboursJSON),
			vh.ID,
		)
		if err != nil {
			slog.Error("Error updating videohash", slog.Int64("videohashID", vh.ID), slog.String("error", err.Error()))
			return fmt.Errorf("update videohash ID %d: %w", vh.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Error committing transaction", slog.String("error", err.Error()))
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("Successfully updated videohash records", slog.Int("count", len(updates)))
	return nil
}

// getVideosByVideohashIDs returns all videos that reference any of the given videohash IDs.
// getVideohashesByIDs returns a map of videohashID -> *Videohash
func (r *videoRepo) getVideohashesByIDs(ctx context.Context, ids []int64) (map[int64]*models.Videohash, error) {
	if len(ids) == 0 {
		return make(map[int64]*models.Videohash), nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	query := fmt.Sprintf(`
        SELECT id, hashType, hashValue, duration, neighbours, bucket
        FROM videohash
        WHERE id IN (%s)
    `, placeholders)

	var allHashes []*models.Videohash
	if err := sqlscan.Select(ctx, r.db, &allHashes, query, utils.ToInterfaceSlice(ids)...); err != nil {
		return nil, fmt.Errorf("fetching videohashes by IDs: %w", err)
	}

	// Build a map from ID => Videohash
	hashMap := make(map[int64]*models.Videohash)
	for _, vh := range allHashes {
		hashMap[vh.ID] = vh
	}
	return hashMap, nil
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
