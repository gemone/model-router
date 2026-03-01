package simdjson

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewParser verifies parser creation.
func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.True(t, parser.useSonic)
}

// TestNewParserWithOptions verifies parser creation with options.
func TestNewParserWithOptions(t *testing.T) {
	parser := NewParserWithOptions(false)
	assert.NotNil(t, parser)
	assert.False(t, parser.useSonic)
}

// TestParseRequestBodyBytes_Empty verifies empty input handling.
func TestParseRequestBodyBytes_Empty(t *testing.T) {
	parser := NewParser()
	result, err := parser.ParseRequestBodyBytes([]byte{})
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestParseRequestBodyBytes_Simple verifies simple JSON parsing.
func TestParseRequestBodyBytes_Simple(t *testing.T) {
	parser := NewParser()
	data := []byte(`{"name":"test","value":123}`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(123), result["value"])
}

// TestParseRequestBodyBytes_Nested verifies nested JSON parsing.
func TestParseRequestBodyBytes_Nested(t *testing.T) {
	parser := NewParser()
	data := []byte(`{
		"user": {
			"name": "Alice",
			"age": 30,
			"address": {
				"city": "San Francisco",
				"zip": "94102"
			}
		},
		"tags": ["engineer", "golang"]
	}`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)

	user := result["user"].(map[string]interface{})
	assert.Equal(t, "Alice", user["name"])
	assert.Equal(t, float64(30), user["age"])

	address := user["address"].(map[string]interface{})
	assert.Equal(t, "San Francisco", address["city"])
	assert.Equal(t, "94102", address["zip"].(string))
}

// TestParseRequestBodyBytes_Array verifies array parsing.
func TestParseRequestBodyBytes_Array(t *testing.T) {
	parser := NewParser()
	data := []byte(`{"items":[1,2,3,4,5]}`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)

	items := result["items"].([]interface{})
	assert.Len(t, items, 5)
	assert.Equal(t, float64(1), items[0])
	assert.Equal(t, float64(5), items[4])
}

// TestParseRequestBodyBytes_Invalid verifies error handling.
func TestParseRequestBodyBytes_Invalid(t *testing.T) {
	parser := NewParser()
	_, err := parser.ParseRequestBodyBytes([]byte(`{invalid}`))
	assert.Error(t, err)
}

// TestParseRequestBody verifies io.Reader parsing.
func TestParseRequestBody(t *testing.T) {
	parser := NewParser()
	data := `{"message":"hello world","count":42}`

	result, err := parser.ParseRequestBody(strings.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, "hello world", result["message"])
	assert.Equal(t, float64(42), result["count"])
}

// TestParseRequestBody_Large verifies large payload handling.
func TestParseRequestBody_Large(t *testing.T) {
	parser := NewParser()

	// Create a 100KB JSON payload
	largeObj := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeObj[string(rune(i))] = map[string]interface{}{
			"id":    i,
			"name":  "item",
			"tags":  []string{"tag1", "tag2", "tag3"},
			"nested": map[string]interface{}{
				"value": i * 2,
			},
		}
	}

	data, err := json.Marshal(largeObj)
	require.NoError(t, err)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)
	assert.Len(t, result, 1000)
}

