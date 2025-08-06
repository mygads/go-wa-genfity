package usecase

import (
	"context"
	"fmt"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
)

type userManagementUsecase struct {
	userRepo        domainUserManagement.IUserManagementRepository
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

func NewUserManagementUsecase(userRepo domainUserManagement.IUserManagementRepository, chatStorageRepo domainChatStorage.IChatStorageRepository) domainUserManagement.IUserManagementUsecase {
	return &userManagementUsecase{
		userRepo:        userRepo,
		chatStorageRepo: chatStorageRepo,
	}
}

// getClientFromContext extracts WhatsApp client from app context for user-specific operations
func (u *userManagementUsecase) getClientFromContext(ctx context.Context) (*whatsmeow.Client, error) {
	if appCtx, ok := ctx.(*domainApp.AppContext); ok {
		if appCtx.UserID == 0 {
			return nil, pkgError.ErrNotLoggedIn
		}
		client := whatsapp.GetClientForUser(appCtx.UserID)
		if client == nil {
			return nil, pkgError.ErrNotConnected
		}
		return client, nil
	}
	// Fallback for backwards compatibility (should not happen in production)
	return whatsapp.GetClient(), nil
}

func (u *userManagementUsecase) CreateUser(request domainUserManagement.CreateUserRequest) (*domainUserManagement.UserResponse, error) {
	// Basic validation
	if request.Username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if len(request.Username) < 3 || len(request.Username) > 50 {
		return nil, fmt.Errorf("username must be between 3 and 50 characters")
	}
	if request.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(request.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	// Check if username already exists
	existingUser, err := u.userRepo.GetByUsername(request.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing username: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("username already exists")
	}

	// Create user
	user := &domainUserManagement.User{
		Username: request.Username,
		Password: request.Password,
		IsActive: true,
	}

	if err := u.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Get WhatsApp connection status for new user (will be false initially)
	sessionManager := whatsapp.GetSessionManager()
	isConnected, isLoggedIn, _ := sessionManager.GetUserConnectionStatus(user.ID)

	return &domainUserManagement.UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		IsActive:    user.IsActive,
		IsConnected: isConnected,
		IsLoggedIn:  isLoggedIn,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (u *userManagementUsecase) GetUser(id int) (*domainUserManagement.UserResponse, error) {
	user, err := u.userRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get WhatsApp connection status
	isConnected, isLoggedIn, _ := whatsapp.GetConnectionStatus()

	return &domainUserManagement.UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		IsActive:    user.IsActive,
		IsConnected: isConnected,
		IsLoggedIn:  isLoggedIn,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (u *userManagementUsecase) GetUserByUsername(username string) (*domainUserManagement.UserResponse, error) {
	user, err := u.userRepo.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get WhatsApp connection status for this specific user
	sessionManager := whatsapp.GetSessionManager()
	isConnected, isLoggedIn, _ := sessionManager.GetUserConnectionStatus(user.ID)

	return &domainUserManagement.UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		IsActive:    user.IsActive,
		IsConnected: isConnected,
		IsLoggedIn:  isLoggedIn,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (u *userManagementUsecase) GetAllUsers() ([]domainUserManagement.UserResponse, error) {
	users, err := u.userRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	// Get user-specific connection statuses instead of global status
	sessionManager := whatsapp.GetSessionManager()

	var responses []domainUserManagement.UserResponse
	for _, user := range users {
		// Get connection status for this specific user
		isConnected, isLoggedIn, _ := sessionManager.GetUserConnectionStatus(user.ID)

		responses = append(responses, domainUserManagement.UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			IsActive:    user.IsActive,
			IsConnected: isConnected,
			IsLoggedIn:  isLoggedIn,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		})
	}

	return responses, nil
}

func (u *userManagementUsecase) UpdateUser(id int, request domainUserManagement.UpdateUserRequest) (*domainUserManagement.UserResponse, error) {
	// Basic validation
	if request.Username != "" && (len(request.Username) < 3 || len(request.Username) > 50) {
		return nil, fmt.Errorf("username must be between 3 and 50 characters")
	}
	if request.Password != "" && len(request.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	// Check if user exists
	existingUser, err := u.userRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if username already exists (if updating username)
	if request.Username != "" && request.Username != existingUser.Username {
		userWithSameUsername, err := u.userRepo.GetByUsername(request.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing username: %w", err)
		}
		if userWithSameUsername != nil {
			return nil, fmt.Errorf("username already exists")
		}
	}

	// Update user
	if err := u.userRepo.Update(id, &request); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Get updated user
	updatedUser, err := u.userRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated user: %w", err)
	}

	// Get WhatsApp connection status for this specific user
	sessionManager := whatsapp.GetSessionManager()
	isConnected, isLoggedIn, _ := sessionManager.GetUserConnectionStatus(id)

	return &domainUserManagement.UserResponse{
		ID:          updatedUser.ID,
		Username:    updatedUser.Username,
		IsActive:    updatedUser.IsActive,
		IsConnected: isConnected,
		IsLoggedIn:  isLoggedIn,
		CreatedAt:   updatedUser.CreatedAt,
		UpdatedAt:   updatedUser.UpdatedAt,
	}, nil
}

func (u *userManagementUsecase) DeleteUser(id int) error {
	// Check if user exists
	existingUser, err := u.userRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser == nil {
		return fmt.Errorf("user not found")
	}

	// Delete user
	if err := u.userRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (u *userManagementUsecase) ValidateUserCredentials(username, password string) bool {
	return u.userRepo.ValidateCredentials(username, password)
}

func (u *userManagementUsecase) GetActiveUserCredentials() map[string]string {
	users, err := u.userRepo.GetActiveUsers()
	if err != nil {
		return make(map[string]string)
	}

	credentials := make(map[string]string)
	for _, user := range users {
		// Note: We can't return the actual password since it's hashed
		// This method should be used differently - for basic auth we need to validate each request
		credentials[user.Username] = user.Password // This contains hashed password
	}

	return credentials
}

// DisconnectWhatsAppSession disconnects WhatsApp session for specific user (admin only)
// Similar to user logout - clears all local data but keeps device registered with WhatsApp server
// Device remains active on server, user just needs to login again
func (u *userManagementUsecase) DisconnectWhatsAppSession(userID int) error {
	// Validate user exists
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user with ID %d not found", userID)
	}

	// Get WhatsApp client - for admin operations, we use the global client as fallback
	// since this is administrative function that may not have user context
	client := whatsapp.GetClient()
	if client == nil {
		return fmt.Errorf("WhatsApp client not initialized")
	}

	ctx := context.Background()

	// Perform partial cleanup (like user logout but without server logout)
	// This clears local data but keeps device registered with WhatsApp server
	_, _, err = whatsapp.PerformPartialCleanupAndUpdateGlobals(ctx, fmt.Sprintf("ADMIN_DISCONNECT_USER_%d", userID), u.chatStorageRepo)
	if err != nil {
		return fmt.Errorf("failed to disconnect WhatsApp session for user %s (ID: %d): %v", user.Username, userID, err)
	}

	logrus.Infof("WhatsApp session disconnected for user %s (ID: %d) - device remains registered with server", user.Username, userID)
	return nil
}

// ReconnectWhatsAppSession reconnects WhatsApp session for specific user (admin only)
func (u *userManagementUsecase) ReconnectWhatsAppSession(userID int) error {
	// Validate user exists
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user with ID %d not found", userID)
	}

	// Get WhatsApp client - for admin operations, we use the global client as fallback
	// since this is administrative function that may not have user context
	client := whatsapp.GetClient()
	if client == nil {
		return fmt.Errorf("WhatsApp client not initialized")
	}

	// Reconnect if not connected
	if !client.IsConnected() {
		err := client.Connect()
		if err != nil {
			return fmt.Errorf("failed to reconnect WhatsApp session for user %s (ID: %d): %v", user.Username, userID, err)
		}
		logrus.Infof("WhatsApp session reconnected for user %s (ID: %d)", user.Username, userID)
	} else {
		logrus.Infof("WhatsApp session already connected for user %s (ID: %d)", user.Username, userID)
	}

	return nil
}

// ClearWhatsAppSession completely clears WhatsApp session and forces logout (admin only)
// Like disconnect but also removes device from WhatsApp server - clears RAM completely
func (u *userManagementUsecase) ClearWhatsAppSession(userID int) error {
	// Validate user exists
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user with ID %d not found", userID)
	}

	// Get WhatsApp client
	// Get WhatsApp client - for admin operations, we use the global client as fallback
	// since this is administrative function that may not have user context
	client := whatsapp.GetClient()
	if client == nil {
		return fmt.Errorf("WhatsApp client not initialized")
	}

	ctx := context.Background()

	// Logout from WhatsApp server first to remove device registration
	if client.IsLoggedIn() {
		logrus.Infof("Logging out WhatsApp for user %s (ID: %d) from server (removing device)...", user.Username, userID)
		err := client.Logout(ctx)
		if err != nil {
			logrus.Errorf("Failed to logout WhatsApp for user %s (ID: %d): %v", user.Username, userID, err)
			// Continue with cleanup even if logout fails
		} else {
			logrus.Infof("WhatsApp logout from server completed for user %s (ID: %d) - device removed", user.Username, userID)
		}
	}

	// Perform complete cleanup (clears all data and RAM)
	_, _, err = whatsapp.PerformCleanupAndUpdateGlobals(ctx, fmt.Sprintf("ADMIN_CLEAR_USER_%d", userID), u.chatStorageRepo)
	if err != nil {
		return fmt.Errorf("failed to clear WhatsApp session for user %s (ID: %d): %v", user.Username, userID, err)
	}

	logrus.Infof("WhatsApp session completely cleared for user %s (ID: %d) - device removed from server and RAM cleared", user.Username, userID)
	return nil
}
