import { sha256 } from 'js-sha256'

export interface AwsCredentials {
	accessKey: string
	secretKey: string
	sessionToken?: string
}

export interface SigV4SignedHeaders {
	[key: string]: string
}

function toAmzDate(date: Date): string {
	const y = date.getUTCFullYear().toString().padStart(4, '0')
	const m = (date.getUTCMonth() + 1).toString().padStart(2, '0')
	const d = date.getUTCDate().toString().padStart(2, '0')
	const hh = date.getUTCHours().toString().padStart(2, '0')
	const mm = date.getUTCMinutes().toString().padStart(2, '0')
	const ss = date.getUTCSeconds().toString().padStart(2, '0')
	return `${y}${m}${d}T${hh}${mm}${ss}Z`
}

function toDateStamp(date: Date): string {
	const y = date.getUTCFullYear().toString().padStart(4, '0')
	const m = (date.getUTCMonth() + 1).toString().padStart(2, '0')
	const d = date.getUTCDate().toString().padStart(2, '0')
	return `${y}${m}${d}`
}

function hmacArray(key: string | Uint8Array, data: string): Uint8Array {
	const bytes = sha256.hmac.array(key as any, data)
	return new Uint8Array(bytes)
}

function hmacHex(key: Uint8Array, data: string): string {
	return sha256.hmac(key as any, data)
}

function getSignatureKey(secretKey: string, dateStamp: string, region: string, service: string): Uint8Array {
	const kDate = hmacArray('AWS4' + secretKey, dateStamp)
	const kRegion = hmacArray(kDate, region)
	const kService = hmacArray(kRegion, service)
	const kSigning = hmacArray(kService, 'aws4_request')
	return kSigning
}

function encodeRfc3986(str: string): string {
	return encodeURIComponent(str).replace(/[!'()*]/g, c => '%' + c.charCodeAt(0).toString(16).toUpperCase())
}

function buildCanonicalQuery(url: URL): string {
	const entries: [string, string][] = []
	url.searchParams.forEach((value, key) => {
		entries.push([key, value])
	})
	entries.sort((a, b) => (a[0] === b[0] ? a[1].localeCompare(b[1]) : a[0].localeCompare(b[0])))
	return entries
		.map(([k, v]) => `${encodeRfc3986(k)}=${encodeRfc3986(v)}`)
		.join('&')
}

function buildCanonicalPath(pathname: string): string {
	if (!pathname) return '/'
	const parts = pathname.split('/').filter(Boolean)
	return '/' + parts.map(p => encodeRfc3986(p)).join('/')
}

export function signS3Request(urlStr: string, method: string, region: string, credentials: AwsCredentials, now: Date): SigV4SignedHeaders {
	const url = new URL(urlStr)
	const service = 's3'
	const amzDate = toAmzDate(now)
	const dateStamp = toDateStamp(now)
	const host = url.host
	const canonicalUri = buildCanonicalPath(url.pathname)
	const canonicalQuery = buildCanonicalQuery(url)
	const payloadHash = 'UNSIGNED-PAYLOAD'
	let canonicalHeaders = `host:${host}\n` + `x-amz-content-sha256:${payloadHash}\n` + `x-amz-date:${amzDate}\n`
	let signedHeaders = 'host;x-amz-content-sha256;x-amz-date'
	if (credentials.sessionToken) {
		canonicalHeaders += `x-amz-security-token:${credentials.sessionToken}\n`
		signedHeaders += ';x-amz-security-token'
	}
	const canonicalRequest = [
		method.toUpperCase(),
		canonicalUri,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	].join('\n')
	const algorithm = 'AWS4-HMAC-SHA256'
	const credentialScope = `${dateStamp}/${region}/${service}/aws4_request`
	const stringToSign = [
		algorithm,
		amzDate,
		credentialScope,
		sha256(canonicalRequest),
	].join('\n')
	const signingKey = getSignatureKey(credentials.secretKey, dateStamp, region, service)
	const signature = hmacHex(signingKey, stringToSign)
	const authorization = `${algorithm} Credential=${credentials.accessKey}/${credentialScope}, SignedHeaders=${signedHeaders}, Signature=${signature}`
	const headers: SigV4SignedHeaders = {
		Authorization: authorization,
		'x-amz-date': amzDate,
		'x-amz-content-sha256': payloadHash,
	}
	if (credentials.sessionToken) {
		headers['x-amz-security-token'] = credentials.sessionToken
	}
	return headers
}

