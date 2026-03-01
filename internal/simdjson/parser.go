// Package simdjson provides fast JSON parsing using Sonic.
// Target: ≥500MB/s throughput, sub-50ms for 1MB payloads.
package simdjson

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"

	"github.com/bytedance/sonic"
)

// Parser handles JSON parsing with Sonic and encoding/json fallback.
type Parser struct {
	// Configurable options
	useSonic bool
}

// NewParser creates a new Sonic JSON parser with fallback enabled.
func NewParser() *Parser {
	return &Parser{
		useSonic: true,
	}
}

// NewParserWithOptions creates a parser with specified options.
func NewParserWithOptions(useSonic bool) *Parser {
	return &Parser{
		useSonic: useSonic,
	}
}

// ParseRequestBody parses JSON from an io.Reader.
// Attempts Sonic first, falls back to encoding/json on error.
func (p *Parser) ParseRequestBody(r io.Reader) (map[string]interface{}, error) {
	// Use buffer pool for efficient memory reuse
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if _, err := io.Copy(buf, r); err != nil {
		return nil, err
	}

	return p.ParseRequestBodyBytes(buf.Bytes())
}

// ParseRequestBodyBytes parses JSON from a byte slice.
// Attempts Sonic first, falls back to encoding/json on error.
func (p *Parser) ParseRequestBodyBytes(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}

	if p.useSonic {
		result, err := p.parseWithSonic(data)
		if err == nil {
			return result, nil
		}
		// Fall through to encoding/json on Sonic failure
	}

	return p.parseWithEncodingJSON(data)
}

// parseWithSonic uses Sonic for high-performance JSON parsing.
func (p *Parser) parseWithSonic(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := sonic.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// parseWithEncodingJSON uses encoding/json as fallback.
func (p *Parser) parseWithEncodingJSON(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Marshal converts a map to JSON bytes using Sonic.
func (p *Parser) Marshal(v map[string]interface{}) ([]byte, error) {
	if p.useSonic {
		data, err := sonic.Marshal(v)
		if err == nil {
			return data, nil
		}
		// Fall through to encoding/json on Sonic failure
	}
	return json.Marshal(v)
}

// MarshalToString converts a map to JSON string using Sonic.
func (p *Parser) MarshalToString(v map[string]interface{}) (string, error) {
	if p.useSonic {
		data, err := sonic.Marshal(v)
		if err == nil {
			return string(data), nil
		}
		// Fall through to encoding/json on Sonic failure
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// bufferPool provides reusable buffers for efficient I/O.
var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}
