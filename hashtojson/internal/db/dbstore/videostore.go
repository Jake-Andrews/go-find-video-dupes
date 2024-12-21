package dbstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"govdupes/internal/models"
	"govdupes/internal/utils"

	"github.com/georgysavva/scany/v2/sqlscan"

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

func (r *videoRepo) GetVideosWithValidHashes(ctx context.Context) ([]models.Video, error) {
	query := `
		SELECT DISTINCT v.*
		FROM video v
		INNER JOIN videohash vh ON v.videoID = vh.videoID
		WHERE vh.bucket != ?`

	var videos []models.Video
	if err := sqlscan.Select(ctx, r.db, &videos, query, -1); err != nil {
		return nil, fmt.Errorf("querying videos: %w", err)
	}

	return videos, nil
}

func (r *videoRepo) GetScreenshotsForValidHashes(ctx context.Context) (map[int64]models.Screenshots, error) {
	query := `
		SELECT DISTINCT v.videoID
		FROM video v
		INNER JOIN videohash vh ON v.videoID = vh.videoID
		WHERE vh.bucket != ?`

	var videoIDs []int64
	if err := sqlscan.Select(ctx, r.db, &videoIDs, query, -1); err != nil {
		return nil, fmt.Errorf("querying valid videoIDs: %w", err)
	}

	if len(videoIDs) == 0 {
		return make(map[int64]models.Screenshots), nil
	}

	placeholders := strings.Repeat("?,", len(videoIDs))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	query = fmt.Sprintf(
		`SELECT videoID, screenshot FROM screenshot WHERE videoID IN (%s)`,
		placeholders,
	)

	var rawScreenshots []struct {
		VideoID    int64  `db:"videoID"`
		Screenshot string `db:"screenshot"`
	}
	if err := sqlscan.Select(ctx, r.db, &rawScreenshots, query, utils.ToInterfaceSlice(videoIDs)...); err != nil {
		return nil, fmt.Errorf("query rawScreenshots: %w", err)
	}

	screenshotsMap := make(map[int64]models.Screenshots)
	for _, row := range rawScreenshots {
		var screenshots []string
		if err := json.Unmarshal([]byte(row.Screenshot), &screenshots); err != nil {
			return nil, fmt.Errorf("unmarshaling screenshots for videoID %d: %w", row.VideoID, err)
		}
		screenshotsMap[row.VideoID] = models.Screenshots{
			Screenshots: screenshots,
		}
	}

	return screenshotsMap, nil
}

func (r *videoRepo) CreateVideo(ctx context.Context, video *models.Video, hashes []*models.Videohash, sc *models.Screenshots) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		}
	}()

	// Insert into video table
	result, err := tx.ExecContext(ctx, `
		INSERT INTO video (
			path, fileName, createdAt, modifiedAt, frameRate, videoCodec,
			audioCodec, width, height, duration, size, bitRate,
			numHardLinks, symbolicLink, isSymbolicLink, isHardLink, inode, device
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		video.Path, video.FileName,
		video.CreatedAt, video.ModifiedAt,
		video.FrameRate, video.VideoCodec, video.AudioCodec,
		video.Width, video.Height, video.Duration,
		video.Size, video.BitRate,
		video.NumHardLinks, video.SymbolicLink, video.IsSymbolicLink,
		video.IsHardLink, video.Inode, video.Device,
	)
	if err != nil {
		return fmt.Errorf("insert video: %w", err)
	}

	videoID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	// Insert into videohash table
	for _, hash := range hashes {
		neighboursJSON, err := json.Marshal(hash.Neighbours)
		if err != nil {
			return fmt.Errorf("encode neighbours: %w", err)
		}

		vhResult, err := tx.ExecContext(ctx, `
			INSERT INTO videohash (videoID, hashType, hashValue, duration, neighbours, bucket)
			VALUES (?, ?, ?, ?, ?, ?)
		`,
			videoID, hash.HashType, hash.HashValue, hash.Duration, string(neighboursJSON), hash.Bucket,
		)
		if err != nil {
			return fmt.Errorf("insert hash: %w", err)
		}

		videohashID, err := vhResult.LastInsertId()
		if err != nil {
			return fmt.Errorf("get videohash last insert id: %w", err)
		}

		// Insert all screenshots associated with this videohash
		for _, screenshot := range sc.Screenshots {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO screenshot (videohashID, screenshots)
				VALUES (?, ?)
			`,
				videohashID, screenshot,
			)
			if err != nil {
				return fmt.Errorf("insert screenshot: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *videoRepo) GetVideo(ctx context.Context, videoPath string) (*models.Video, []models.Videohash, error) {
	var video models.Video
	if err := sqlscan.Get(ctx, r.db, &video, `
		SELECT *
		FROM video
		WHERE path = ?
	`, videoPath); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("video not found")
		}
		return nil, nil, fmt.Errorf("get video: %w", err)
	}

	var hashes []models.Videohash
	if err := sqlscan.Select(ctx, r.db, &hashes, `
		SELECT *
		FROM videohash
		WHERE videoID = ?
	`, video.VideoID); err != nil {
		return nil, nil, fmt.Errorf("get hashes: %w", err)
	}

	return &video, hashes, nil
}

func (r *videoRepo) GetAllVideos(ctx context.Context) ([]*models.Video, error) {
	var videos []*models.Video
	if err := sqlscan.Select(ctx, r.db, &videos, `
		SELECT *
		FROM video
		ORDER BY videoID
	`); err != nil {
		return nil, fmt.Errorf("get videos: %w", err)
	}

	return videos, nil
}

func (r *videoRepo) GetAllVideoHashes(ctx context.Context) ([]*models.Videohash, error) {
	var hashes []*models.Videohash
	if err := sqlscan.Select(ctx, r.db, &hashes, `
		SELECT *
		FROM videohash
		ORDER BY videohashID
	`); err != nil {
		return nil, fmt.Errorf("get hashes: %w", err)
	}

	return hashes, nil
}

func (r *videoRepo) BulkUpdateVideohashes(ctx context.Context, updates []models.Videohash) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE videohash
		SET hashType = ?, hashValue = ?, duration = ?, bucket = ?
		WHERE videohashID = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, vh := range updates {
		_, err = stmt.ExecContext(ctx, vh.HashType, vh.HashValue, vh.Duration, vh.Bucket, vh.VideohashID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("update videohashID %d: %w", vh.VideohashID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
