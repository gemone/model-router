package proxy

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHopHeader(t *testing.T) {
	tests := []struct {
		header string
		isHop  bool
	}{
		{"Connection", true},
		{"Keep-Alive", true},
		{"Proxy-Authenticate", true},
		{"Proxy-Authorization", true},
		{"Te", true},
		{"Trailers", true},
		{"Transfer-Encoding", true},
		{"Upgrade", true},
		{"Content-Type", false},
		{"Authorization", false},
		{"X-Custom-Header", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := isHopHeader(tt.header)
			assert.Equal(t, tt.isHop, result)
		})
	}
}

func TestDelHopHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Connection", "keep-alive")
	header.Set("Authorization", "Bearer token")
	header.Set("Transfer-Encoding", "chunked")

	delHopHeaders(header)

	assert.Equal(t, "application/json", header.Get("Content-Type"))
	assert.Equal(t, "Bearer token", header.Get("Authorization"))
	assert.Empty(t, header.Get("Connection"))
	assert.Empty(t, header.Get("Transfer-Encoding"))
}

func TestCopyHeaders(t *testing.T) {
	src := http.Header{}
	src.Set("Content-Type", "application/json")
	src.Set("Authorization", "Bearer token")
	src.Set("Connection", "keep-alive") // should be skipped

	dst := http.Header{}
	copyHeaders(dst, src)

	assert.Equal(t, "application/json", dst.Get("Content-Type"))
	assert.Equal(t, "Bearer token", dst.Get("Authorization"))
	assert.Empty(t, dst.Get("Connection"))
}

func TestGetContentEncoding(t *testing.T) {
	tests := []struct {
		data     []byte
		expected string
	}{
		{[]byte{0x1f, 0x8b, 0x08, 0x00}, "gzip"},
		{[]byte{0x1f, 0x8b}, "gzip"},
		{[]byte{0x00, 0x00}, ""},
		{[]byte{}, ""},
		{[]byte{0x78, 0x9c}, ""}, // zlib but not gzip
	}

	for _, tt := range tests {
		t.Run(string(tt.data), func(t *testing.T) {
			result := GetContentEncoding(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}
