import { get } from '@/lib/http'

export interface VideoSegmentsSignatureCredentials {
	accessKey: string
	secretKey: string
	sessionToken: string
	expiration: string
}

export interface VideoSegmentsSignatureResponse {
	base_url: string
	credentials: VideoSegmentsSignatureCredentials
}

export function getVideoSegmentsSignature(videoId: string) {
	return get<VideoSegmentsSignatureResponse>(`/videos/${videoId}/segments/signature`)
}
