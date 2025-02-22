package store

import (
	"context"

	"govdupes/internal/models"
)

type VideoStore interface {
	CreateVideo(ctx context.Context, video *models.Video, hash *models.Videohash, sc *models.Screenshots) error
	UpdateVideos(ctx context.Context, videos []*models.Video) error
	BatchCreateVideos(ctx context.Context, videos []*models.VideoData) error
	GetVideo(ctx context.Context, videoPath string) (*models.Video, *models.Videohash, error)
	GetAllVideoHashes(ctx context.Context) ([]*models.Videohash, error)
	GetAllVideos(ctx context.Context) ([]*models.Video, error)
	BulkUpdateVideohashes(ctx context.Context, updates []*models.Videohash) error
	GetVideosWithValidHashes(ctx context.Context) ([]models.Video, error)
	GetScreenshotsForValidHashes(ctx context.Context) (map[int64]models.Screenshots, error)
	GetDuplicateVideoData(ctx context.Context) ([][]*models.VideoData, error)
	GetVideosByVideohashIDs(ctx context.Context, hashIDs []int64) (map[int64][]*models.Video, error)
	DeleteVideoByID(ctx context.Context, videoID int64) error
}
