package sampler

type Sampler interface {
	Sample(videoPath string) ([]byte, error)
	Name() string
}
