import { get, post } from '@/lib/http'

export interface InitUploadPayload {
	filename: string
	mime_type?: string
}

export interface InitUploadResponse {
	upload_id: string
	object_key: string
	bucket: string
}

export interface UploadPartResponse {
	etag: string
}

export interface CompletePartPayload {
	PartNumber: number
	ETag: string
}

export interface CompleteUploadResponse {
	msg: string
	video_id: string
}

export interface CreatorVideoItem {
	id: string
	title: string
	description?: string | null
	cover_url?: string | null
	created_at: string
	duration?: number
	status?: string
	visibility?: string
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

export async function uploadVideoPart(params: {
	upload_id: string
	object_key: string
	partNumber: number
	chunk: Blob
}): Promise<UploadPartResponse> {
	const formData = new FormData()
	formData.append('upload_id', params.upload_id)
	formData.append('object_key', params.object_key)
	formData.append('part_number', String(params.partNumber))
	formData.append('chunk', params.chunk)

	return post<UploadPartResponse>('/videos/upload/part', formData, {
		headers: {
			'Content-Type': 'multipart/form-data',
		},
	})
}

export async function completeVideoUpload(params: {
	upload_id: string
	object_key: string
	filename: string
	parts: CompletePartPayload[]
}): Promise<CompleteUploadResponse> {
	return post<CompleteUploadResponse>('/videos/upload/complete', {
		upload_id: params.upload_id,
		object_key: params.object_key,
		filename: params.filename,
		parts: params.parts,
	})
}

export async function getMyVideos(params: {
	page?: number
	page_size?: number
	keyword?: string
}): Promise<CreatorVideoListResponse> {
	return get<CreatorVideoListResponse>('/videos/plateform/list', params)
}