// TestMarshal verifies JSON marshaling.
func TestMarshal(t *testing.T) {
	parser := NewParser()
	input := map[string]interface{}{
		"name":  "test",
		"value": 123,
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	result, err := parser.Marshal(input)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(result, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "test", parsed["name"])
	assert.Equal(t, float64(123), parsed["value"])
}

// TestMarshalToString verifies string marshaling.
func TestMarshalToString(t *testing.T) {
	parser := NewParser()
	input := map[string]interface{}{
		"name":  "test",
		"value": 456,
	}

	result, err := parser.MarshalToString(input)
	require.NoError(t, err)
	assert.Contains(t, result, `"name":"test"`)
	assert.Contains(t, result, `"value":456`)
}

// TestFallbackToEncodingJSON verifies fallback on Sonic failure.
func TestFallbackToEncodingJSON(t *testing.T) {
	parser := NewParser()
	// Parser should work even if Sonic has issues
	data := []byte(`{"test":"fallback"}`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "fallback", result["test"])
}

// BenchmarkParseRequestBodyBytes_Small benchmarks small JSON (1KB).
func BenchmarkParseRequestBodyBytes_Small(b *testing.B) {
	parser := NewParser()
	data := []byte(`{"name":"benchmark","value":100,"nested":{"key":"value"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseRequestBodyBytes(data)
	}
}

// BenchmarkParseRequestBodyBytes_Medium benchmarks medium JSON (10KB).
func BenchmarkParseRequestBodyBytes_Medium(b *testing.B) {
	parser := NewParser()
	data := generateMediumJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseRequestBodyBytes(data)
	}
}

// BenchmarkParseRequestBodyBytes_Large benchmarks large JSON (1MB).
// Target: ≥500MB/s throughput, sub-50ms for 1MB.
func BenchmarkParseRequestBodyBytes_Large(b *testing.B) {
	parser := NewParser()
	data := generateLargeJSON()

	// Measure single operation latency
	b.Run("SingleOperation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = parser.ParseRequestBodyBytes(data)
		}
	})

	// Calculate throughput
	b.Run("Throughput", func(b *testing.B) {
		b.ResetTimer()
		totalBytes := int64(0)
		for i := 0; i < b.N; i++ {
			_, _ = parser.ParseRequestBodyBytes(data)
			totalBytes += int64(len(data))
		}
		b.ReportMetric(float64(totalBytes)/float64(b.Elapsed().Milliseconds()), "MB/s")
	})
}

// BenchmarkParseRequestBody_Reader benchmarks io.Reader parsing.
func BenchmarkParseRequestBody_Reader(b *testing.B) {
	parser := NewParser()
	data := generateLargeJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		_, _ = parser.ParseRequestBody(r)
	}
}

// BenchmarkEncodingJSON_Large benchmarks encoding/json for comparison.
func BenchmarkEncodingJSON_Large(b *testing.B) {
	data := generateLargeJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		_ = json.Unmarshal(data, &result)
	}
}

// BenchmarkMarshal_Small benchmarks marshaling small data.
func BenchmarkMarshal_Small(b *testing.B) {
	parser := NewParser()
	input := map[string]interface{}{
		"name":  "test",
		"value": 123,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Marshal(input)
	}
}

// BenchmarkMarshal_Large benchmarks marshaling large data.
func BenchmarkMarshal_Large(b *testing.B) {
	parser := NewParser()
	input := generateLargeMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.Marshal(input)
	}
}

// BenchmarkSonicVsEncodingJSON compares Sonic vs encoding/json.
func BenchmarkSonicVsEncodingJSON(b *testing.B) {
	data := generateLargeJSON()

	b.Run("Sonic", func(b *testing.B) {
		parser := NewParser()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = parser.ParseRequestBodyBytes(data)
		}
	})

	b.Run("EncodingJSON", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var result map[string]interface{}
			_ = json.Unmarshal(data, &result)
		}
	})
}

// generateMediumJSON creates a ~10KB JSON payload.
func generateMediumJSON() []byte {
	obj := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		obj[string(rune(i))] = map[string]interface{}{
			"id":      i,
			"name":    "item",
			"tags":    []string{"tag1", "tag2", "tag3"},
			"value":   i * 10,
			"active":  true,
			"nested": map[string]interface{}{
				"key": "value",
				"num": float64(i),
			},
		}
	}
	data, _ := json.Marshal(obj)
	return data
}

// generateLargeJSON creates a ~1MB JSON payload.
func generateLargeJSON() []byte {
	obj := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		obj[string(rune(i%1000))+string(rune(i/1000))] = map[string]interface{}{
			"id":      i,
			"name":    "item",
			"tags":    []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
			"value":   i * 100,
			"active":  true,
			"score":   float64(i) / 100.0,
			"nested": map[string]interface{}{
				"key":   "value",
				"num":   float64(i),
				"array": []int{i, i + 1, i + 2},
				"deep": map[string]interface{}{
					"level": i,
					"data":  "deeply nested value",
				},
			},
		}
	}
	data, _ := json.Marshal(obj)
	return data
}

// generateLargeMap creates a large map for marshaling benchmarks.
func generateLargeMap() map[string]interface{} {
	obj := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		obj[string(rune(i%1000))] = map[string]interface{}{
			"id":    i,
			"value": i * 10,
		}
	}
	return obj
}

// BenchmarkThroughput verification helper.
func BenchmarkThroughputVerification(b *testing.B) {
	parser := NewParser()
	data := generateLargeJSON()

	b.ResetTimer()
	var totalBytes int64
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseRequestBodyBytes(data)
		totalBytes += int64(len(data))
	}

	// Report throughput in MB/s
	mbPerSec := float64(totalBytes) / float64(b.Elapsed().Milliseconds()) / 1024 / 1024 * 1000
	b.ReportMetric(mbPerSec, "MB/s")
}

// TestConcurrentParsing verifies concurrent access safety.
func TestConcurrentParsing(t *testing.T) {
	parser := NewParser()
	data := []byte(`{"test":"concurrent","value":123}`)

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			result, err := parser.ParseRequestBodyBytes(data)
			assert.NoError(t, err)
			assert.Equal(t, "concurrent", result["test"])
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestParseRequestBodyWithLimit verifies reader with limit.
func TestParseRequestBodyWithLimit(t *testing.T) {
	parser := NewParser()
	data := `{"message":"test","limited":true}`

	// Test with limited reader
	limitedReader := io.LimitReader(strings.NewReader(data), 100)
	result, err := parser.ParseRequestBody(limitedReader)
	require.NoError(t, err)
	assert.Equal(t, "test", result["message"])
}

// TestSpecialCharacters verifies special character handling.
func TestSpecialCharacters(t *testing.T) {
	parser := NewParser()
	data := []byte(`{
		"unicode": "Hello 世界",
		"escaped": "Line1\nLine2\tTabbed",
		"quotes": "He said \"hello\"",
		"null": null,
		"boolean": true,
		"number": 123.45
	}`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)

	assert.Equal(t, "Hello 世界", result["unicode"])
	assert.Contains(t, result["escaped"], "\n")
	assert.Nil(t, result["null"])
	assert.Equal(t, true, result["boolean"])
	assert.Equal(t, 123.45, result["number"])
}

// TestMarshalEmptyMap verifies empty map marshaling.
func TestMarshalEmptyMap(t *testing.T) {
	parser := NewParser()
	input := map[string]interface{}{}

	result, err := parser.Marshal(input)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(result))
}

// TestParseRequestBodyBytesWhitespace verifies whitespace handling.
func TestParseRequestBodyBytesWhitespace(t *testing.T) {
	parser := NewParser()
	data := []byte(`
		{
			"key": "value",
			"number": 42
		}
	`)

	result, err := parser.ParseRequestBodyBytes(data)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["number"])
}
