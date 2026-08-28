<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { UiIcon } from '@/components/ui'
import type { DanmakuEngine } from './useDanmaku'
import {
	DANMAKU_AREA_OPTIONS,
	DANMAKU_FONT_OPTIONS,
	DANMAKU_MODE_LABELS,
	DANMAKU_PRESET_COLORS,
} from './types'

const props = defineProps<{ engine: DanmakuEngine }>()

const {
	settings,
	count,
	inputText,
	inputMode,
	inputColor,
	setMode,
	setColor,
	toggleEnabled,
	send,
} = props.engine

const settingsOpen = ref(false)
const colorOpen = ref(false)
const rootRef = ref<HTMLElement | null>(null)

function onCustomColor(e: Event) {
	setColor((e.target as HTMLInputElement).value)
}

function handleClickOutside(e: MouseEvent) {
	const el = rootRef.value
	if (el && !el.contains(e.target as Node)) {
		settingsOpen.value = false
		colorOpen.value = false
	}
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))
</script>

<template>
	<div ref="rootRef" class="relative mt-1 md:mt-2">
		<div class="flex items-center gap-1.5 md:gap-2">
			<!-- 弹幕开关 -->
			<button
				class="flex items-center rounded-full border px-2 md:px-3 py-0.5 md:py-1 text-xs whitespace-nowrap transition-colors"
				:class="settings.enabled ? 'border-primary/50 bg-primary/15 text-primary' : 'border-white/30 bg-black/40 text-white/70'"
				type="button"
				@click="toggleEnabled"
			>
				<UiIcon name="message-square" :size="13" class="mr-1 hidden sm:block" />
				<span>{{ settings.enabled ? '弹幕开' : '弹幕关' }}</span>
			</button>

			<!-- 颜色选择 -->
			<div class="relative">
				<button
					class="flex items-center gap-1 rounded bg-white/10 px-2 py-0.5 md:py-1 text-xs text-white transition-colors hover:bg-white/25"
					type="button"
					@click="colorOpen = !colorOpen; settingsOpen = false"
				>
					<span class="h-4 w-4 rounded-full ring-1 ring-white/40" :style="{ backgroundColor: inputColor }"></span>
					<UiIcon name="chevron-down" :size="12" />
				</button>
				<Transition name="pop">
					<div
						v-if="colorOpen"
						class="absolute bottom-8 left-0 z-30 w-44 rounded-lg bg-black/95 py-2 shadow-xl ring-1 ring-white/10"
					>
						<div class="grid grid-cols-5 gap-1.5 px-2">
							<button
								v-for="c in DANMAKU_PRESET_COLORS"
								:key="c"
								type="button"
								class="h-6 w-6 rounded-full transition-transform hover:scale-110"
								:class="inputColor === c ? 'ring-2 ring-white ring-offset-1 ring-offset-black' : 'ring-1 ring-white/20'"
								:style="{ backgroundColor: c }"
								@click="setColor(c); colorOpen = false"
							></button>
						</div>
						<div class="mt-2 flex items-center justify-between border-t border-white/10 px-3 pt-2">
							<span class="text-[11px] text-white/60">自定义</span>
							<input
								type="color"
								:value="inputColor"
								class="h-6 w-8 cursor-pointer rounded border-0 bg-transparent p-0"
								@input="onCustomColor"
							/>
						</div>
					</div>
				</Transition>
			</div>

			<!-- 模式选择 -->
			<div class="flex rounded bg-white/10 p-0.5">
				<button
					v-for="m in DANMAKU_MODE_LABELS"
					:key="m.value"
					type="button"
					class="rounded px-2 py-0.5 text-xs transition-colors"
					:class="inputMode === m.value ? 'bg-white/25 text-white' : 'text-white/60 hover:text-white'"
					@click="setMode(m.value)"
				>
					{{ m.label }}
				</button>
			</div>

			<!-- 输入框 -->
			<input
				v-model="inputText"
				class="h-7 md:h-8 flex-1 rounded border border-white/20 bg-black/40 px-2 md:px-3 text-xs text-white outline-none transition-colors placeholder:text-white/40 focus:border-primary/60"
				placeholder="发个弹幕，一起讨论…"
				@keyup.enter="send"
			/>

			<!-- 发送 -->
			<button
				class="flex h-7 md:h-8 items-center gap-1 rounded bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] px-3 md:px-4 text-xs font-medium text-white transition-all hover:brightness-110 whitespace-nowrap"
				type="button"
				@click="send"
			>
				<UiIcon name="send" :size="13" />
				<span>发送</span>
			</button>

			<!-- 设置 -->
			<button
				class="flex h-7 md:h-8 w-7 md:w-8 items-center justify-center rounded bg-white/10 text-white transition-colors hover:bg-white/25"
				type="button"
				@click="settingsOpen = !settingsOpen; colorOpen = false"
			>
				<UiIcon name="settings" :size="14" />
			</button>
		</div>

		<!-- 弹幕设置面板 -->
		<Transition name="pop">
			<div
				v-if="settingsOpen"
				class="absolute bottom-10 right-0 z-30 w-64 rounded-xl bg-black/95 p-4 text-xs shadow-2xl ring-1 ring-white/10"
			>
				<div class="mb-3 flex items-center justify-between">
					<span class="font-medium text-white">弹幕设置</span>
					<span class="text-[11px] text-white/50">共 {{ count }} 条</span>
				</div>

				<!-- 不透明度 -->
				<div class="mb-3">
					<div class="mb-1 flex items-center justify-between">
						<span class="text-white/70">不透明度</span>
						<span class="tabular-nums text-white/50">{{ Math.round(settings.opacity * 100) }}%</span>
					</div>
					<input v-model.number="settings.opacity" type="range" min="0.1" max="1" step="0.05" class="w-full" />
				</div>

				<!-- 显示区域 -->
				<div class="mb-3">
					<div class="mb-1.5 text-white/70">显示区域</div>
					<div class="grid grid-cols-4 gap-1">
						<button
							v-for="a in DANMAKU_AREA_OPTIONS"
							:key="a.value"
							type="button"
							class="rounded px-1 py-1 text-[11px] transition-colors"
							:class="settings.area === a.value ? 'bg-primary/30 text-white' : 'bg-white/10 text-white/60 hover:bg-white/20'"
							@click="settings.area = a.value"
						>
							{{ a.label }}
						</button>
					</div>
				</div>

				<!-- 字号 -->
				<div class="mb-3">
					<div class="mb-1.5 text-white/70">字号</div>
					<div class="grid grid-cols-3 gap-1">
						<button
							v-for="f in DANMAKU_FONT_OPTIONS"
							:key="f.value"
							type="button"
							class="rounded px-1 py-1 text-[11px] transition-colors"
							:class="settings.fontSize === f.value ? 'bg-primary/30 text-white' : 'bg-white/10 text-white/60 hover:bg-white/20'"
							@click="settings.fontSize = f.value"
						>
							{{ f.label }}
						</button>
					</div>
				</div>

				<!-- 滚动速度 -->
				<div>
					<div class="mb-1 flex items-center justify-between">
						<span class="text-white/70">滚动速度</span>
						<span class="tabular-nums text-white/50">{{ settings.speed }}s</span>
					</div>
					<input v-model.number="settings.speed" type="range" min="4" max="12" step="1" class="w-full" />
				</div>
			</div>
		</Transition>
	</div>
</template>

<style scoped>
.pop-enter-active,
.pop-leave-active {
	transition: opacity 0.15s ease, transform 0.15s ease;
}
.pop-enter-from,
.pop-leave-to {
	opacity: 0;
	transform: translateY(6px);
}
</style>
