package duplicate

type SearchMethod interface {
	CompareHashes(hashes1, hashes2 []string) (bool, error)
}
