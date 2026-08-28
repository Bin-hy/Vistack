<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BiliLayout from '@/layouts/BiliLayout.vue'
import { UiIcon } from '@/components/ui'
import { getVideoRecommend, type VideoItem } from './api/api'
import VideoCard from '@/components/video/VideoCard.vue'

const videos = ref<VideoItem[]>([])
const loading = ref(true)

async function loadData() {
    loading.value = true
    try {
        const res = await getVideoRecommend()
        videos.value = res.videos || []
    } catch (error) {
        console.error('load recommend failed', error)
    } finally {
        loading.value = false
    }
}

onMounted(() => {
    loadData()
})
</script>

<template>
	<BiliLayout>
        <div class="py-4">
            <h1 class="mb-4 flex items-center gap-2 text-xl font-semibold">
                <UiIcon name="star" :size="20" class="text-primary" /> 热门推荐
            </h1>

            <div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                 <div v-for="i in 10" :key="i" class="space-y-2">
                    <div class="aspect-video animate-pulse rounded-lg bg-secondary"></div>
                    <div class="h-4 w-3/4 animate-pulse rounded bg-secondary"></div>
                    <div class="h-3 w-1/2 animate-pulse rounded bg-secondary"></div>
                 </div>
            </div>

            <div v-else-if="videos.length > 0" class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                <VideoCard
                    v-for="item in videos"
                    :key="item.id"
                    :video="item"
                />
            </div>

            <div v-else class="flex flex-col items-center justify-center py-20 text-muted-foreground">
                <p>暂无推荐内容</p>
            </div>
        </div>
	</BiliLayout>
</template>

<style scoped>
</style>
