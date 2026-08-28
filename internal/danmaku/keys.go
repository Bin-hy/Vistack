package danmaku

import "fmt"

func danmakuKey(videoID int64) string { return fmt.Sprintf("vistack:danmaku:%d", videoID) }
