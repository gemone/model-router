package compression

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gemone/model-router/internal/model"
)

// SemanticClustering implements similarity-based message clustering for compression
// NOTE: This is experimental and not yet integrated with the main compression pipeline.
// It defines its own Session and CompressedResult types which differ from the standard
// model.Session and CompressedContext types used elsewhere in the compression package.
type SemanticClustering struct {
	similarityThreshold float64 // Threshold for considering messages similar (0-1)
	tokenBudgetRatio    float64 // Ratio of context window to use for compressed output
}

// NewSemanticClustering creates a new semantic clustering compressor
func NewSemanticClustering() *SemanticClustering {
	return &SemanticClustering{
		similarityThreshold: 0.3, // 30% word overlap for similarity
		tokenBudgetRatio:    0.15, // 15% of context
	}
}

// Session represents a conversation session with messages
type Session struct {
	Messages []model.Message
}

// CompressedResult holds the compression output
type CompressedResult struct {
	Messages          []model.Message
	OriginalTokens    int
	CompressedTokens  int
	ReductionRatio    float64
}

// contentAsString extracts the string content from a model.Message
func contentAsString(msg model.Message) string {
	switch v := msg.Content.(type) {
	case string:
		return v
	case []interface{}:
		// Handle array of content parts (multimodal)
		var result string
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok {
					result += text + " "
				}
			}
		}
		return strings.TrimSpace(result)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// estimateTokens estimates token count for a message
func estimateTokens(msg model.Message) int {
	content := contentAsString(msg)
	// Rough estimate: 1 token per 4 characters
	return len(content) / 4
}

// Compress performs semantic clustering compression on the session
func (sc *SemanticClustering) Compress(session *Session, maxTokens int) (*CompressedResult, error) {
	if session == nil || len(session.Messages) == 0 {
		return &CompressedResult{
			Messages:          []model.Message{},
			OriginalTokens:    0,
			CompressedTokens:  0,
			ReductionRatio:    0,
		}, nil
	}

	// Calculate original token count
	originalTokens := 0
	for _, msg := range session.Messages {
		originalTokens += estimateTokens(msg)
	}

	// Calculate token budget (15% of max context)
	tokenBudget := int(float64(maxTokens) * sc.tokenBudgetRatio)

	// Cluster similar messages
	clusters := sc.clusterMessages(session.Messages)

	// Merge messages within each cluster
	mergedMessages := sc.mergeClusters(clusters)

	// Select messages to fit within token budget
	resultMessages := sc.selectWithinBudget(mergedMessages, tokenBudget)

	// Calculate compressed token count
	compressedTokens := 0
	for _, msg := range resultMessages {
		compressedTokens += estimateTokens(msg)
	}

	reductionRatio := 0.0
	if originalTokens > 0 {
		reductionRatio = 1.0 - float64(compressedTokens)/float64(originalTokens)
	}

	return &CompressedResult{
		Messages:    resultMessages,
		OriginalTokens: originalTokens,
		CompressedTokens: compressedTokens,
		ReductionRatio: reductionRatio,
	}, nil
}

// cluster groups similar messages together using similarity threshold
type cluster struct {
	Messages []model.Message
}

func (sc *SemanticClustering) clusterMessages(messages []model.Message) []cluster {
	var clusters []cluster

	for _, msg := range messages {
		// Try to add to existing cluster
		added := false
		for i := range clusters {
			representative := clusters[i].Messages[0]
			if sc.similarity(contentAsString(msg), contentAsString(representative)) >= sc.similarityThreshold {
				clusters[i].Messages = append(clusters[i].Messages, msg)
				added = true
				break
			}
		}

		// Create new cluster if not similar to any existing
		if !added {
			clusters = append(clusters, cluster{
				Messages: []model.Message{msg},
			})
		}
	}

	return clusters
}

