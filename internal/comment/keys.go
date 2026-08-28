package comment

import "fmt"

func likeKey(commentID int64) string       { return fmt.Sprintf("vistack:comment:like:%d", commentID) }
func commentCountKey(videoID int64) string { return fmt.Sprintf("vistack:comment:count:%d", videoID) }
func listCacheKey(videoID int64) string    { return fmt.Sprintf("vistack:comment:list:%d", videoID) }
