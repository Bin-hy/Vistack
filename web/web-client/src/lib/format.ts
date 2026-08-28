// 计数格式化：>=1亿 显示 x.x亿，>=1万 显示 x.x万，否则原样。
export function formatCount(n?: number | null): string {
	if (!n || n <= 0) return '0'
	if (n >= 100000000) return trimZero((n / 100000000).toFixed(1)) + '亿'
	if (n >= 10000) return trimZero((n / 10000).toFixed(1)) + '万'
	return String(n)
}

function trimZero(s: string): string {
	return s.replace(/\.0$/, '')
}
