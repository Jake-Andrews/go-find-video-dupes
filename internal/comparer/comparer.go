package comparer

import "govdupes/internal/sampler"

type Comparer interface {
	Compare(hashes1, hashes2 []string) bool
	Name() string
	ValidateSampler(sampler sampler.Sampler) error
}
