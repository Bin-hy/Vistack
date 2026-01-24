<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiButton, UiCard, UiInput } from '@/components/ui'
import { put, post as httpPost } from '@/lib/http'
import CreatorVideoCard from '@/components/creator/VideoCard.vue'
import { useUserStore } from '@/stores/user'
import SparkMD5 from 'spark-md5'
import {
	initVideoUpload,
	getUploadPartUrl,
	listUploadedParts,
	completeVideoUpload,
	type CompletePartPayload,
	getMyVideos,
	type CreatorVideoItem,
	deleteVideo,
} from './api/api'

const userStore = useUserStore()

const activeTab = ref<'upload' | 'videos'>('videos')

const title = ref('')
const description = ref('')
const file = ref<File | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const progress = ref(0)
const statusText = ref('')
const uploadedVideoId = ref<string | null>(null)
const errorMessage = ref<string | null>(null)
const fileHash = ref<string>('')

const chunkSize = 8 * 1024 * 1024

const canUpload = computed(() => !!file.value && !uploading.value)

const selectedFileName = computed(() => {
	if (!file.value) return '未选择文件'
	return file.value.name
})

const selectedFileSize = computed(() => {
	if (!file.value) return ''
	return formatSize(file.value.size)
})

function onFileChange(e: Event) {
	const target = e.target as HTMLInputElement
	const files = target.files
	if (files && files.length > 0) {
		file.value = files[0] as File
		fileHash.value = '' // reset hash
	} else {
		file.value = null
		fileHash.value = ''
	}
}

function triggerSelectFile() {
	if (fileInputRef.value) {
		fileInputRef.value.click()
	}
}

function formatSize(size: number): string {
	if (size >= 1024 * 1024 * 1024) {
		return (size / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
	}
	if (size >= 1024 * 1024) {
		return (size / (1024 * 1024)).toFixed(1) + ' MB'
	}
	if (size >= 1024) {
		return (size / 1024).toFixed(1) + ' KB'
	}
	return size + ' B'
}

function calculateHash(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const blobSlice = File.prototype.slice
		const chunkSize = 2 * 1024 * 1024
		const chunks = Math.ceil(file.size / chunkSize)
		const spark = new SparkMD5.ArrayBuffer()
		const fileReader = new FileReader()
		let currentChunk = 0

		fileReader.onload = function (e) {
			spark.append(e.target?.result as ArrayBuffer)
			currentChunk++

			if (currentChunk < chunks) {
				// Yield to UI thread
				setTimeout(loadNext, 0)
			} else {
				resolve(spark.end())
			}
			// Update progress for hashing (optional, mapped to 0-10%)
			progress.value = Math.floor((currentChunk / chunks) * 10)
			statusText.value = `正在计算文件指纹 (${Math.floor((currentChunk / chunks) * 100)}%)`
		}

		fileReader.onerror = function () {
			reject('Hash calculation failed')
		}

		function loadNext() {
			const start = currentChunk * chunkSize
			const end = start + chunkSize >= file.size ? file.size : start + chunkSize
			fileReader.readAsArrayBuffer(blobSlice.call(file, start, end))
		}

		loadNext()
	})
}

