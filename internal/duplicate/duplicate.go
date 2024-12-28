package duplicate

import (
	"fmt"
	"log"
	"math"

	"govdupes/internal/models"
)

type DuplicateOptions struct {
	MaxDurationDiff int
	MaxHashDistance int
}

func FindVideoDuplicates(hashes []*models.Videohash) ([][]int, []*models.Videohash, error) {
	options := DuplicateOptions{MaxDurationDiff: 5, MaxHashDistance: 4}
	initializeBuckets(hashes)

	for i, video := range hashes {
		if video.Bucket == -1 {
			log.Printf("Video %d has no bucket, assigning to a bucket...\n", i)
			assignToExistingOrNewBucket(video, i, hashes, options)
			log.Println(video)
		} else {
			log.Printf("Video: %d has no bucket, video: %v", i, video)
		}
	}

	buckets := collectBuckets(hashes)
	duplicateHashes := collectDuplicateHashes(hashes)

	return buckets, duplicateHashes, nil
}

func initializeBuckets(hashes []*models.Videohash) {
	log.Println("Initializing buckets...")
	for _, video := range hashes {
		video.Bucket = -1
		video.Neighbours = []int{}
	}
}

func assignToExistingOrNewBucket(video *models.Videohash, index int, hashes []*models.Videohash, options DuplicateOptions) {
	log.Printf("Assigning video %d to a bucket...\n", index)
	neighbors := findNeighbors(index, hashes, options)
	video.Neighbours = neighbors

	// Try to assign to an existing bucket from its neighbors
	for _, neighborIndex := range neighbors {
		neighbor := hashes[neighborIndex]
		if neighbor.Bucket != -1 {
			log.Printf("Found existing bucket %d for video %d\n", neighbor.Bucket, index)
			video.Bucket = neighbor.Bucket
			break
		}
	}

	// If no existing bucket was found, create a new one
	if video.Bucket == -1 {
		video.Bucket = findNextBucketID(hashes)
		log.Printf("No existing bucket found for video %d. Assigning to new bucket %d\n", index, video.Bucket)
	}

	// Propagate the bucket assignment to all neighbors
	propagateBucket(video.Bucket, neighbors, hashes)
}

func findNeighbors(index int, hashes []*models.Videohash, options DuplicateOptions) []int {
	var neighbors []int
	currentVideo := hashes[index]
	log.Printf("Finding neighbors for video %d (hash: %s, duration: %.2f)...\n", index, currentVideo.HashValue, currentVideo.Duration)

	// Directly compare the hash string
	for i, neighbor := range hashes {
		if index == i || currentVideo.ID == neighbor.ID {
			continue // Skip self and identical videos
		}

		durationDiff := math.Abs(float64(currentVideo.Duration - neighbor.Duration))
		log.Printf("Duration diff between video %d and video %d: %f seconds\n", index, i, durationDiff)

		if int(durationDiff) > options.MaxDurationDiff {
			log.Printf("Skipping video %d (duration diff: %f) - exceeds max duration diff\n", i, durationDiff)
			continue
		}

		// Directly calculate the hash distance on the strings
		hashDistance, _ := calcHammingDistance(currentVideo.HashValue, neighbor.HashValue)
		log.Printf("Hash distance between video %d and video %d: %d\n", index, i, hashDistance)

		if hashDistance <= options.MaxHashDistance {
			neighbors = append(neighbors, i)
			log.Printf("Video %d is a neighbor to video %d\n", i, index)
		} else {
			log.Printf("Hash distance too high for video %d and video %d (threshold: %d)\n", index, i, options.MaxHashDistance)
		}
	}

	return neighbors
}

func propagateBucket(bucket int, neighbors []int, hashes []*models.Videohash) {
	for _, neighborIndex := range neighbors {
		neighbor := hashes[neighborIndex]
		if neighbor.Bucket == -1 {
			log.Printf("Propagating bucket %d to video %d\n", bucket, neighborIndex)
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
	log.Printf("Next bucket ID: %d\n", maxBucket+1)
	return maxBucket + 1
}

func collectBuckets(hashes []*models.Videohash) [][]int {
	bucketMap := make(map[int][]int)
	for _, video := range hashes {
		if video.Bucket != -1 {
			bucketMap[video.Bucket] = append(bucketMap[video.Bucket], int(video.ID))
		}
	}

	var buckets [][]int
	for _, ids := range bucketMap {
		if len(ids) > 1 { // Only include groups with more than one video
			buckets = append(buckets, ids)
		}
	}

	log.Printf("Buckets collected: %v\n", buckets)
	return buckets
}

func IsSimilarTo(hash1 *models.Videohash, hash2 *models.Videohash, options DuplicateOptions) (bool, error) {
	log.Printf("Comparing video %d with video %d for similarity...\n", hash1.ID, hash2.ID)

	if math.Abs(float64(hash1.Duration)-float64(hash2.Duration)) > float64(options.MaxDurationDiff) {
		log.Printf("Videos have different durations (diff: %f), not similar.\n", math.Abs(float64(hash1.Duration)-float64(hash2.Duration)))
		return false, nil
	}

	distance, err := calcHammingDistance(hash1.HashValue, hash2.HashValue)
	if err != nil {
		return false, err
	}
	log.Printf("Hash distance between video %d and video %d: %d\n", hash1.ID, hash2.ID, distance)
	if distance > options.MaxHashDistance {
		log.Printf("Hash distance exceeds max limit (%d > %d), not similar.\n", distance, options.MaxHashDistance)
		return false, nil
	}

	return true, nil
}

func calcHammingDistance(hashVal1 string, hashVal2 string) (int, error) {
	log.Printf("Calculating Hamming distance between hashes: %s and %s\n", hashVal1, hashVal2)

	// Assuming hashVal1 and hashVal2 are hexadecimal strings or binary strings
	if len(hashVal1) != len(hashVal2) {
		return 0, fmt.Errorf("hash values must have the same length")
	}

	// Calculate Hamming distance on string comparison
	distance := 0
	for i := 0; i < len(hashVal1); i++ {
		if hashVal1[i] != hashVal2[i] {
			distance++
		}
	}

	log.Printf("Hamming distance between hashes: %d\n", distance)
	return distance, nil
}

func collectDuplicateHashes(hashes []*models.Videohash) []*models.Videohash {
	bucketMap := make(map[int][]*models.Videohash)

	// Group videohashes by their bucket
	for _, video := range hashes {
		if video.Bucket != -1 {
			bucketMap[video.Bucket] = append(bucketMap[video.Bucket], video)
		}
	}

	var duplicates []*models.Videohash
	for _, group := range bucketMap {
		if len(group) > 1 { // Only include groups with more than one video
			duplicates = append(duplicates, group...)
		}
	}

	log.Printf("Collected duplicate videohashes: %v\n", duplicates)
	return duplicates
}
