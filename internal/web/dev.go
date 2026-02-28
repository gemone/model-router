//go:build !embed
// +build !embed

package web

import (
	"io/fs"
)

// FS 开发模式下返回 nil，使用代理
func FS() fs.FS {
	return nil
}

// DevMode 返回是否为开发模式
func DevMode() bool {
	return true
}