async function startUpload() {
	if (!userStore.isLoggedIn) {
		alert('请先登录')
		return
	}
	if (!file.value) {
		alert('请先选择视频文件')
		return
	}
	uploading.value = true
	errorMessage.value = null
	uploadedVideoId.value = null
	progress.value = 0
	statusText.value = '正在初始化上传'

	try {
		const currentFile = file.value

		// 1. Calculate Hash
		if (!fileHash.value) {
			fileHash.value = await calculateHash(currentFile)
		}

		// 2. Init Upload
		statusText.value = '正在初始化...'
		const initResp = await initVideoUpload({
			filename: currentFile.name,
			mime_type: currentFile.type,
			file_hash: fileHash.value,
		})

		if (initResp.uploaded) {
			// Instant upload success
			progress.value = 100
			statusText.value = '极速秒传成功！正在转码...'
			uploadedVideoId.value = initResp.video_id
			uploading.value = false
			return
		}

		// 3. Resume Check
		statusText.value = '正在检查断点...'
		const uploadedPartsResp = await listUploadedParts({
			upload_id: initResp.upload_id,
			object_key: initResp.object_key,
		})

		// Map uploaded parts: PartNumber -> ETag
		const uploadedMap = new Map<number, string>()
		const parts = uploadedPartsResp.parts || []
		parts.forEach(p => {
			uploadedMap.set(p.PartNumber, p.ETag)
		})

		const totalParts = Math.ceil(currentFile.size / chunkSize)
		const partsToUpload: number[] = []

		// Identify missing parts
		for (let i = 1; i <= totalParts; i++) {
			if (!uploadedMap.has(i)) {
				partsToUpload.push(i)
			}
		}

		// 4. Concurrent Upload
		const completedParts: CompletePartPayload[] = []
		// Fill already uploaded parts
		uploadedMap.forEach((etag, partNum) => {
			completedParts.push({ PartNumber: partNum, ETag: etag })
		})

		const concurrency = 6 // Increased concurrency for faster upload
		let completedCount = uploadedMap.size
		const totalCount = totalParts

		// Worker function
		const uploadWorker = async () => {
			while (partsToUpload.length > 0) {
				const partNumber = partsToUpload.shift()
				if (!partNumber) break

				const start = (partNumber - 1) * chunkSize
				const end = Math.min(start + chunkSize, currentFile.size)
				const chunk = currentFile.slice(start, end)

				try {
					// Get Presigned URL
					const signResp = await getUploadPartUrl({
						upload_id: initResp.upload_id,
						object_key: initResp.object_key,
						partNumber,
					})

					// Direct PUT to MinIO
					const uploadResp = await fetch(signResp.url, {
						method: 'PUT',
						body: chunk,
					})

					if (!uploadResp.ok) {
						throw new Error(`Upload part ${partNumber} failed: ${uploadResp.statusText}`)
					}

					// Get ETag from header
					const etag = uploadResp.headers.get('ETag') || ''
					if (!etag) {
						throw new Error(`No ETag for part ${partNumber}`)
					}

					completedParts.push({ PartNumber: partNumber, ETag: etag })
					completedCount++

					// Update progress (10% - 95%)
					const uploadProgress = Math.floor((completedCount / totalCount) * 85) + 10
					progress.value = uploadProgress
					statusText.value = `正在上传分片 ${completedCount}/${totalCount}`
				} catch (err) {
					console.error(err)
					// Put back to queue for retry (simple retry logic)
					// In production, should have max retry count
					partsToUpload.push(partNumber)
					// Wait a bit before retry
					await new Promise(r => setTimeout(r, 2000))
				}
			}
		}

		// Start workers
		const workers = Array(concurrency).fill(null).map(() => uploadWorker())
		await Promise.all(workers)

		// 5. Complete Upload
		statusText.value = '正在合并分片...'
		progress.value = 98

		// Sort parts by PartNumber
		completedParts.sort((a, b) => a.PartNumber - b.PartNumber)

		const completeResp = await completeVideoUpload({
			upload_id: initResp.upload_id,
			object_key: initResp.object_key,
			filename: title.value || currentFile.name,
			file_hash: fileHash.value,
			parts: completedParts,
		})

		uploadedVideoId.value = completeResp.video_id
		progress.value = 100
		statusText.value = '上传完成，正在转码'

	} catch (e: any) {
		const message = e?.message ?? '上传失败，请稍后重试'
		errorMessage.value = message
		statusText.value = '上传失败'
		console.error(e)
	} finally {
		uploading.value = false
	}
}

