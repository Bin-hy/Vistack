import { get } from '@/lib/http'

export interface VideoAuthor {
	id: string
	nickname: string
	avatar_url: string
}

export interface VideoItem {
	id: string
	title: string
	description?: string
	cover_url: string
	duration: number
	status: string
	visibility: string
	created_at: string
	updated_at: string
	user?: VideoAuthor
}

export interface RecommendResponse {
	videos: VideoItem[]
}

export function getVideoRecommend() {
	return get<RecommendResponse>('/videos/recommend')
}
