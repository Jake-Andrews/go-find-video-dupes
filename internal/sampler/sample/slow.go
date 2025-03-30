package sample

type SlowSampler struct {
	SkipPercent float64
	FPS         float64
}

func (s *SlowSampler) Sample(videoPath string) ([]byte, error) {
	return []byte{}, nil
}

func (s *SlowSampler) Name() string {
	return "SlowSampler"
}