const videos = ref<CreatorVideoItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const listLoading = ref(false)
const listError = ref('')
const keyword = ref('')
const editingVideo = ref<CreatorVideoItem | null>(null)
const editDialogOpen = ref(false)
const editTitle = ref('')
const editDescription = ref('')
const editVisibility = ref<'public' | 'private'>('public')
const editCoverUrl = ref('')
const editCoverFileId = ref<number | null>(null)
const editSaving = ref(false)
const editError = ref('')
const coverInputRef = ref<HTMLInputElement | null>(null)
const cropDialogOpen = ref(false)
const rawCoverFile = ref<File | null>(null)
const cropPreviewUrl = ref('')
const cropScale = ref(1)
const cropOffsetX = ref(0)
const cropOffsetY = ref(0)
const cropUploading = ref(false)
const cropContainerRef = ref<HTMLElement | null>(null)
const isDragging = ref(false)
const pendingCoverBlob = ref<Blob | null>(null)
const pendingCoverName = ref('cover.jpg')
let dragStartX = 0
let dragStartY = 0
let dragStartOffsetX = 0
let dragStartOffsetY = 0
let cropContainerWidth = 0
let cropContainerHeight = 0

async function loadVideos() {
	if (!userStore.isLoggedIn) return
	listLoading.value = true
	listError.value = ''
	try {
		const resp = await getMyVideos({
			page: page.value,
			page_size: pageSize.value,
			keyword: keyword.value.trim() || undefined,
		})
		videos.value = resp.list || []
		total.value = resp.total || 0
		page.value = resp.page || 1
		pageSize.value = resp.page_size || pageSize.value
	} catch (e: any) {
		listError.value = e?.message ?? '加载视频列表失败'
	} finally {
		listLoading.value = false
	}
}

function goPrevPage() {
	if (page.value <= 1) return
	page.value -= 1
	loadVideos()
}

function goNextPage() {
	if (page.value * pageSize.value >= total.value) return
	page.value += 1
	loadVideos()
}

function onSearch() {
	page.value = 1
	loadVideos()
}

function openEditDialog(video: CreatorVideoItem) {
	editingVideo.value = video
	editTitle.value = video.title || ''
	editDescription.value = video.description || ''
	editVisibility.value = (video.visibility as 'public' | 'private') || 'public'
	editCoverUrl.value = video.cover_url || ''
	editCoverFileId.value = null
	editError.value = ''
	editDialogOpen.value = true
}

async function handleDeleteVideo(video: CreatorVideoItem) {
	if (!confirm(`确定要删除视频“${video.title}”吗？此操作不可恢复。`)) {
		return
	}
	try {
		await deleteVideo(video.id)
		alert('删除成功')
		editDialogOpen.value = false
		loadVideos()
	} catch (e: any) {
		alert(e?.message ?? '删除失败')
	}
}

function triggerSelectCover() {
	if (coverInputRef.value) coverInputRef.value.click()
}

async function onCoverFileChange(e: Event) {
	const target = e.target as HTMLInputElement
	const files = target.files
	if (!files || files.length === 0) return
	const file = files[0]
	if (!file) return
	rawCoverFile.value = file
	pendingCoverBlob.value = null
	editError.value = ''
	const reader = new FileReader()
	reader.onload = () => {
		cropPreviewUrl.value = typeof reader.result === 'string' ? reader.result : ''
		cropScale.value = 1
		cropOffsetX.value = 0
		cropOffsetY.value = 0
		cropDialogOpen.value = true
	}
	reader.readAsDataURL(file)
}

function onCropMouseDown(event: MouseEvent) {
	if (!cropContainerRef.value || !cropPreviewUrl.value) return
	isDragging.value = true
	const rect = cropContainerRef.value.getBoundingClientRect()
	cropContainerWidth = rect.width
	cropContainerHeight = rect.height
	dragStartX = event.clientX
	dragStartY = event.clientY
	dragStartOffsetX = cropOffsetX.value
	dragStartOffsetY = cropOffsetY.value
}

function onCropMouseMove(event: MouseEvent) {
	if (!isDragging.value || !cropContainerWidth || !cropContainerHeight) return
	const dx = event.clientX - dragStartX
	const dy = event.clientY - dragStartY
	const nx = dx / cropContainerWidth * 2
	const ny = dy / cropContainerHeight * 2
	let nextX = dragStartOffsetX + nx
	let nextY = dragStartOffsetY + ny
	if (nextX > 1) nextX = 1
	if (nextX < -1) nextX = -1
	if (nextY > 1) nextY = 1
	if (nextY < -1) nextY = -1
	cropOffsetX.value = nextX
	cropOffsetY.value = nextY
}

