//go:build !embed
// +build !embed

package web

import (
	"io/fs"
)

// FS 开发模式下返回 nil，使用代理
func FS() (fs.FS, error) {
	return nil, nil
}

// DevMode 返回是否为开发模式
func DevMode() bool {
	return true
}
