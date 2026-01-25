<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BiliLayout from '@/layouts/BiliLayout.vue'
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
            <h1 class="text-xl font-semibold mb-4 flex items-center gap-2">
                <span class="text-pink-500">🔥</span> 热门推荐
            </h1>
            
            <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4 sm:gap-6">
                 <div v-for="i in 10" :key="i" class="space-y-2">
                    <div class="aspect-video bg-gray-200 rounded-lg animate-pulse"></div>
                    <div class="h-4 bg-gray-200 rounded w-3/4 animate-pulse"></div>
                    <div class="h-3 bg-gray-200 rounded w-1/2 animate-pulse"></div>
                 </div>
            </div>
            
            <div v-else-if="videos.length > 0" class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4 sm:gap-6">
                <VideoCard 
                    v-for="item in videos" 
                    :key="item.id" 
                    :video="item" 
                />
            </div>
            
            <div v-else class="flex flex-col items-center justify-center py-20 text-gray-500">
                <p>暂无推荐内容</p>
            </div>
        </div>
	</BiliLayout>
</template>

<style scoped>
</style>
