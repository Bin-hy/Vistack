package interaction

import (
	"context"
	"strconv"
)

// Hot 返回热门视频 ID 列表（按 sort 维度降序）。sort: "play" 或 "like"。
func (s *Service) Hot(ctx context.Context, sort string, limit int) ([]int64, error) {
	key := hotPlayKey
	if sort == "like" {
		key = hotLikeKey
	}
	if limit <= 0 {
		limit = s.opts.LeaderboardSize
	}

	vals, err := s.rdb.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(vals))
	for _, v := range vals {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
