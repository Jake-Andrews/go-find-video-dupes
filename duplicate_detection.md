https://github.com/facebook/ThreatExchange/
https://www.hackerfactor.com/blog/index.php?/archives/971-FB-TMK-PDQ-WTF.html
> Before using `ffmpeg-go`, FFmpeg must be installed and accessible via your
> path

Set Cpu limit/request For FFmpeg-go

e := ComplexFilterExample("./sample_data/in1.mp4", "./sample_data/overlay.png", "./sample_data/out2.mp4")
err := e.RunWithResource(0.1, 0.5)
if err != nil {
    assert.Nil(t, err)
}
