package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"

	"github.com/binhy/vistack/internal/core"
)

type VideoInfo struct {
	Streams []struct {
		Width              int    `json:"width"`
		Height             int    `json:"height"`
		CodedWidth         int    `json:"coded_width"`
		CodedHeight        int    `json:"coded_height"`
		DisplayAspectRatio string `json:"display_aspect_ratio"`
		SampleAspectRatio  string `json:"sample_aspect_ratio"`
	} `json:"streams"`
}

func TestFfmpeg(t *testing.T) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,coded_width,coded_height,display_aspect_ratio,sample_aspect_ratio",
		"-of", "json",
		"D:\\视频\\35358771221-1-192.mp4", // 替换为您的视频路径
	)

	output, err := cmd.Output()
	if err != nil {
		panic(err)
	}

	var info VideoInfo
	json.Unmarshal(output, &info)

	if len(info.Streams) > 0 {
		s := info.Streams[0]
		fmt.Printf("存储分辨率: %dx%d\n", s.Width, s.Height)
		fmt.Printf("编码分辨率: %dx%d\n", s.CodedWidth, s.CodedHeight)
		fmt.Printf("显示宽高比: %s\n", s.DisplayAspectRatio)
		fmt.Printf("采样宽高比: %s\n", s.SampleAspectRatio)

		// 如果 SAR 不是 1:1，计算实际显示分辨率
		if s.SampleAspectRatio != "1:1" && s.SampleAspectRatio != "" {
			fmt.Printf("\n⚠️  警告: SAR 不是 1:1，这可能导致分辨率混淆\n")
		}
	}
}

func TestTranscode(t *testing.T) {
	qualities, err := core.TranscodeToDASH(
		"D:\\视频\\35358771221-1-192.mp4",
		"tmp/35358771221-1-193/",
	)
	if err != nil {
		t.Fatal(err)
	}

	// 验证档位数量
	if len(qualities) != 3 {
		t.Errorf("Expected 3 qualities, got %d", len(qualities))
	}
}
