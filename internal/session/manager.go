// Package session provides session and context management.
package session

import (
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"gorm.io/gorm"
)

// Manager manages user sessions
type Manager struct {
	db *gorm.DB
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		db: database.GetDB(),
	}
}

// LoadSessionMessages loads all messages for a session from the database
func (m *Manager) LoadSessionMessages(sessionID string) ([]model.Message, error) {
	if sessionID == "" {
		return []model.Message{}, nil
	}

	var sessionMessages []model.SessionMessage
	err := m.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&sessionMessages).Error
	if err != nil {
		return nil, err
	}

	// Convert SessionMessage to model.Message
	messages := make([]model.Message, len(sessionMessages))
	for i, sm := range sessionMessages {
		messages[i] = model.Message{
			Role:    sm.Role,
			Content: sm.Content,
		}
	}

	return messages, nil
}

// SaveSessionMessage saves a message to the database
func (m *Manager) SaveSessionMessage(sessionID string, msg model.Message, tokens int) error {
	if sessionID == "" {
		return nil // Skip saving if no session
	}

	sessionMsg := &model.SessionMessage{
		SessionID: sessionID,
		Role:      msg.Role,
		Content:   m.contentToString(msg.Content),
		Tokens:    tokens,
	}

	return m.db.Create(sessionMsg).Error
}

// contentToString converts Message Content (which can be string or interface{}) to string
func (m *Manager) contentToString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// Handle structured content (e.g., multimodal)
		// For now, just return empty string - could be enhanced
		return ""
	default:
		return ""
	}
}
