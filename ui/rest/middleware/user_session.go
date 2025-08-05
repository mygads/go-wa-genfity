package middleware

import (
	"context"
	"encoding/base64"
	"strings"

	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

const (
	UserIDKey      = "user_id"
	UserSessionKey = "user_session"
	UsernameKey    = "username"
)

// UserSessionMiddleware extracts user information and ensures user has a WhatsApp session
func UserSessionMiddleware(userUsecase domainUserManagement.IUserManagementUsecase, chatStorageRepo domainChatStorage.IChatStorageRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract user credentials from Basic Auth
		auth := c.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Authorization required",
			})
		}

		payload, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid authorization encoding",
			})
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid authorization format",
			})
		}

		username := pair[0]

		// Get user from database
		user, err := userUsecase.GetUserByUsername(username)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid user credentials",
			})
		}

		// Set user information in context
		c.Locals(UserIDKey, user.ID)
		c.Locals(UsernameKey, user.Username)

		// Get or create user's WhatsApp session
		sessionManager := whatsapp.GetSessionManager()
		session, err := sessionManager.GetOrCreateUserSession(
			context.Background(),
			user.ID,
			user.Username,
			chatStorageRepo,
		)
		if err != nil {
			logrus.Errorf("Failed to create user session for %s: %v", user.Username, err)
			return c.Status(fiber.StatusInternalServerError).JSON(utils.ResponseData{
				Status:  fiber.StatusInternalServerError,
				Code:    "SESSION_ERROR",
				Message: "Failed to initialize user session",
			})
		}

		// Set session in context
		c.Locals(UserSessionKey, session)

		logrus.Debugf("User session middleware: user %s (ID: %d) authenticated", user.Username, user.ID)

		return c.Next()
	}
}

// GetUserIDFromContext extracts user ID from fiber context
func GetUserIDFromContext(c *fiber.Ctx) (int, bool) {
	userID, ok := c.Locals(UserIDKey).(int)
	return userID, ok
}

// GetUsernameFromContext extracts username from fiber context
func GetUsernameFromContext(c *fiber.Ctx) (string, bool) {
	username, ok := c.Locals(UsernameKey).(string)
	return username, ok
}

// GetUserSessionFromContext extracts user WhatsApp session from fiber context
func GetUserSessionFromContext(c *fiber.Ctx) (*whatsapp.UserSession, bool) {
	session, ok := c.Locals(UserSessionKey).(*whatsapp.UserSession)
	return session, ok
}
