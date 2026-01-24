import { del, get, post } from '@/lib/http'

export interface InitUploadPayload {
	filename: string
	file_hash: string
	mime_type?: string
}

export interface InitUploadResponse {
	upload_id: string
	object_key: string
	bucket: string
	uploaded: boolean
	video_id: string
}

export interface GetUploadPartUrlResponse {
	url: string
}

export interface MinioObjectPart {
	PartNumber: number
	ETag: string
	Size: number
	LastModified: string
}

export interface ListUploadedPartsResponse {
	parts: MinioObjectPart[]
}

export interface CompletePartPayload {
	PartNumber: number
	ETag: string
}

export interface CompleteUploadResponse {
	msg: string
	video_id: string
}

export const VideoStatus = {
	Uploaded: 'uploaded',
	Processing: 'processing',
	Published: 'published',
	Failed: 'failed',
	Deleted: 'deleted',
} as const
export type VideoStatus = typeof VideoStatus[keyof typeof VideoStatus]

export const VideoVisibility = {
	Public: 'public',
	Private: 'private',
	Unlisted: 'unlisted',
} as const
export type VideoVisibility = typeof VideoVisibility[keyof typeof VideoVisibility]

export interface CreatorVideoItem {
	id: string
	title: string
	description?: string | null
	cover_url?: string | null
	created_at: string
	duration?: number
	status?: VideoStatus
	visibility?: VideoVisibility
}

export interface CreatorVideoListResponse {
	list: CreatorVideoItem[]
	total: number
	page: number
	page_size: number
}

export async function initVideoUpload(payload: InitUploadPayload): Promise<InitUploadResponse> {
	return post<InitUploadResponse>('/videos/upload/init', payload)
}

export async function getUploadPartUrl(params: {
	upload_id: string
	object_key: string
	partNumber: number
}): Promise<GetUploadPartUrlResponse> {
	return get<GetUploadPartUrlResponse>('/videos/upload/sign', {
		upload_id: params.upload_id,
		object_key: params.object_key,
		partNumber: params.partNumber,
	})
}

export async function listUploadedParts(params: {
	upload_id: string
	object_key: string
}): Promise<ListUploadedPartsResponse> {
	return get<ListUploadedPartsResponse>('/videos/upload/parts', {
		upload_id: params.upload_id,
		object_key: params.object_key,
	})
}

export async function completeVideoUpload(params: {
	upload_id: string
	object_key: string
	filename: string
	file_hash: string
	parts: CompletePartPayload[]
}): Promise<CompleteUploadResponse> {
	return post<CompleteUploadResponse>('/videos/upload/complete', params)
}

export async function getMyVideos(params: {
	page?: number
	page_size?: number
	keyword?: string
}): Promise<CreatorVideoListResponse> {
	return get<CreatorVideoListResponse>('/videos/plateform/list', params)
}

export async function deleteVideo(videoId: string): Promise<void> {
	return del<void>(`/videos/${videoId}`)
}
