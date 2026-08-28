package v1

import (
	"context"
	"strconv"

	authpb "github.com/binhy/vistack/internal/auth/pb/auth/v1"
	"github.com/binhy/vistack/internal/authclient"
	"github.com/binhy/vistack/internal/core"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"go.uber.org/zap"
)

// userClient 由 role/api.go 启动时注入，供视频作者展示等场景批量查询用户信息。
var userClient *authclient.UserClient

// SetUserClient 注入 auth 用户查询客户端。
func SetUserClient(c *authclient.UserClient) {
	userClient = c
}

// resolveAuthor 查询单个用户的作者信息。
func resolveAuthor(ctx context.Context, userID int64) *VideoAuthorResponse {
	if userID == 0 || userClient == nil {
		return nil
	}
	infos, err := userClient.GetUserInfos(ctx, []int64{userID})
	if err != nil {
		core.Logger.Error("get user infos failed", zap.Error(err))
		return nil
	}
	if info, ok := infos[userID]; ok {
		return toAuthor(info)
	}
	return nil
}

// resolveAuthors 批量查询视频作者信息，返回 user_id -> 作者 映射（去重后一次 RPC）。
func resolveAuthors(ctx context.Context, videos []mVideo.Video) map[int64]*VideoAuthorResponse {
	result := make(map[int64]*VideoAuthorResponse)
	if userClient == nil {
		return result
	}
	ids := make([]int64, 0, len(videos))
	seen := make(map[int64]struct{}, len(videos))
	for _, v := range videos {
		if v.UserID == 0 {
			continue
		}
		if _, ok := seen[v.UserID]; !ok {
			seen[v.UserID] = struct{}{}
			ids = append(ids, v.UserID)
		}
	}
	if len(ids) == 0 {
		return result
	}
	infos, err := userClient.GetUserInfos(ctx, ids)
	if err != nil {
		core.Logger.Error("get user infos failed", zap.Error(err))
		return result
	}
	for id, info := range infos {
		result[id] = toAuthor(info)
	}
	return result
}

func toAuthor(info *authpb.UserInfo) *VideoAuthorResponse {
	return &VideoAuthorResponse{
		ID:        strconv.FormatInt(info.Id, 10),
		Nickname:  info.Nickname,
		AvatarURL: info.AvatarUrl,
	}
}
