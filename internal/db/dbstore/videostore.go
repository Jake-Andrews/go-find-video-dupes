package dbstore

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	store "govdupes/internal/db"
	"govdupes/internal/models"
)

type videoRepo struct {
	db *sql.DB
}

func NewVideoStore(DB *sql.DB) store.VideoStore {
	return &videoRepo{
		db: DB,
	}
}

func (r *videoRepo) CreateVideo(ctx context.Context, video *models.Video, hashes *[]models.Videohash) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	sqlRes, err := tx.ExecContext(ctx, `
		INSERT INTO video (
			path, fileName, createdAt, modifiedAt, frameRate, videoCodec,
			audioCodec, width, height, duration, size, bitRate
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		video.Path, video.FileName,
		video.CreatedAt.Format(time.RFC3339), video.ModifiedAt.Format(time.RFC3339),
		video.FrameRate, video.VideoCodec, video.AudioCodec,
		video.Width, video.Height, int(video.Duration.Seconds()),
		video.Size, video.BitRate,
	)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("insert video: %w", err)
	}

	videoID, err := sqlRes.LastInsertId()
	if err != nil {
		log.Fatalf("Error, trying to get last sql insert id, error: %v", err)
	}
	log.Println(videoID)

	for _, hash := range *hashes {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO videohash (videoID, value, hashType) VALUES (?, ?, ?)`,
			videoID, hash.Value, hash.HashType,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert hash: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *videoRepo) GetVideo(ctx context.Context, videoPath string) (*models.Video, []models.Videohash, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT videoID, path, fileName, createdAt, modifiedAt, frameRate, videoCodec, audioCodec, width, height, duration, size, bitRate
		FROM video WHERE path = ?`, videoPath)

	var video models.Video
	var duration int
	err := row.Scan(
		&video.VideoID, &video.Path, &video.FileName, &video.CreatedAt, &video.ModifiedAt,
		&video.FrameRate, &video.VideoCodec, &video.AudioCodec, &video.Width, &video.Height,
		&duration, &video.Size, &video.BitRate,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("video not found")
		}
		return nil, nil, fmt.Errorf("get video: %w", err)
	}
	// Convert duration to time.Duration
	video.Duration = time.Duration(duration) * time.Second

	// Query for associated video hashes
	rows, err := r.db.QueryContext(ctx, `
		SELECT value, hashType FROM videohash WHERE videoID = ?`, video.VideoID)
	if err != nil {
		return nil, nil, fmt.Errorf("get hashes: %w", err)
	}
	defer rows.Close()

	// Collect hashes in a separate slice
	var hashes []models.Videohash
	for rows.Next() {
		var hash models.Videohash
		if err := rows.Scan(&hash.Value, &hash.HashType); err != nil {
			return nil, nil, fmt.Errorf("scan hash: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	return &video, hashes, nil
}

func (r *videoRepo) GetVideos(ctx context.Context) ([]*models.Video, map[int64][]models.Videohash, error) {
	// Query to retrieve all videos
	rows, err := r.db.QueryContext(ctx, `
		SELECT videoID, path, fileName, createdAt, modifiedAt, frameRate, videoCodec, audioCodec, width, height, duration, size, bitRate
		FROM video`)
	if err != nil {
		return nil, nil, fmt.Errorf("get videos: %w", err)
	}
	defer rows.Close()

	// Slice to hold all videos
	var videos []*models.Video

	for rows.Next() {
		var video models.Video
		var duration int
		if err := rows.Scan(
			&video.VideoID, &video.Path, &video.FileName, &video.CreatedAt, &video.ModifiedAt,
			&video.FrameRate, &video.VideoCodec, &video.AudioCodec, &video.Width, &video.Height,
			&duration, &video.Size, &video.BitRate,
		); err != nil {
			return nil, nil, fmt.Errorf("scan video: %w", err)
		}
		video.Duration = time.Duration(duration) * time.Second
		videos = append(videos, &video)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	// Map to hold video hashes, keyed by VideoID
	hashes := make(map[int64][]models.Videohash)

	// Query to retrieve all video hashes
	hashRows, err := r.db.QueryContext(ctx, `
		SELECT videoID, value, hashType FROM videohash`)
	if err != nil {
		return nil, nil, fmt.Errorf("get video hashes: %w", err)
	}
	defer hashRows.Close()

	for hashRows.Next() {
		var hash models.Videohash
		if err := hashRows.Scan(&hash.VideoID, &hash.Value, &hash.HashType); err != nil {
			return nil, nil, fmt.Errorf("scan hash: %w", err)
		}
		hashes[hash.VideoID] = append(hashes[hash.VideoID], hash)
	}
	if err := hashRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("hash rows error: %w", err)
	}

	return videos, hashes, nil
}
