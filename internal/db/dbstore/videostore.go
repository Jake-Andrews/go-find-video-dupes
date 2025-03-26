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

func (r *videoRepo) GetDuplicateVideoData(ctx context.Context) ([][]*models.VideoData, error) {
	var videohashes []*models.Videohash
	query := `
        SELECT *
        FROM videohash
    `
	if err := sqlscan.Select(ctx, r.db, &videohashes, query); err != nil {
		return nil, fmt.Errorf("querying videohashes: %v", err)
	}
	if len(videohashes) == 0 {
		return nil, nil
	}

	hashIDs := make([]int64, 0, len(videohashes))
	for _, vh := range videohashes {
		hashIDs = append(hashIDs, vh.ID)
	}

	// Fetch videos keyed by videohash IDs
	videosByHashID, err := r.GetVideosByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching related videos: %w", err)
	}

	// Fetch screenshots keyed by videohash IDs
	screenshotMap, err := r.getScreenshotsByVideohashIDs(ctx, hashIDs)
	if err != nil {
		return nil, fmt.Errorf("fetching screenshots: %w", err)
	}

	// Group everything by bucket (duplicate groups)
	groupedByBucket := make(map[int][]*models.VideoData)

	for _, vh := range videohashes {
		if vh.Bucket == -1 {
			continue
		}
		vids := videosByHashID[vh.ID]
		sc := screenshotMap[vh.ID]
		if sc == nil {
			sc = &models.Screenshots{}
		}

		// redundancy
		if len(vids) > 0 {
			for _, vid := range vids {
				groupedByBucket[vh.Bucket] = append(groupedByBucket[vh.Bucket], &models.VideoData{
					Video:      *vid,
					Videohash:  *vh,
					Screenshot: *sc,
				})
			}
		} else {
			// if no videos reference this hash, still store the hash + screenshot
			groupedByBucket[vh.Bucket] = append(groupedByBucket[vh.Bucket], &models.VideoData{
				Videohash:  *vh,
				Screenshot: *sc,
			})
		}
	}

	var result [][]*models.VideoData
	for _, group := range groupedByBucket {
		if len(group) >= 2 {
			result = append(result, group)
		}
	}

	return result, nil
}

