package hash

type HashType interface {
	GenerateHashes(videoPaths string) (string, error)
}
