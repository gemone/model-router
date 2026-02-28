//go:build !embed
// +build !embed

package web

import (
	"net/http"
)

// FS 开发模式下返回 nil，使用代理
func FS() http.FileSystem {
	return nil
}

// DevMode 返回是否为开发模式
func DevMode() bool {
	return true
}
