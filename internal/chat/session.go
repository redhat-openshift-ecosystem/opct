package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// ChatMessage represents a single message in a chat session.
type ChatMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// Session represents a chat session with its metadata and messages.
type Session struct {
	ID       string        `json:"id"`
	Created  string        `json:"created"`  // RFC3339
	Updated  string        `json:"updated"`  // RFC3339
	Messages []ChatMessage `json:"messages"`
}

// SessionsDir returns the path to the chat sessions directory for a given report directory.
func SessionsDir(reportDir string) string {
	return filepath.Join(reportDir, "chat-sessions")
}

// EnsureSessionsDir creates the sessions directory if it doesn't exist.
func EnsureSessionsDir(reportDir string) error {
	dir := SessionsDir(reportDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create sessions directory %s: %w", dir, err)
	}
	return nil
}

// NewSession creates a new session with a generated ID and the current time.
func NewSession() *Session {
	now := time.Now().UTC()
	id := fmt.Sprintf("session-%d", now.UnixNano())
	ts := now.Format(time.RFC3339)
	return &Session{
		ID:       id,
		Created:  ts,
		Updated:  ts,
		Messages: []ChatMessage{},
	}
}

// SaveSession saves a session as JSON to the sessions directory, updating the Updated field.
func SaveSession(reportDir string, session *Session) error {
	if err := EnsureSessionsDir(reportDir); err != nil {
		return err
	}

	session.Updated = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session %s: %w", session.ID, err)
	}

	filePath := filepath.Join(SessionsDir(reportDir), session.ID+".json")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write session file %s: %w", filePath, err)
	}

	log.Debugf("Saved chat session %s to %s", session.ID, filePath)
	return nil
}

// LoadSession loads a session by ID from the sessions directory.
func LoadSession(reportDir string, id string) (*Session, error) {
	filePath := filepath.Join(SessionsDir(reportDir), id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file %s: %w", filePath, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session %s: %w", id, err)
	}

	return &session, nil
}

// ListSessions lists all sessions in the sessions directory, sorted by Updated descending.
// Returns only session metadata (ID, Created, Updated) without full messages.
// Returns an empty slice (not nil) if no sessions exist.
func ListSessions(reportDir string) ([]Session, error) {
	sessions := []Session{}

	dir := SessionsDir(reportDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return sessions, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		session, err := LoadSession(reportDir, id)
		if err != nil {
			log.Warnf("Skipping invalid session file %s: %v", entry.Name(), err)
			continue
		}

		// Return only metadata, not full messages.
		sessions = append(sessions, Session{
			ID:      session.ID,
			Created: session.Created,
			Updated: session.Updated,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Updated > sessions[j].Updated
	})

	return sessions, nil
}
