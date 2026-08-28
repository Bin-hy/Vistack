package danmaku

import "sync/atomic"

// acNode AC 自动机节点。
type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	isEnd    bool // 自身或 fail 链祖先是否为词尾（已做传播）
}

// SensitiveFilter AC 自动机敏感词过滤器。根节点用 atomic.Pointer 原子替换，支持并发无锁读 + 动态重建。
type SensitiveFilter struct {
	root atomic.Pointer[acNode]
}

// NewSensitiveFilter 用词表构建过滤器。
func NewSensitiveFilter(words []string) *SensitiveFilter {
	f := &SensitiveFilter{}
	f.Reload(words)
	return f
}

// Reload 重建 trie 并原子替换根（并发安全，新词立即生效）。
func (f *SensitiveFilter) Reload(words []string) {
	root := &acNode{children: make(map[rune]*acNode)}
	for _, w := range words {
		if w == "" {
			continue
		}
		cur := root
		for _, r := range w {
			next, ok := cur.children[r]
			if !ok {
				next = &acNode{children: make(map[rune]*acNode)}
				cur.children[r] = next
			}
			cur = next
		}
		cur.isEnd = true
	}
	buildFail(root)
	f.root.Store(root)
}

// buildFail BFS 构建 fail 链，并把词尾状态沿 fail 链传播到子节点。
func buildFail(root *acNode) {
	queue := make([]*acNode, 0, 16)
	for _, child := range root.children {
		child.fail = root
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for r, child := range cur.children {
			fail := cur.fail
			for fail != nil {
				if next, ok := fail.children[r]; ok {
					child.fail = next
					if next.isEnd {
						child.isEnd = true // 后缀命中传播
					}
					break
				}
				fail = fail.fail
			}
			if child.fail == nil {
				child.fail = root
			}
			queue = append(queue, child)
		}
	}
}

// Contains 判断文本是否包含任一敏感词。
func (f *SensitiveFilter) Contains(text string) bool {
	root := f.root.Load()
	if root == nil {
		return false
	}
	cur := root
	for _, r := range text {
		for cur != root && cur.children[r] == nil {
			cur = cur.fail
		}
		if next, ok := cur.children[r]; ok {
			cur = next
		}
		if cur.isEnd {
			return true
		}
	}
	return false
}
