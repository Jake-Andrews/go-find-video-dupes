package hash

import (
	"log/slog"
	"os"
	"testing"

	"govdupes/internal/config"
	"govdupes/internal/filesystem"
	"govdupes/internal/videoprocessor"
	"govdupes/internal/videoprocessor/ffprobe"
)

func TestMain(m *testing.M) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	os.Exit(m.Run())
}

func TestSlowHash(t *testing.T) {
	wantHashValue := "94aad439d6a8d770b1cee936c49ba452aad5b54a36c5aa5293a4ce152bdcb74292ae5f210eb9e55486b954225db2ee558fb0764d9aa45966adb0570dbaa45b26a7b8560fd1a25d23b1ae5521ceb9457898a74a14e99e725daa95285bbe815d6aa8d7374815ac5a75b4caba654ab6c9329be48d536499e7148cf3a718638ee8538ef9824c718af55a87f8934f38a3564a82bf98433ee55269c0af9e402ff45965d9a2957a25f74924dda2977824b3cd24d9a6957825b6d825c0bfb8412ea5d26d87f8a34e7182b55a98e7a719628ca95799e6a459629ce95698f3a718628fa8578cf3a75c7182ac538dfab10e7d82e55087f8f0067d82a7d487faf0077c11aa9587f8f0876e11aa95c6bbe11f4e6083c6c6bce31c5aa493ccdab4d10658a5db9acbb4c5166895a6d9cbb4c5166895a6d9cbb4c1166895a6dbcbb4c5166895a6d9cbb2c5166895a6d9c9b2c5176895a6d9c9b2c5176895a6d9cbb2c5166895a6d9cdb2c4136c91a7d9c3acd1275825d2b787f893423c4a95fac1be97400f6a95dad79de2282b1fd490b1dce4fde4a1a007babaeeace5e1840490a28dd4876deea38d9492c1e9b2f2b6b9bba8e4e6d400f2e5c19c9d941823f7d8a4d62f5877a82981be976a2549d6aa83b4b64a1d46f69a87b8cb403c4be5ba8fb0d3613c43e1ba87b0ca713c4be4ba8fb086613f42e4bb87b895613e45e1ba87bc93613d52b1acc7b89a601f6aa4bac5beb0401f6b94dac5beb8401f6b94d8c5beb840176bb49987babc41176bb4919de2a7184d4fa0b698d3e3cc2a7148afdb90a0ebceb424f1"
	filePath := "../../sneed.webm"
	cfg := config.Config{SilentFFmpeg: false}
	vp := videoprocessor.NewFFmpegInstance(&cfg)

	fileInfo, _ := os.Lstat(filePath)
	slog.Info("fileInfo", "name", fileInfo.Name(), "size", fileInfo.Size())

	video := filesystem.CreateVideo(filePath, fileInfo, filesystem.FileIdentity{})
	_ = ffprobe.GetVideoInfo(&video)

	got, _, err := createSlowPhash(vp, &video)
	if err != nil {
		t.Fatalf("createSlowHash(%q) err = %q, want nil", filePath, err)
	}

	if got.HashValue != wantHashValue {
		t.Fatalf("createSlowHash got = %q, want = %q", got.HashValue, wantHashValue)
	}
}

