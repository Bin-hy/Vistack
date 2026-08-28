export interface CommentAuthor {
  id: string
  nickname: string
  avatar_url: string
}

export interface CommentAttachment {
  type: string
  url: string
}

export interface CommentItem {
  id: string
  video_id: string
  user_id: string
  root_id?: number | null
  parent_id?: number | null
  reply_to_id?: number | null
  content: string
  attachments: CommentAttachment[]
  status: string
  like_count: number
  reply_count: number
  created_at: string
  deleted: boolean
  author?: CommentAuthor | null
  reply_to_author?: CommentAuthor | null
  liked: boolean
}

export interface CommentListResponse {
  comments: CommentItem[]
  next_cursor: string
  total: string
}
