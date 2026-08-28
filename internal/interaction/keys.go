package interaction

import "fmt"

const (
	hotPlayKey = "vistack:hot:play"
	hotLikeKey = "vistack:hot:like"
	pendingKey = "vistack:interaction:pending"
)

func likeKey(videoID int64) string { return fmt.Sprintf("vistack:like:%d", videoID) }
func favKey(videoID int64) string  { return fmt.Sprintf("vistack:fav:%d", videoID) }
func playKey(videoID int64) string { return fmt.Sprintf("vistack:play:%d", videoID) }
