package whatsapp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

// UserSession represents a WhatsApp session for a specific user
type UserSession struct {
	UserID          int
	Username        string
	Client          *whatsmeow.Client
	DB              *sqlstore.Container
	KeysDB          *sqlstore.Container
	ChatStorageRepo domainChatStorage.IChatStorageRepository
}

// SessionManager manages WhatsApp sessions for multiple users
type SessionManager struct {
	sessions map[int]*UserSession // userID -> UserSession
	mutex    sync.RWMutex
}

var (
	sessionManager *SessionManager
	once           sync.Once
)

// GetSessionManager returns the singleton session manager
func GetSessionManager() *SessionManager {
	once.Do(func() {
		sessionManager = &SessionManager{
			sessions: make(map[int]*UserSession),
		}
	})
	return sessionManager
}

// GetUserSession returns the WhatsApp session for a specific user
func (sm *SessionManager) GetUserSession(userID int) *UserSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.sessions[userID]
}

// CreateUserSession creates a new WhatsApp session for a user
func (sm *SessionManager) CreateUserSession(ctx context.Context, userID int, username string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*UserSession, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Check if session already exists
	if session, exists := sm.sessions[userID]; exists {
		logrus.Infof("Session already exists for user %s (ID: %d)", username, userID)
		return session, nil
	}

	// Create user-specific database URIs
	userDBURI := fmt.Sprintf("file:%s/user_%d_whatsapp.db?_foreign_keys=on",
		filepath.Dir(config.DBURI[5:]), userID) // Remove "file:" prefix

	var userKeysDBURI string
	if config.DBKeysURI != "" {
		if config.DBKeysURI == ":memory:" || config.DBKeysURI == "file::memory:?cache=shared&_foreign_keys=on" {
			// Use per-user persistent keys database instead of memory to avoid FK constraints issues
			userKeysDBURI = fmt.Sprintf("file:storages/user_%d_keys.db?_foreign_keys=on", userID)
		} else {
			userKeysDBURI = fmt.Sprintf("file:%s/user_%d_keys.db?_foreign_keys=on",
				filepath.Dir(config.DBKeysURI[5:]), userID)
		}
	}

	logrus.Infof("Creating WhatsApp session for user %s (ID: %d)", username, userID)
	logrus.Infof("User DB URI: %s", userDBURI)
	logrus.Infof("User Keys DB URI: %s", userKeysDBURI)

	// Initialize user-specific databases
	userDB := InitWaDB(ctx, userDBURI)
	var userKeysDB *sqlstore.Container
	if userKeysDBURI != "" {
		userKeysDB = InitWaDB(ctx, userKeysDBURI)
	}

	// Initialize user-specific client
	userClient := InitWaCLI(ctx, userDB, userKeysDB, chatStorageRepo)

	// Create user session
	session := &UserSession{
		UserID:          userID,
		Username:        username,
		Client:          userClient,
		DB:              userDB,
		KeysDB:          userKeysDB,
		ChatStorageRepo: chatStorageRepo,
	}

	// Store session
	sm.sessions[userID] = session

	logrus.Infof("Successfully created WhatsApp session for user %s (ID: %d)", username, userID)
	return session, nil
}

// RemoveUserSession removes a user's WhatsApp session
func (sm *SessionManager) RemoveUserSession(userID int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[userID]
	if !exists {
		return fmt.Errorf("session not found for user ID: %d", userID)
	}

	// Disconnect client if connected
	if session.Client != nil {
		session.Client.Disconnect()
	}

	// Close databases
	if session.DB != nil {
		session.DB.Close()
	}
	if session.KeysDB != nil {
		session.KeysDB.Close()
	}

	// Remove from sessions map
	delete(sm.sessions, userID)

	logrus.Infof("Successfully removed WhatsApp session for user %s (ID: %d)", session.Username, userID)
	return nil
}

// GetUserClient returns the WhatsApp client for a specific user
func (sm *SessionManager) GetUserClient(userID int) *whatsmeow.Client {
	session := sm.GetUserSession(userID)
	if session == nil {
		return nil
	}
	return session.Client
}

// GetUserDB returns the WhatsApp database for a specific user
func (sm *SessionManager) GetUserDB(userID int) *sqlstore.Container {
	session := sm.GetUserSession(userID)
	if session == nil {
		return nil
	}
	return session.DB
}

