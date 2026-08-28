package auth

import (
	"context"

	authpb "github.com/binhy/vistack/internal/auth/pb/auth/v1"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/model/entity/user"
)

// Service 实现 UserService gRPC：按 user_id 批量返回用户公开信息。
type Service struct {
	authpb.UnimplementedUserServiceServer
}

func NewService() *Service { return &Service{} }

func (s *Service) GetUserInfos(ctx context.Context, req *authpb.GetUserInfosRequest) (*authpb.GetUserInfosResponse, error) {
	ids := req.GetUserIds()
	resp := &authpb.GetUserInfosResponse{Users: make([]*authpb.UserInfo, 0, len(ids))}
	if len(ids) == 0 {
		return resp, nil
	}

	var users []user.User
	if err := core.DB.Preload("Role").Preload("Profile").Preload("Profile.Avatar").
		Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}

	for i := range users {
		resp.Users = append(resp.Users, toProtoUserInfo(&users[i]))
	}
	return resp, nil
}

func toProtoUserInfo(u *user.User) *authpb.UserInfo {
	info := &authpb.UserInfo{
		Id:       u.ID,
		Username: u.Username,
	}
	if u.Role.ID != 0 {
		info.Role = u.Role.Name
	}
	if u.Profile != nil {
		if u.Profile.Nickname != nil {
			info.Nickname = *u.Profile.Nickname
		}
		if u.Profile.Avatar != nil {
			f := u.Profile.Avatar
			if f.Bucket != "" && f.ObjectKey != "" {
				info.AvatarUrl = f.PublicURL(core.GetPublicBaseURL())
			}
		}
	}
	if info.Nickname == "" {
		info.Nickname = u.Username
	}
	return info
}