// similarity computes Jaccard similarity between two text strings
func (sc *SemanticClustering) similarity(text1, text2 string) float64 {
	words1 := sc.tokenize(text1)
	words2 := sc.tokenize(text2)

	if len(words1) == 0 && len(words2) == 0 {
		return 1.0
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Calculate intersection
	intersection := 0
	wordSet2 := make(map[string]bool)
	for _, w := range words2 {
		wordSet2[w] = true
	}

	for _, w := range words1 {
		if wordSet2[w] {
			intersection++
		}
	}

	// Calculate union
	union := len(words1) + len(words2) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// tokenize splits text into words (lowercase, alphanumeric only)
func (sc *SemanticClustering) tokenize(text string) []string {
	words := make(map[string]bool)

	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	}

	parts := strings.FieldsFunc(text, f)
	for _, part := range parts {
		if len(part) > 2 { // Skip very short words
			words[strings.ToLower(part)] = true
		}
	}

	result := make([]string, 0, len(words))
	for w := range words {
		result = append(result, w)
	}

	return result
}

// mergeClusters combines messages within each cluster while preserving key information
func (sc *SemanticClustering) mergeClusters(clusters []cluster) []model.Message {
	result := make([]model.Message, 0, len(clusters))

	for _, c := range clusters {
		if len(c.Messages) == 0 {
			continue
		}

		if len(c.Messages) == 1 {
			result = append(result, c.Messages[0])
			continue
		}

		// Merge multiple messages
		merged := sc.mergeMessages(c.Messages)
		result = append(result, merged)
	}

	return result
}

// mergeMessages combines similar messages into one representative message
func (sc *SemanticClustering) mergeMessages(messages []model.Message) model.Message {
	if len(messages) == 0 {
		return model.Message{}
	}
	if len(messages) == 1 {
		return messages[0]
	}

	// Use the first message as base (most recent)
	base := messages[0]

	// For user messages, combine unique key points
	if base.Role == "user" {
		keyPoints := sc.extractKeyPoints(messages)
		baseContent := contentAsString(base)
		mergedContent := sc.formatMergedContent(baseContent, keyPoints)

		return model.Message{
			Role:    base.Role,
			Content: mergedContent,
		}
	}

	// For assistant/system messages, keep the most recent complete response
	return base
}

// extractKeyPoints identifies unique information across similar messages
func (sc *SemanticClustering) extractKeyPoints(messages []model.Message) []string {
	if len(messages) <= 1 {
		return nil
	}

	// Collect all unique sentences/phrases beyond the first message
	seen := make(map[string]bool)
	var keyPoints []string

	baseWords := sc.tokenize(contentAsString(messages[0]))
	baseWordSet := make(map[string]bool)
	for _, w := range baseWords {
		baseWordSet[w] = true
	}

	for i := 1; i < len(messages); i++ {
		content := contentAsString(messages[i])
		sentences := sc.splitSentences(content)

		for _, sentence := range sentences {
			// Check if sentence adds new information
			words := sc.tokenize(sentence)
			newWords := 0
			for _, w := range words {
				if !baseWordSet[w] {
					newWords++
				}
			}

			// Include if it has significant new content
			if newWords > 2 && len(sentence) > 10 {
				key := strings.TrimSpace(sentence)
				if key != "" && !seen[key] {
					seen[key] = true
					keyPoints = append(keyPoints, key)
				}
			}
		}
	}

	return keyPoints
}

// splitSentences roughly splits text into sentences
func (sc *SemanticClustering) splitSentences(text string) []string {
	var sentences []string
	var builder strings.Builder

	for _, r := range text {
		builder.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(builder.String())
			if len(s) > 0 {
				sentences = append(sentences, s)
			}
			builder.Reset()
		}
	}

	// Add remaining text
	if s := strings.TrimSpace(builder.String()); len(s) > 0 {
		sentences = append(sentences, s)
	}

	return sentences
}

// formatMergedContent combines base content with key points
func (sc *SemanticClustering) formatMergedContent(base string, keyPoints []string) string {
	if len(keyPoints) == 0 {
		return base
	}

	var builder strings.Builder
	builder.WriteString(base)

	// Add key points as additional context
	builder.WriteString(" [Additional context from similar messages: ")
	for i, point := range keyPoints {
		if i > 0 {
			builder.WriteString("; ")
		}
		// Truncate long points to avoid bloat
		if len(point) > 100 {
			point = point[:97] + "..."
		}
		builder.WriteString(point)
	}
	builder.WriteString("]")

	return builder.String()
}

// selectWithinBudget chooses messages that fit within the token budget
// Prioritizes recent messages and system messages
func (sc *SemanticClustering) selectWithinBudget(messages []model.Message, budget int) []model.Message {
	if len(messages) == 0 {
		return messages
	}

	// Calculate total tokens
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += estimateTokens(msg)
	}

	if totalTokens <= budget {
		return messages
	}

	// Need to select subset. Strategy: keep recent messages, prioritize system
	// Separate system messages from others
	var systemMessages []model.Message
	var regularMessages []model.Message

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			regularMessages = append(regularMessages, msg)
		}
	}

	// Always keep system messages
	systemTokens := 0
	for _, msg := range systemMessages {
		systemTokens += estimateTokens(msg)
	}

	remainingBudget := budget - systemTokens

	// Select recent regular messages to fit budget
	var selected []model.Message
	selected = append(selected, systemMessages...)

	// Add regular messages from most recent (assuming order is chronological)
	usedTokens := systemTokens
	for i := len(regularMessages) - 1; i >= 0; i-- {
		msg := regularMessages[i]
		msgTokens := estimateTokens(msg)
		if usedTokens + msgTokens <= remainingBudget {
			selected = append(selected, msg)
			usedTokens += msgTokens
		}
	}

	// Sort selected messages by role (system first, then chronological)
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Role == "system" && selected[j].Role != "system" {
			return true
		}
		if selected[i].Role != "system" && selected[j].Role == "system" {
			return false
		}
		return false
	})

	if len(selected) == 0 {
		// Fallback: at least keep the most recent message
		mostRecent := messages[len(messages)-1]
		return []model.Message{mostRecent}
	}

	return selected
}

// SetSimilarityThreshold updates the similarity threshold
func (sc *SemanticClustering) SetSimilarityThreshold(threshold float64) error {
	if threshold < 0 || threshold > 1 {
		return fmt.Errorf("similarity threshold must be between 0 and 1")
	}
	sc.similarityThreshold = threshold
	return nil
}

// SetTokenBudgetRatio updates the token budget ratio
func (sc *SemanticClustering) SetTokenBudgetRatio(ratio float64) error {
	if ratio <= 0 || ratio > 1 {
		return fmt.Errorf("token budget ratio must be between 0 and 1")
	}
	sc.tokenBudgetRatio = ratio
	return nil
}
