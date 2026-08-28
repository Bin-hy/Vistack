import { del, get, post } from '@/lib/http'
import type { CommentItem, CommentListResponse } from './types'

export interface CreateCommentPayload {
  content: string
  parent_id?: number | null
  reply_to_id?: number | null
  attachments?: { type: string; file_id: number }[]
}

export function listComments(videoId: string, cursor = 0, limit = 20) {
  return get<CommentListResponse>(`/videos/${videoId}/comments`, { cursor, limit })
}

export function listReplies(commentId: string, cursor = 0, limit = 20) {
  return get<{ comments: CommentItem[]; next_cursor: string }>(`/comments/${commentId}/replies`, {
    cursor,
    limit,
  })
}

export function commentCount(videoId: string) {
  return get<{ total: string }>(`/videos/${videoId}/comments/count`)
}

export function createComment(videoId: string, payload: CreateCommentPayload) {
  return post<{ id: string; status: string }>(`/videos/${videoId}/comments`, payload)
}

export function toggleCommentLike(commentId: string) {
  return post<{ liked: boolean; like_count: number }>(`/comments/${commentId}/like`)
}

export function deleteComment(commentId: string) {
  return del<{ deleted: boolean }>(`/comments/${commentId}`)
}

export function uploadCommentImage(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  return post<{ file_id: string; url: string }>('/file/comment', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