function onCropMouseUp() {
	isDragging.value = false
}

function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image()
		img.onload = () => resolve(img)
		img.onerror = () => reject(new Error('load image failed'))
		img.src = src
	})
}

async function confirmCrop() {
	if (!cropPreviewUrl.value) return
	cropUploading.value = true
	try {
		const img = await loadImage(cropPreviewUrl.value)
		const naturalW = img.naturalWidth
		const naturalH = img.naturalHeight
		if (!naturalW || !naturalH) throw new Error('invalid image')
		const aspect = 16 / 9
		const scale = cropScale.value <= 1 ? 1 : cropScale.value
		let cropW = naturalW / scale
		let cropH = cropW / aspect
		if (cropH > naturalH/scale) {
			cropH = naturalH / scale
			cropW = cropH * aspect
		}
		const maxOffsetX = (naturalW - cropW) / 2
		const maxOffsetY = (naturalH - cropH) / 2
		let centerX = naturalW / 2 + cropOffsetX.value * maxOffsetX
		let centerY = naturalH / 2 + cropOffsetY.value * maxOffsetY
		const halfW = cropW / 2
		const halfH = cropH / 2
		if (centerX < halfW) centerX = halfW
		if (centerX > naturalW - halfW) centerX = naturalW - halfW
		if (centerY < halfH) centerY = halfH
		if (centerY > naturalH - halfH) centerY = naturalH - halfH
		const sx = centerX - halfW
		const sy = centerY - halfH
		const canvas = document.createElement('canvas')
		canvas.width = 1280
		canvas.height = 720
		const ctx = canvas.getContext('2d')
		if (!ctx) throw new Error('no canvas context')
		ctx.drawImage(img, sx, sy, cropW, cropH, 0, 0, canvas.width, canvas.height)
		const blob: Blob = await new Promise((resolve, reject) => {
			canvas.toBlob(b => {
				if (b) resolve(b)
				else reject(new Error('toBlob failed'))
			}, 'image/jpeg', 0.9)
		})
		pendingCoverBlob.value = blob
		const originalFile = rawCoverFile.value
		pendingCoverName.value = originalFile?.name || 'cover.jpg'
		editCoverUrl.value = canvas.toDataURL('image/jpeg', 0.9)
		editError.value = ''
		cropDialogOpen.value = false
	} catch (e: any) {
		editError.value = e?.message ?? '封面处理失败'
	} finally {
		cropUploading.value = false
	}
}

async function saveEdit() {
	if (!editingVideo.value) return
	editSaving.value = true
	editError.value = ''
	try {
		let coverFileId: string | null = null
		if (pendingCoverBlob.value) {
			const formData = new FormData()
			formData.append('file', pendingCoverBlob.value, pendingCoverName.value)
			const resp = await httpPost<{ file_id: string; url: string }>('/file/cover', formData, {
				headers: { 'Content-Type': 'multipart/form-data' },
			})
			coverFileId = resp.file_id
			editCoverUrl.value = resp.url
		}
		const payload: any = {
			title: editTitle.value,
			description: editDescription.value,
			visibility: editVisibility.value,
		}
		if (coverFileId) {
			payload.cover_file_id = coverFileId
		}
		await put(`/videos/${editingVideo.value.id}`, payload)
		editDialogOpen.value = false
		pendingCoverBlob.value = null
		pendingCoverName.value = 'cover.jpg'
		await loadVideos()
	} catch (e: any) {
		editError.value = e?.message ?? '保存失败'
	} finally {
		editSaving.value = false
	}
}

function switchTab(tab: 'upload' | 'videos') {
	if (activeTab.value === tab) return
	if (activeTab.value === 'upload' && uploading.value) {
		if (!confirm('视频正在上传中，切换将取消上传，确定要离开吗？')) {
			return
		}
	}
	activeTab.value = tab
}

const preventUnload = (e: BeforeUnloadEvent) => {
	if (uploading.value) {
		e.preventDefault()
		e.returnValue = ''
	}
}

