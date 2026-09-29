package api

import "strconv"

// ParsePagination 解析并钳制分页参数，避免 page<1 或 pageSize<1 导致的非法 OFFSET 与除零。
func ParsePagination(pageStr, sizeStr string) (page, pageSize int) {
	page, _ = strconv.Atoi(pageStr)
	pageSize, _ = strconv.Atoi(sizeStr)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
