package store

import (
	"context"

	"govdupes/internal/models"
)

type VideoStore interface {
	CreateVideo(ctx context.Context, video *models.Video, hash []models.Videohash) error
	GetVideo(ctx context.Context, videoPath string) (*models.Video, []models.Videohash, error)
	GetVideos(ctx context.Context) ([]models.Video, map[int64][]models.Videohash)
}