// GetUserConnectionStatus returns the connection status for a specific user
func (sm *SessionManager) GetUserConnectionStatus(userID int) (isConnected bool, isLoggedIn bool, deviceID string) {
	session := sm.GetUserSession(userID)
	if session == nil || session.Client == nil {
		return false, false, ""
	}

	isConnected = session.Client.IsConnected()
	isLoggedIn = session.Client.IsLoggedIn()

	if session.Client.Store != nil && session.Client.Store.ID != nil {
		deviceID = session.Client.Store.ID.String()
	}

	return isConnected, isLoggedIn, deviceID
}

// ListActiveSessions returns a list of all active user sessions
func (sm *SessionManager) ListActiveSessions() map[int]*UserSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// Create a copy to avoid race conditions
	sessions := make(map[int]*UserSession)
	for userID, session := range sm.sessions {
		sessions[userID] = session
	}
	return sessions
}

// GetOrCreateUserSession gets an existing session or creates a new one
func (sm *SessionManager) GetOrCreateUserSession(ctx context.Context, userID int, username string, chatStorageRepo domainChatStorage.IChatStorageRepository) (*UserSession, error) {
	// Try to get existing session first
	if session := sm.GetUserSession(userID); session != nil {
		return session, nil
	}

	// Create new session if it doesn't exist
	return sm.CreateUserSession(ctx, userID, username, chatStorageRepo)
}

// ManualReconnectUser manually reconnects a specific user (for login attempts, QR scan, etc.)
func (sm *SessionManager) ManualReconnectUser(userID int) error {
	session := sm.GetUserSession(userID)
	if session == nil {
		return fmt.Errorf("user session not found for userID: %d", userID)
	}

	if session.Client == nil {
		return fmt.Errorf("client not initialized for userID: %d", userID)
	}

	logrus.Infof("[MANUAL-RECONNECT] Connecting user %d (%s) to WhatsApp server...", userID, session.Username)

	if err := session.Client.Connect(); err != nil {
		logrus.Errorf("[MANUAL-RECONNECT] Failed to connect user %d (%s): %v", userID, session.Username, err)
		return fmt.Errorf("failed to connect user %d: %v", userID, err)
	}

	logrus.Infof("[MANUAL-RECONNECT] User %d (%s) connected successfully", userID, session.Username)
	return nil
}

// DisconnectUser manually disconnects a specific user (to save resources when not logged in)
func (sm *SessionManager) DisconnectUser(userID int) error {
	session := sm.GetUserSession(userID)
	if session == nil {
		return fmt.Errorf("user session not found for userID: %d", userID)
	}

	if session.Client == nil {
		return fmt.Errorf("client not initialized for userID: %d", userID)
	}

	logrus.Infof("[MANUAL-DISCONNECT] Disconnecting user %d (%s) from WhatsApp server...", userID, session.Username)
	session.Client.Disconnect()
	logrus.Infof("[MANUAL-DISCONNECT] User %d (%s) disconnected successfully", userID, session.Username)
	return nil
}

// ReconnectAllLoggedInUsers reconnects only users who are already logged in to WhatsApp
func (sm *SessionManager) ReconnectAllLoggedInUsers() {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	loggedInCount := 0
	connectedCount := 0

	for userID, session := range sm.sessions {
		if session.Client != nil {
			if session.Client.IsLoggedIn() {
				loggedInCount++
				if !session.Client.IsConnected() {
					logrus.Infof("[RECONNECT-LOGGED-IN] Reconnecting logged-in user %d (%s)...", userID, session.Username)
					if err := session.Client.Connect(); err != nil {
						logrus.Errorf("[RECONNECT-LOGGED-IN] Failed to reconnect user %d (%s): %v", userID, session.Username, err)
					} else {
						connectedCount++
						logrus.Infof("[RECONNECT-LOGGED-IN] User %d (%s) reconnected successfully", userID, session.Username)
					}
				} else {
					connectedCount++
				}
			} else {
				logrus.Debugf("[RECONNECT-LOGGED-IN] User %d (%s) not logged in, keeping disconnected", userID, session.Username)
			}
		}
	}

	logrus.Infof("[RECONNECT-LOGGED-IN] Summary: %d logged-in users, %d successfully connected", loggedInCount, connectedCount)
}

// HasStoredSession checks if a user has stored WhatsApp session data (previously logged in)
func (sm *SessionManager) HasStoredSession(userID int) bool {
	session := sm.GetUserSession(userID)
	if session == nil || session.Client == nil {
		return false
	}

	// Check if there's stored session data in the database
	if session.Client.Store != nil && session.Client.Store.ID != nil {
		return true
	}

	return false
}
