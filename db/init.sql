-- ======================================
-- 用户系统 & 权限管理 RBAC & ABAC & ACL
-- ======================================

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) UNIQUE,
    password_hash TEXT NOT NULL,
    role_id BIGINT NOT NULL -- REFERENCES roles(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'banned')),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE user_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname VARCHAR(100),
    avatar_url TEXT,
);

CREATE TABLE roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT
);

-- 权限表 存储资源方法和资源uri
CREATE TABLE authorities (
    id BIGSERIAL PRIMARY KEY,
    resource_method VARCHAR(10) NOT NULL COMMENT '资源方法, 例如GET, POST, PATCH, PUT, DELETE',
    resource_uri VARCHAR(255) NOT NULL COMMENT '资源uri, 例如/videos, /users/{id}'
);

CREATE TABLE role_authority (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    authority_id BIGINT NOT NULL REFERENCES authority(id) ON DELETE CASCADE
);

-- 用户额外权限 + 用户提出权限
CREATE TABLE user_authority (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE COMMENT '关联的用户id',
    authority_id BIGINT NOT NULL REFERENCES authority(id) ON DELETE CASCADE COMMENT '关联的权限id',
    grand_status BOOLEAN DEFAULT FALSE COMMENT '是否授权',
    remark TEXT COMMENT '权限备注，例如为什么授权，为什么禁用',
)

--
-- ======================================
-- 文件管理
-- ======================================
CREATE TABLE files (
    id BIGSERIAL PRIMARY KEY,
    bucket VARCHAR(100) NOT NULL COMMENT 'MINIO 桶名称',
    object_key TEXT NOT NULL COMMENT '对象路径（相对 bucket 的 key)',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
      CHECK (status IN ('active','inactive', 'replaced', 'deleted')) COMMENT '文件状态',
    ref_type VARCHAR(50) COMMENT '引用类型，例如 avatar / video / video_segment / cover',
    ref_id BIGINT COMMENT '引用id, 例如user_id / video_id / transcode_id',
    mime_type VARCHAR(100) COMMENT '文件mime类型',
    size BIGINT COMMENT '文件大小, 字节',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(bucket, object_key)
)

-- ======================================
-- 视频管理
-- ======================================

CREATE TABLE videos (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(255) NOT NULL COMMENT '视频标题',
    description TEXT COMMENT '视频描述',
    cover_file_id BIGINT COMMENT '视频封面文件id', -- REFERENCES files(id)
    duration INT DEFAULT 0 COMMENT '视频时长, 秒',
    status VARCHAR(20) NOT NULL DEFAULT 'uploaded'
        CHECK (status IN ('uploaded', 'processing', 'published', 'blocked')),
    visibility VARCHAR(20) NOT NULL DEFAULT 'public'
        CHECK (visibility IN ('public', 'private', 'unlisted')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 原始视频上传记录
CREATE TABLE video_sources (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL, -- REFERENCES videos(id) ON DELETE CASCADE,
    file_id BIGINT NOT NULL, -- REFERENCES files(id) ON DELETE CASCADE,
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

-- 转码任务
CREATE TABLE video_transcodes (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL ,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    resolution VARCHAR(20) COMMENT '视频分辨率, 例如1080p, 720p',
    codec VARCHAR(50) COMMENT '视频编码格式, 例如H.264, H.265',
    manifest_file_id BIGINT COMMENT 'MPD 文件id', -- REFERENCES files(id)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- DASH | HLS 清单文件
CREATE TABLE video_manifest (
  id BIGSERIAL PRIMARY KEY,
  video_id BIGINT NOT NULL REFERENCES videos(id),
  protocol VARCHAR(20) NOT NULL CHECK (protocol IN ('dash','hls')),
  file_id BIGINT NOT NULL REFERENCES files(id), -- MPD | HLS 清单文件id
  profiles JSONB COMMENT '码率信息, 例如[{"resolution" : "360p", "bitrate" :800000}]',
  status VARCHAR(20) DEFAULT 'ready',
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ======================================
-- 标签系统
-- ======================================

CREATE TABLE tags (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE video_tags (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, tag_id)
);

-- ======================================
-- 社交互动
-- ======================================

-- 视频评论
CREATE TABLE video_comments (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id BIGINT REFERENCES video_comments(id) ON DELETE CASCADE COMMENT '父评论id',
    content TEXT NOT NULL COMMENT '评论内容',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 视频点赞
CREATE TABLE video_likes (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (video_id, user_id) COMMENT '视频点赞主键, 视频id + 用户id'
);

-- 视频收藏
CREATE TABLE video_favorites (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (video_id, user_id) COMMENT '视频收藏主键, 视频id + 用户id'
);

-- ======================================
-- 播放日志
-- ======================================

CREATE TABLE video_play_logs (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL COMMENT '用户id, 可能为空',
    played_at TIMESTAMPTZ DEFAULT NOW() COMMENT '播放时间',
    ip_address INET COMMENT '用户ip地址',
    user_agent TEXT COMMENT '用户代理信息'
);

-- ======================================
-- 审计日志
-- ======================================

-- CREATE TABLE audit_logs (
--     id BIGSERIAL PRIMARY KEY,
--     user_id BIGINT REFERENCES users(id) ON DELETE SET NULL ,
--     action VARCHAR(255) NOT NULL COMMENT '操作类型',
--     target_type VARCHAR(100) COMMENT '目标资源类型',
--     target_uuid UUID COMMENT '目标资源id',
--     created_at TIMESTAMPTZ DEFAULT NOW()
-- );
