package dbstore

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *videoRepo) CreateVideo(ctx context.Context, video *models.Video, hashes []*models.Videohash) error {
	log.Printf("Creating video, path: %q\n", video.Path)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	sqlRes, err := tx.ExecContext(ctx, `
		INSERT INTO video (
			path, fileName, createdAt, modifiedAt, frameRate, videoCodec,
			audioCodec, width, height, duration, size, bitRate,
			numHardLinks, symbolicLink, isSymbolicLink, isHardLink, inode, device
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		video.Path, video.FileName,
		video.CreatedAt.Format(time.RFC3339), video.ModifiedAt.Format(time.RFC3339),
		video.FrameRate, video.VideoCodec, video.AudioCodec,
		video.Width, video.Height, video.Duration,
		video.Size, video.BitRate,
		video.NumHardLinks, video.SymbolicLink, video.IsSymbolicLink,
		video.IsHardLink, video.Inode, video.Device,
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

	for _, hash := range hashes {
		neighboursJSON, err := json.Marshal(hash.Neighbours)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("encode neighbours: %w", err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO videohash (videoID, hashType, hashValue, duration, neighbours, bucket) 
			VALUES (?, ?, ?, ?, ?, ?)`,
			videoID, hash.HashType, hash.HashValue, hash.Duration, string(neighboursJSON), hash.Bucket,
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
		SELECT videoID, path, fileName, createdAt, modifiedAt, frameRate, videoCodec, audioCodec,
		       width, height, duration, size, bitRate, numHardLinks, symbolicLink, 
		       isSymbolicLink, isHardLink, inode, device
		FROM video WHERE path = ?`, videoPath)

	var video models.Video
	var duration float32
	err := row.Scan(
		&video.VideoID, &video.Path, &video.FileName, &video.CreatedAt, &video.ModifiedAt,
		&video.FrameRate, &video.VideoCodec, &video.AudioCodec, &video.Width, &video.Height,
		&duration, &video.Size, &video.BitRate,
		&video.NumHardLinks, &video.SymbolicLink, &video.IsSymbolicLink,
		&video.IsHardLink, &video.Inode, &video.Device,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("video not found")
		}
		return nil, nil, fmt.Errorf("get video: %w", err)
	}
	video.Duration = duration

	rows, err := r.db.QueryContext(ctx, `
		SELECT videohashID, videoID, hashType, hashValue, duration, neighbours, bucket 
		FROM videohash WHERE videoID = ?`, video.VideoID)
	if err != nil {
		return nil, nil, fmt.Errorf("get hashes: %w", err)
	}
	defer rows.Close()

	var hashes []models.Videohash
	for rows.Next() {
		var hash models.Videohash
		var neighboursJSON string
		if err := rows.Scan(&hash.VideohashID, &hash.VideoID, &hash.HashType, &hash.HashValue, &hash.Duration, &neighboursJSON, &hash.Bucket); err != nil {
			return nil, nil, fmt.Errorf("scan hash: %w", err)
		}
		if err := json.Unmarshal([]byte(neighboursJSON), &hash.Neighbours); err != nil {
			return nil, nil, fmt.Errorf("decode neighbours: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("rows error: %w", err)
	}

	return &video, hashes, nil
}

func (r *videoRepo) GetVideos(ctx context.Context) ([]*models.Video, []*models.Videohash, error) {
	log.Println("Getting videos from the database!")
	rows, err := r.db.QueryContext(ctx, `
		SELECT videoID, path, fileName, createdAt, modifiedAt, frameRate, videoCodec, audioCodec,
		       width, height, duration, size, bitRate, numHardLinks, symbolicLink, 
		       isSymbolicLink, isHardLink, inode, device
		FROM video`)
	if err != nil {
		return nil, nil, fmt.Errorf("get videos: %w", err)
	}
	defer rows.Close()

	var videos []*models.Video

	for rows.Next() {
		var video models.Video
		var duration float32
		if err := rows.Scan(
			&video.VideoID, &video.Path, &video.FileName, &video.CreatedAt, &video.ModifiedAt,
			&video.FrameRate, &video.VideoCodec, &video.AudioCodec, &video.Width, &video.Height,
			&duration, &video.Size, &video.BitRate,
			&video.NumHardLinks, &video.SymbolicLink, &video.IsSymbolicLink,
			&video.IsHardLink, &video.Inode, &video.Device,
		); err != nil {
			return nil, nil, fmt.Errorf("scan video: %w", err)
		}
		video.Duration = duration
		videos = append(videos, &video)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("video rows error: %w", err)
	}

	var hashes []*models.Videohash

	hashRows, err := r.db.QueryContext(ctx, `
		SELECT videohashID, videoID, hashType, hashValue, duration, neighbours, bucket 
		FROM videohash ORDER BY videohashID`)
	if err != nil {
		return nil, nil, fmt.Errorf("get hashes: %w", err)
	}
	defer hashRows.Close()

	for hashRows.Next() {
		var hash models.Videohash
		var neighboursJSON string
		if err := hashRows.Scan(&hash.VideohashID, &hash.VideoID, &hash.HashType, &hash.HashValue, &hash.Duration, &neighboursJSON, &hash.Bucket); err != nil {
			return nil, nil, fmt.Errorf("scan hash: %w", err)
		}
		if err := json.Unmarshal([]byte(neighboursJSON), &hash.Neighbours); err != nil {
			return nil, nil, fmt.Errorf("decode neighbours: %w", err)
		}
		hashes = append(hashes, &hash)
	}
	if err := hashRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("hash rows error: %w", err)
	}

	log.Println("Finished getting videos from the database!")
	return videos, hashes, nil
}