func (r *videoRepo) GetVideosByVideohashIDs(ctx context.Context, hashIDs []int64) (map[int64][]*models.Video, error) {
	if len(hashIDs) == 0 {
		return nil, nil
	}

	// placeholders for the IN clause
	placeholders := make([]string, len(hashIDs))
	args := make([]any, len(hashIDs))
	for i, id := range hashIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
        SELECT *
        FROM video
        WHERE FK_video_videohash IN (%s)
    `, strings.Join(placeholders, ", "))

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
// Video references videohash JOIN on video.FK_video_videohash = videohash.id.
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
	// Query all valid *videohash IDs*
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

	// Fetch screenshots by these videohash IDs
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

	// Decode screenshots into a map keyed by videohash ID
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

	// Check if the videohash already exists
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
		// Insert the videohash record if it doesn't exist
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

	// Now attach the existing/new videohash to the video
	video.FKVideoVideohash = existingHashID

	// Insert the video referencing the existing/new videohash
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

	// Insert screenshot referencing the same videohash
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

func (r *videoRepo) BatchCreateVideos(ctx context.Context, videos []*models.VideoData) error {
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

	/*
		videoInsertQuery := `
			INSERT INTO video (
				path, fileName, createdAt, modifiedAt, videoCodec, audioCodec,
				width, height, duration, size, bitRate, numHardLinks,
				symbolicLink, isSymbolicLink, isHardLink, inode, device,
				sampleRateAvg, avgFrameRate, FK_video_videohash
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
		`
	*/

	videohashInsertQuery := `
		INSERT INTO videohash (hashValue, hashType, duration, neighbours, bucket)
		VALUES (?, ?, ?, ?, ?);
	`

	screenshotInsertQuery := `
		INSERT INTO screenshot (FK_screenshot_videohash, screenshots)
		VALUES (?, ?);
	`

	for _, videoData := range videos {
		video := videoData.Video
		videohash := videoData.Videohash
		screenshots := videoData.Screenshot

		// Insert the videohash record
		neighboursJSON, err := json.Marshal(videohash.Neighbours)
		if err != nil {
			return fmt.Errorf("marshal neighbours: %w", err)
		}

		hashResult, err := tx.ExecContext(ctx, videohashInsertQuery,
			videohash.HashValue, videohash.HashType, videohash.Duration,
			string(neighboursJSON), videohash.Bucket,
		)
		if err != nil {
			return fmt.Errorf("insert videohash: %w", err)
		}
		existingHashID, err := hashResult.LastInsertId()
		if err != nil {
			return fmt.Errorf("retrieve hash ID: %w", err)
		}

		// Attach hash ID to video
		video.FKVideoVideohash = existingHashID

		// Insert the video referencing the existing/new videohash
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
		// Insert screenshots
		base64Images, err := screenshots.EncodeImages()
		if err != nil {
			return fmt.Errorf("encode images: %w", err)
		}
		jsonImages, err := json.Marshal(base64Images)
		if err != nil {
			return fmt.Errorf("marshal images: %w", err)
		}
		_, err = tx.ExecContext(ctx, screenshotInsertQuery,
			existingHashID, string(jsonImages),
		)
		if err != nil {
			return fmt.Errorf("insert screenshots: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// GetVideo fetches a single video by file path along with its videohash.
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

// helps to build the query args for a variable list of IDs
func int64ToInterfaceSlice(ids []int64) []any {
	result := make([]any, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}

// buildInsertQueryAndValues generates an INSERT statement's column, placeholders, and values
// from a struct 'v' that has `db:"columnName"` tags. It skips empty db tags and the "id" field.
func buildInsertQueryAndValues(v any) ([]string, []string, []any, error) {
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
		values       []any
	)

	rt := rv.Type()
	for i := range rv.NumField() {
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

func (r *videoRepo) DeleteVideoByID(ctx context.Context, videoID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM video WHERE id = ?", videoID)
	return err
}

func (r *videoRepo) UpdateVideos(ctx context.Context, videos []*models.Video) error {
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

	for _, video := range videos {
		cols, _, vals, err := buildInsertQueryAndValues(video)
		if err != nil {
			return fmt.Errorf("build update data for video ID %d: %w", video.ID, err)
		}

		updateQuery := fmt.Sprintf(
			"UPDATE video SET %s WHERE id = ?;",
			strings.Join(func() []string {
				updates := make([]string, len(cols))
				for i, col := range cols {
					updates[i] = fmt.Sprintf("%s = ?", col)
				}
				return updates
			}(), ", "),
		)

		vals = append(vals, video.ID)

		_, err = tx.ExecContext(ctx, updateQuery, vals...)
		if err != nil {
			return fmt.Errorf("update video ID %d: %w", video.ID, err)
		}
	}

	// Check for orphaned videohashes
	hashQuery := `
		SELECT id
		FROM videohash
		WHERE id NOT IN (
			SELECT DISTINCT FK_video_videohash FROM video
		);
	`

	hashIDs := []int64{}
	rows, err := tx.QueryContext(ctx, hashQuery)
	if err != nil {
		return fmt.Errorf("query orphaned videohashes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var hashID int64
		if err := rows.Scan(&hashID); err != nil {
			return fmt.Errorf("scan orphaned hash ID: %w", err)
		}
		hashIDs = append(hashIDs, hashID)
	}

	if rows.Err() != nil {
		return fmt.Errorf("iterate orphaned hash rows: %w", rows.Err())
	}

	for _, hashID := range hashIDs {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM screenshot
			WHERE FK_screenshot_videohash = ?;
		`, hashID)
		if err != nil {
			return fmt.Errorf("delete screenshots for orphaned hash ID %d: %w", hashID, err)
		}

		_, err = tx.ExecContext(ctx, `
			DELETE FROM videohash
			WHERE id = ?;
		`, hashID)
		if err != nil {
			return fmt.Errorf("delete orphaned videohash ID %d: %w", hashID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
