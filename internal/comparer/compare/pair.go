package compare

import "govdupes/internal/sampler"

type PairComparer struct{}

func (pc *PairComparer) Compare(hashes1, hashes2 []string) bool {
	return true
}

func (pc *PairComparer) Name() string {
	return ""
}

func (pc *PairComparer) ValidateSampler(sampler sampler.Sampler) error {
	return nil
}
