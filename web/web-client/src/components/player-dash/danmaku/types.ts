// 弹幕模式：与后端约定一致 —— 0 滚动 / 1 顶部 / 2 底部
export type DanmakuMode = 0 | 1 | 2

export interface DanmakuItem {
	time_offset: number
	content: string
	color?: string
	mode?: number
}

export interface DanmakuSettings {
	enabled: boolean
	opacity: number
	area: number // 1 => 1/4屏, 2 => 1/2屏, 3 => 3/4屏, 4 => 全屏
	fontSize: 'small' | 'normal' | 'large'
	speed: number // 滚动弹幕穿越屏幕所需秒数，越小越快
}

// B 站风格的预设弹幕颜色
export const DANMAKU_PRESET_COLORS = [
	'#FFFFFF',
	'#FF5C5C',
	'#FF8A2E',
	'#FFE14D',
	'#7CFC00',
	'#00E5FF',
	'#4D9FFF',
	'#B388FF',
	'#FF6EB4',
	'#FFD700',
]

export const DANMAKU_MODE_LABELS: { value: DanmakuMode; label: string }[] = [
	{ value: 0, label: '滚动' },
	{ value: 1, label: '顶部' },
	{ value: 2, label: '底部' },
]

export const DANMAKU_AREA_OPTIONS: { value: number; label: string }[] = [
	{ value: 1, label: '1/4屏' },
	{ value: 2, label: '1/2屏' },
	{ value: 3, label: '3/4屏' },
	{ value: 4, label: '全屏' },
]

export const DANMAKU_FONT_OPTIONS: { value: DanmakuSettings['fontSize']; label: string }[] = [
	{ value: 'small', label: '小' },
	{ value: 'normal', label: '中' },
	{ value: 'large', label: '大' },
]
