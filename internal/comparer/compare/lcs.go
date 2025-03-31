package compare

import "govdupes/internal/sampler"

type LCSComparer struct {
	HammingDistance int
	SlidingWindow   int
}

func (pc *LCSComparer) Compare(hashes1, hashes2 []string) bool {
	return true
}

func (pc *LCSComparer) Name() string {
	return ""
}

func (pc *LCSComparer) ValidateSampler(sampler sampler.Sampler) error {
	return nil
}
