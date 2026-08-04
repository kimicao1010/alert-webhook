package models

import (
	"strconv"
	"strings"
)

// extractByPath 从嵌套 JSON 节点按路径提取值。
// 路径语法：点分段 + 可选数组下标，如 "alerts[0].labels.severity"、"receiver"、"alerts[0]"。
// 任一分段缺失或类型不匹配时返回 nil（不报错），与 missingkey=zero 语义一致。
func extractByPath(node any, path string) any {
	segments := parsePath(path)
	for _, seg := range segments {
		node = descend(node, seg)
		if node == nil {
			return nil
		}
	}
	return node
}

// pathSegment 一个路径段：对象键或数组下标。
type pathSegment struct {
	key     string
	index   int // 仅当 isIndex 为 true 时有效
	isIndex bool
}

// parsePath 解析 "a.b[0].c" 为段序列。
func parsePath(path string) []pathSegment {
	raw := strings.Split(path, ".")
	segs := make([]pathSegment, 0, len(raw))
	for _, part := range raw {
		if part == "" {
			continue
		}
		// 处理形如 b[0]、b[1] 的数组下标
		if i := strings.IndexByte(part, '['); i >= 0 {
			key := part[:i]
			if key != "" {
				segs = append(segs, pathSegment{key: key})
			}
			for _, rest := range splitBrackets(part[i:]) {
				if n, err := strconv.Atoi(rest); err == nil {
					segs = append(segs, pathSegment{index: n, isIndex: true})
				}
			}
			continue
		}
		segs = append(segs, pathSegment{key: part})
	}
	return segs
}

// splitBrackets 提取 "[0][1]" 中的数字。
func splitBrackets(s string) []string {
	var res []string
	for {
		open := strings.IndexByte(s, '[')
		if open < 0 {
			break
		}
		close := strings.IndexByte(s[open:], ']')
		if close < 0 {
			break
		}
		res = append(res, s[open+1:open+close])
		s = s[open+close+1:]
	}
	return res
}

// descend 沿单个段下探。
func descend(node any, seg pathSegment) any {
	if seg.isIndex {
		arr, ok := node.([]any)
		if !ok || seg.index < 0 || seg.index >= len(arr) {
			return nil
		}
		return arr[seg.index]
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	return m[seg.key]
}