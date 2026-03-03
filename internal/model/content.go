package model

// HasImageContent 检查消息列表是否包含图像内容
func HasImageContent(messages []Message) bool {
	for _, msg := range messages {
		if hasImageInContent(msg.Content) {
			return true
		}
	}
	return false
}

// hasImageInContent 检查单个消息内容是否包含图像
func hasImageInContent(content interface{}) bool {
	if content == nil {
		return false
	}

	// 检查是否为 ContentPart 数组（多模态内容）
	switch v := content.(type) {
	case []interface{}:
		for _, item := range v {
			if part, ok := item.(map[string]interface{}); ok {
				if partType, ok := part["type"].(string); ok && partType == "image_url" {
					return true
				}
			}
		}
	case []ContentPart:
		for _, part := range v {
			if part.Type == "image_url" {
				return true
			}
		}
	case string:
		// 纯文本内容，不包含图像
		return false
	}

	return false
}
