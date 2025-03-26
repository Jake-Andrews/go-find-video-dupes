// duplicate.go
package duplicate

import (
	"fmt"
	"log/slog"
	"math"

	"govdupes/internal/models"
)

type DuplicateOptions struct {
	MaxDurationDiff int
	MaxHashDistance int
}

func FindVideoDuplicates(hashes []*models.Videohash) error {
	options := DuplicateOptions{MaxDurationDiff: 5, MaxHashDistance: 4}
	initializeBuckets(hashes)

	for i, video := range hashes {
		if video.Bucket == -1 {
			slog.Info("Video has no bucket, assigning to a bucket", slog.Int("index", i))
			assignToExistingOrNewBucket(video, i, hashes, options)
			slog.Debug("Assigned video", slog.Any("video", video))
		} else {
			slog.Debug("Video already has a bucket", slog.Int("index", i), slog.Any("video", video))
		}
	}

	logBuckets(hashes)
	return nil
}

func initializeBuckets(hashes []*models.Videohash) {
	slog.Info("Initializing buckets")
	for _, video := range hashes {
		video.Bucket = -1
		video.Neighbours = []int{}
	}
}

func logBuckets(hashes []*models.Videohash) {
	bucketMap := make(map[int][]int)
	for _, video := range hashes {
		if video.Bucket != -1 {
			bucketMap[video.Bucket] = append(bucketMap[video.Bucket], int(video.ID))
		}
	}

	for bucket, ids := range bucketMap {
		if len(ids) > 1 {
			slog.Info("Bucket found", slog.Int("bucket", bucket), slog.Any("video_ids", ids))
		}
	}
}

func assignToExistingOrNewBucket(video *models.Videohash, index int, hashes []*models.Videohash, options DuplicateOptions) {
	slog.Info("Assigning video to a bucket", slog.Int("index", index))
	neighbors := findNeighbors(index, hashes, options)
	video.Neighbours = neighbors

	for _, neighborIndex := range neighbors {
		neighbor := hashes[neighborIndex]
		if neighbor.Bucket != -1 {
			slog.Info("Found existing bucket for video", slog.Int("bucket", neighbor.Bucket), slog.Int("index", index))
			video.Bucket = neighbor.Bucket
			break
		}
	}

	if video.Bucket == -1 {
		video.Bucket = findNextBucketID(hashes)
		slog.Info("No existing bucket found, assigning new bucket", slog.Int("index", index), slog.Int("bucket", video.Bucket))
	}

	propagateBucket(video.Bucket, neighbors, hashes)
}

func findNeighbors(index int, hashes []*models.Videohash, options DuplicateOptions) []int {
	var neighbors []int
	currentVideo := hashes[index]
	slog.Info("Finding neighbors for video", slog.Int("index", index), slog.String("hash", currentVideo.HashValue), slog.Float64("duration", float64(currentVideo.Duration)))

	for i, neighbor := range hashes {
		if index == i || currentVideo.ID == neighbor.ID {
			continue
		}

		durationDiff := math.Abs(float64(currentVideo.Duration - neighbor.Duration))
		slog.Debug("Duration difference", slog.Int("video1", index), slog.Int("video2", i), slog.Float64("difference", durationDiff))

		if int(durationDiff) > options.MaxDurationDiff {
			slog.Debug("Skipping video due to duration difference", slog.Int("video", i), slog.Float64("difference", durationDiff))
			continue
		}

		hashDistance, _ := calcHammingDistance(currentVideo.HashValue, neighbor.HashValue)
		slog.Debug("Hash distance", slog.Int("video1", index), slog.Int("video2", i), slog.Int("distance", hashDistance))

		if hashDistance <= options.MaxHashDistance {
			neighbors = append(neighbors, i)
			slog.Info("Neighbor found", slog.Int("video1", index), slog.Int("video2", i))
		} else {
			slog.Debug("Hash distance too high", slog.Int("video1", index), slog.Int("video2", i), slog.Int("threshold", options.MaxHashDistance))
		}
	}

	return neighbors
}

func propagateBucket(bucket int, neighbors []int, hashes []*models.Videohash) {
	for _, neighborIndex := range neighbors {
		neighbor := hashes[neighborIndex]
		if neighbor.Bucket == -1 {
			slog.Info("Propagating bucket", slog.Int("bucket", bucket), slog.Int("video", neighborIndex))
			neighbor.Bucket = bucket
			propagateBucket(bucket, neighbor.Neighbours, hashes)
		}
	}
}

func findNextBucketID(hashes []*models.Videohash) int {
	maxBucket := -1
	for _, video := range hashes {
		if video.Bucket > maxBucket {
			maxBucket = video.Bucket
		}
	}
	slog.Info("Next bucket ID", slog.Int("nextBucket", maxBucket+1))
	return maxBucket + 1
}

func calcHammingDistance(hashVal1 string, hashVal2 string) (int, error) {
	slog.Debug("Calculating Hamming distance", slog.String("hash1", hashVal1), slog.String("hash2", hashVal2))

	if len(hashVal1) != len(hashVal2) {
		return 0, fmt.Errorf("hash values must have the same length")
	}

	distance := 0
	for i := range len(hashVal1) {
		if hashVal1[i] != hashVal2[i] {
			distance++
		}
	}

	slog.Debug("Hamming distance calculated", slog.Int("distance", distance))
	return distance, nil
}
