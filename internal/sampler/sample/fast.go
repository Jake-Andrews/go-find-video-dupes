package sample

type FastSampler struct {
	Frames      int
	SkipPercent float64
}

func (s *FastSampler) Sample(videoPath string) ([]byte, error) {
	return []byte{}, nil
}

func (s *FastSampler) Name() string {
	return "FastSampler"
}