onUnmounted(() => {
	window.removeEventListener('beforeunload', preventUnload)
})

onBeforeRouteLeave((to, from, next) => {
	if (uploading.value) {
		if (confirm('视频正在上传中，离开页面将取消上传，确定要离开吗？')) {
			next()
		} else {
			next(false)
		}
	} else {
		next()
	}
})

onMounted(() => {
	window.addEventListener('beforeunload', preventUnload)
	if (activeTab.value === 'videos') {
		loadVideos()
	}
})

watch(activeTab, val => {
	if (val === 'videos' && videos.value.length === 0 && !listLoading.value) {
		loadVideos()
	}
})
</script>

<template>
	<BiliLayout>
		<div class="flex items-start gap-6">
			<aside class="w-56 space-y-4">
				<UiCard class="p-4 space-y-3">
					<div class="text-sm font-semibold">创作中心</div>
					<div class="text-xs text-gray-500 mt-1">内容管理</div>
					<button
						class="mt-1 w-full rounded px-3 py-1.5 text-left text-xs transition-colors"
						:class="
							activeTab === 'videos'
								? 'bg-blue-50 text-blue-600 font-medium'
								: 'text-gray-600 hover:bg-gray-50'
						"
						@click="switchTab('videos')"
					>
						视频管理
					</button>
					<div class="text-xs text-gray-500 mt-3">投稿</div>
					<button
						class="mt-1 w-full rounded px-3 py-1.5 text-left text-xs transition-colors"
						:class="
							activeTab === 'upload'
								? 'bg-pink-50 text-pink-600 font-medium'
								: 'text-gray-600 hover:bg-gray-50'
						"
						@click="switchTab('upload')"
					>
						视频投稿
					</button>
				</UiCard>
			</aside>
			<div class="flex-1 space-y-4">
				<template v-if="activeTab === 'upload'">
					<UiCard class="p-6">
						<h1 class="text-xl font-semibold mb-2">创作中心 · 视频投稿</h1>
						<p class="text-sm text-gray-500 mb-4">一个参考 Bilibili 风格的简洁投稿界面。</p>
						<div class="space-y-4">
							<div class="space-y-1">
								<div class="text-xs text-gray-500">稿件标题</div>
								<UiInput v-model="title" placeholder="填写视频标题" />
							</div>
							<div class="space-y-1">
								<div class="text-xs text-gray-500">稿件简介</div>
								<textarea
									v-model="description"
									rows="4"
									class="w-full text-sm rounded-md border border-[hsl(var(--input))] bg-white px-3 py-2 shadow-sm placeholder:text-gray-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[hsl(var(--ring))] focus-visible:ring-offset-0"
									placeholder="简单介绍一下你的视频内容"
								/>
							</div>
							<div class="space-y-2">
								<div class="text-xs text-gray-500">视频文件</div>
								<div
									class="flex items-center justify-between rounded-md border border-dashed border-pink-300 bg-pink-50 px-4 py-3 cursor-pointer hover:border-pink-400"
								>
									<div class="flex flex-col">
										<span class="text-sm font-medium text-pink-600">{{ selectedFileName }}</span>
										<span
											v-if="selectedFileSize"
											class="text-xs text-pink-400 mt-0.5"
										>
											{{ selectedFileSize }}
										</span>
									</div>
									<UiButton
										variant="outline"
										size="sm"
										class="border-pink-400 text-pink-500 hover:bg-pink-50"
										:disabled="uploading"
										@click.stop.prevent="triggerSelectFile"
									>
										选择视频
									</UiButton>
									<input
										type="file"
										accept="video/*"
										class="hidden"
										ref="fileInputRef"
										@change="onFileChange"
									/>
								</div>
							</div>
							<div v-if="uploading || progress > 0" class="space-y-2">
								<div class="flex items-center justify-between text-xs text-gray-500">
									<span>{{ statusText || '准备上传' }}</span>
									<span>{{ progress }}%</span>
								</div>
								<div class="h-2 rounded-full bg-gray-200 overflow-hidden">
									<div
										class="h-full bg-pink-500 transition-all"
										:style="{ width: progress + '%' }"
									/>
								</div>
							</div>
							<div v-if="errorMessage" class="text-xs text-red-500">
								{{ errorMessage }}
							</div>
							<div v-if="uploadedVideoId" class="text-xs text-green-600">
								已提交转码，视频 ID：{{ uploadedVideoId }}
							</div>
							<div class="pt-2">
								<UiButton
									class="w-full bg-pink-500 hover:bg-pink-600 text-white"
									size="lg"
									:disabled="!canUpload"
									@click="startUpload"
								>
									{{ uploading ? '上传中…' : '开始上传' }}
								</UiButton>
							</div>
						</div>
					</UiCard>
				</template>
				<template v-else>
					<UiCard class="p-6 space-y-4">
						<div class="flex items-center justify-between">
							<div>
								<h1 class="text-xl font-semibold">内容管理 · 视频管理</h1>
								<p class="text-xs text-gray-500 mt-1">管理你投稿的所有视频，查看状态与基础信息。</p>
							</div>
							<div class="flex items-center gap-2">
								<UiInput
									v-model="keyword"
									placeholder="搜索标题"
									class="h-9 w-56"
									@keyup.enter="onSearch"
								/>
								<UiButton size="sm" class="h-9" @click="onSearch">搜索</UiButton>
							</div>
						</div>
						<div v-if="listLoading" class="py-6 text-sm text-gray-500">正在加载视频列表…</div>
						<div v-else-if="listError" class="py-6 text-sm text-red-500">{{ listError }}</div>
					<div v-else-if="videos.length === 0" class="py-10 text-center text-sm text-gray-500">
						暂无视频稿件，先去右侧投稿一个吧。
					</div>
					<div v-else class="space-y-3">
						<CreatorVideoCard
							v-for="item in videos"
							:key="item.id"
							:video="item"
							@edit="openEditDialog"
						/>
					</div>
						<div
							v-if="total > pageSize"
							class="pt-4 flex items-center justify-between text-[11px] text-gray-500"
						>
							<div>共 {{ total }} 个视频</div>
							<div class="flex items-center gap-2">
								<UiButton
									variant="outline"
									size="sm"
									class="h-7 px-3 text-xs"
									:disabled="page === 1"
									@click="goPrevPage"
								>
									上一页
								</UiButton>
								<div>第 {{ page }} 页</div>
								<UiButton
									variant="outline"
									size="sm"
									class="h-7 px-3 text-xs"
									:disabled="page * pageSize >= total"
									@click="goNextPage"
								>
									下一页
								</UiButton>
							</div>
						</div>
					</UiCard>
				</template>
			</div>
		</div>
		<div
			v-if="editDialogOpen && editingVideo"
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		>
			<div class="w-full max-w-lg rounded-lg bg-white p-6 shadow-lg space-y-4">
				<div class="flex items-center justify-between mb-2">
					<h2 class="text-base font-semibold">视频管理</h2>
					<button class="text-xs text-gray-500" @click="editDialogOpen = false">关闭</button>
				</div>
				<div class="flex gap-4">
					<div class="w-40 flex-shrink-0">
						<div class="text-xs text-gray-500 mb-1">封面</div>
						<div
							class="relative w-full aspect-video overflow-hidden rounded border border-dashed border-gray-300 bg-gray-50 flex items-center justify-center cursor-pointer"
							@click="triggerSelectCover"
						>
							<img
								v-if="editCoverUrl"
								:src="editCoverUrl"
								alt="cover"
								class="h-full w-full object-cover"
							/>
							<span v-else class="text-[11px] text-gray-400">点击选择封面</span>
						</div>
						<input
							ref="coverInputRef"
							type="file"
							accept="image/*"
							class="hidden"
							@change="onCoverFileChange"
						/>
					</div>
					<div class="flex-1 space-y-3">
						<div class="space-y-1">
							<div class="text-xs text-gray-500">标题</div>
							<UiInput v-model="editTitle" placeholder="填写视频标题" />
						</div>
						<div class="space-y-1">
							<div class="text-xs text-gray-500">简介</div>
							<textarea
								v-model="editDescription"
								rows="4"
								class="w-full text-sm rounded-md border border-[hsl(var(--input))] bg-white px-3 py-2 shadow-sm placeholder:text-gray-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[hsl(var(--ring))] focus-visible:ring-offset-0"
							/>
						</div>
						<div class="space-y-1">
							<div class="text-xs text-gray-500">可见性</div>
							<div class="flex items-center gap-4 text-xs text-gray-600">
								<label class="flex items-center gap-1">
									<input
										type="radio"
										value="public"
										v-model="editVisibility"
									/>
									<span>公开</span>
								</label>
								<label class="flex items-center gap-1">
									<input
										type="radio"
										value="private"
										v-model="editVisibility"
									/>
									<span>仅自己可见</span>
								</label>
							</div>
						</div>
					</div>
				</div>
				<div v-if="editError" class="text-xs text-red-500">
					{{ editError }}
				</div>
				<div class="flex justify-between pt-2">
					<UiButton
						variant="ghost"
						size="sm"
						class="h-9 px-4 text-xs text-red-600 hover:text-red-700 hover:bg-red-50"
						@click="() => handleDeleteVideo(editingVideo!)"
					>
						删除视频
					</UiButton>
					<div class="flex gap-2">
						<UiButton
							variant="outline"
							size="sm"
							class="h-9 px-4 text-xs"
							@click="editDialogOpen = false"
						>
							取消
						</UiButton>
						<UiButton
							size="sm"
							class="h-9 px-4 text-xs bg-blue-600 hover:bg-blue-700 text-white"
							:disabled="editSaving"
							@click="saveEdit"
						>
							{{ editSaving ? '保存中…' : '保存' }}
						</UiButton>
					</div>
				</div>
			</div>
		</div>
		<div
			v-if="cropDialogOpen"
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
		>
			<div class="w-full max-w-2xl rounded-lg bg-white p-6 shadow-lg space-y-4">
				<div class="flex items-center justify-between mb-2">
					<h2 class="text-base font-semibold">调整封面</h2>
					<button class="text-xs text-gray-500" @click="cropDialogOpen = false">关闭</button>
				</div>
				<div
					ref="cropContainerRef"
					class="relative w-full max-w-xl mx-auto aspect-video overflow-hidden rounded bg-gray-900 flex items-center justify-center"
					@mousedown="onCropMouseDown"
					@mousemove="onCropMouseMove"
					@mouseup="onCropMouseUp"
					@mouseleave="onCropMouseUp"
				>
					<img
						v-if="cropPreviewUrl"
						:src="cropPreviewUrl"
						alt="crop"
						class="select-none pointer-events-none"
						:style="{
							transform: `translate(${cropOffsetX * 20}%, ${cropOffsetY * 20}%) scale(${cropScale})`,
						}"
					/>
					<span v-else class="text-xs text-gray-400">未选择图片</span>
				</div>
				<div class="space-y-2 pt-3">
					<div class="flex items-center justify-between text-xs text-gray-500">
						<span>缩放</span>
						<span>{{ cropScale.toFixed(2) }}x</span>
					</div>
					<input
						type="range"
						min="1"
						max="3"
						step="0.01"
						v-model.number="cropScale"
						class="w-full"
					/>
					<p class="text-[11px] text-gray-400">按住图片拖动，调整显示区域</p>
				</div>
				<div class="flex justify-end gap-2 pt-2">
					<UiButton
						variant="outline"
						size="sm"
						class="h-9 px-4 text-xs"
						:disabled="cropUploading"
						@click="cropDialogOpen = false"
					>
						取消
					</UiButton>
					<UiButton
						size="sm"
						class="h-9 px-4 text-xs bg-blue-600 hover:bg-blue-700 text-white"
						:disabled="cropUploading"
						@click="confirmCrop"
					>
						{{ cropUploading ? '上传中…' : '使用该封面' }}
					</UiButton>
				</div>
			</div>
		</div>
	</BiliLayout>
</template>

<style scoped>
</style>
