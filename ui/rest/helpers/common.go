package helpers

import (
	"context"
	"mime/multipart"
	"time"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

func SetAutoConnectAfterBootingWithUserManagement(service domainApp.IAppUsecase, userManagementUsecase domainUserManagement.IUserManagementUsecase, chatStorageRepo domainChatStorage.IChatStorageRepository) {
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	sessionManager := whatsapp.GetSessionManager()

	// Get all registered users from database
	users, err := userManagementUsecase.GetAllUsers()
	if err != nil {
		logrus.Errorf("Failed to get users from database: %v", err)
		logrus.Info("Falling back to checking active sessions only...")
		SetAutoConnectAfterBooting(service)
		return
	}

	// Display user count and status summary
	logrus.Infof("=== WhatsApp Connection Status Summary ===")
	logrus.Infof("Total registered users in database: %d", len(users))

	if len(users) == 0 {
		logrus.Info("No users found in database. Reconnecting global client...")
		_ = service.Reconnect(ctx)
		return
	}

	// Check and create sessions for all registered users
	activeSessionCount := 0
	connectedCount := 0
	loggedInCount := 0

	for _, user := range users {
		if !user.IsActive {
			logrus.Infof("User %d (%s): INACTIVE - skipping", user.ID, user.Username)
			continue
		}

		// Get or create user session
		_, err := sessionManager.GetOrCreateUserSession(ctx, user.ID, user.Username, chatStorageRepo)
		if err != nil {
			logrus.Errorf("Failed to create session for user %d (%s): %v", user.ID, user.Username, err)
			continue
		}

		activeSessionCount++

		// Get connection status
		isConnected, isLoggedIn, deviceID := sessionManager.GetUserConnectionStatus(user.ID)

		status := "DISCONNECTED"
		if isConnected && isLoggedIn {
			status = "CONNECTED & LOGGED IN"
			connectedCount++
			loggedInCount++
		} else if isConnected {
			status = "CONNECTED (NOT LOGGED IN)"
			connectedCount++
		} else if isLoggedIn {
			status = "LOGGED IN (NOT CONNECTED)"
			loggedInCount++
		}

		deviceInfo := ""
		if deviceID != "" {
			deviceInfo = " | Device: " + deviceID[:10] + "..."
		}

		logrus.Infof("User %d (%s): %s%s", user.ID, user.Username, status, deviceInfo)
	}

	logrus.Infof("Summary: %d active sessions, %d connected, %d logged in", activeSessionCount, connectedCount, loggedInCount)
	logrus.Info("===========================================")

	if activeSessionCount == 0 {
		logrus.Info("No active sessions to reconnect. Reconnecting global client...")
		_ = service.Reconnect(ctx)
		return
	}

	// Reconnect all active users
	logrus.Info("Starting reconnection process for all active user sessions...")

	activeSessions := sessionManager.ListActiveSessions()
	for userID, session := range activeSessions {
		logrus.Infof("[USER %d] Starting reconnect process for user %s...", userID, session.Username)

		if session.Client == nil {
			logrus.Errorf("[USER %d] Client is nil for user %s, skipping reconnect", userID, session.Username)
			continue
		}

		// Disconnect first
		session.Client.Disconnect()

		// Reconnect
		err := session.Client.Connect()
		if err != nil {
			logrus.Errorf("[USER %d] Reconnect failed for user %s: %v", userID, session.Username, err)
		} else {
			isConnected := session.Client.IsConnected()
			isLoggedIn := session.Client.IsLoggedIn()

			status := "DISCONNECTED"
			if isConnected && isLoggedIn {
				status = "CONNECTED & LOGGED IN"
			} else if isConnected {
				status = "CONNECTED (NOT LOGGED IN)"
			} else if isLoggedIn {
				status = "LOGGED IN (NOT CONNECTED)"
			}

			logrus.Infof("[USER %d] Reconnection completed for user %s - Status: %s",
				userID, session.Username, status)
		}
	}

	// No need for global client in multi-user system
	logrus.Info("=== Reconnection process completed for all user sessions ===")
}

func SetAutoConnectAfterBooting(service domainApp.IAppUsecase) {
	time.Sleep(2 * time.Second)

	// Get session manager to check all users
	sessionManager := whatsapp.GetSessionManager()

	// Load all registered users from database and create sessions if they don't exist
	// Note: We need a way to get user management usecase here to load users
	// For now, let's work with existing active sessions and show a more informative message

	activeSessions := sessionManager.ListActiveSessions()

	// Display user count and status summary
	logrus.Infof("=== WhatsApp Connection Status Summary ===")
	logrus.Infof("Total active user sessions: %d", len(activeSessions))
	logrus.Info("Note: Only users who have logged in before will have active sessions")

	if len(activeSessions) == 0 {
		logrus.Info("No active user sessions found.")
		logrus.Info("Users need to login first to create WhatsApp sessions.")
		logrus.Info("Reconnecting global client for backwards compatibility...")
		_ = service.Reconnect(context.Background())
		return
	}

	// Display status for each user
	connectedCount := 0
	loggedInCount := 0

	for userID, session := range activeSessions {
		isConnected, isLoggedIn, deviceID := sessionManager.GetUserConnectionStatus(userID)

		status := "DISCONNECTED"
		if isConnected && isLoggedIn {
			status = "CONNECTED & LOGGED IN"
			connectedCount++
			loggedInCount++
		} else if isConnected {
			status = "CONNECTED (NOT LOGGED IN)"
			connectedCount++
		} else if isLoggedIn {
			status = "LOGGED IN (NOT CONNECTED)"
			loggedInCount++
		}

		deviceInfo := ""
		if deviceID != "" {
			deviceInfo = " | Device: " + deviceID[:10] + "..."
		}

		logrus.Infof("User %d (%s): %s%s", userID, session.Username, status, deviceInfo)
	}

	logrus.Infof("Summary: %d connected, %d logged in out of %d active sessions", connectedCount, loggedInCount, len(activeSessions))
	logrus.Info("===========================================")

	// Reconnect all users with active sessions
	logrus.Info("Starting reconnection process for all active user sessions...")

	for userID, session := range activeSessions {
		logrus.Infof("[USER %d] Starting reconnect process for user %s...", userID, session.Username)

		if session.Client == nil {
			logrus.Errorf("[USER %d] Client is nil for user %s, skipping reconnect", userID, session.Username)
			continue
		}

		// Disconnect first
		session.Client.Disconnect()

		// Reconnect
		err := session.Client.Connect()
		if err != nil {
			logrus.Errorf("[USER %d] Reconnect failed for user %s: %v", userID, session.Username, err)
		} else {
			isConnected := session.Client.IsConnected()
			isLoggedIn := session.Client.IsLoggedIn()

			status := "DISCONNECTED"
			if isConnected && isLoggedIn {
				status = "CONNECTED & LOGGED IN"
			} else if isConnected {
				status = "CONNECTED (NOT LOGGED IN)"
			} else if isLoggedIn {
				status = "LOGGED IN (NOT CONNECTED)"
			}

			logrus.Infof("[USER %d] Reconnection completed for user %s - Status: %s",
				userID, session.Username, status)
		}
	}

	logrus.Info("=== Reconnection process completed for all active sessions ===")
}

func SetAutoReconnectCheckingForAllUsers() {
	// Run every 5 minutes to check if all user connections are still alive, if not, reconnect
	go func() {
		for {
			time.Sleep(5 * time.Minute)

			sessionManager := whatsapp.GetSessionManager()
			activeSessions := sessionManager.ListActiveSessions()

			for userID, session := range activeSessions {
				if session.Client != nil && !session.Client.IsConnected() {
					logrus.Infof("[AUTO-RECONNECT] User %d (%s) disconnected, attempting reconnection...", userID, session.Username)
					if err := session.Client.Connect(); err != nil {
						logrus.Errorf("[AUTO-RECONNECT] Failed to reconnect user %d (%s): %v", userID, session.Username, err)
					} else {
						logrus.Infof("[AUTO-RECONNECT] User %d (%s) reconnected successfully", userID, session.Username)
					}
				}
			}
		}
	}()
}

func SetAutoReconnectChecking(cli *whatsmeow.Client) {
	// Run every 5 minutes to check if the connection is still alive, if not, reconnect
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			if !cli.IsConnected() {
				_ = cli.Connect()
			}
		}
	}()
}

func MultipartFormFileHeaderToBytes(fileHeader *multipart.FileHeader) []byte {
	file, _ := fileHeader.Open()
	defer file.Close()

	fileBytes := make([]byte, fileHeader.Size)
	_, _ = file.Read(fileBytes)

	return fileBytes
}
