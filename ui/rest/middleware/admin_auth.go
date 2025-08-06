package middleware

import (
	"encoding/base64"
	"strings"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

// AdminBasicAuth middleware untuk mengamankan admin endpoints
func AdminBasicAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		logrus.Debugf("AdminBasicAuth middleware called for path: %s", c.Path())
		auth := c.Get("Authorization")
		if auth == "" {
			c.Set("WWW-Authenticate", `Basic realm="Admin Area"`)
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Authorization required",
			})
		}

		if !strings.HasPrefix(auth, "Basic ") {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid authorization format",
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

		username, password := pair[0], pair[1]

		// Check admin credentials from config
		if config.AdminUsername != "" && config.AdminPassword != "" {
			logrus.Debugf("Admin auth - checking credentials: %s vs %s", username, config.AdminUsername)
			if username == config.AdminUsername && password == config.AdminPassword {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
			Status:  fiber.StatusUnauthorized,
			Code:    "UNAUTHORIZED",
			Message: "Invalid admin credentials",
		})
	}
}

// UserBasicAuth middleware yang menggunakan database untuk validasi user
func UserBasicAuth(userUsecase domainUserManagement.IUserManagementUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		logrus.Debugf("UserBasicAuth middleware called for path: %s", c.Path())
		auth := c.Get("Authorization")
		if auth == "" {
			c.Set("WWW-Authenticate", `Basic realm="User Area"`)
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Authorization required",
			})
		}

		if !strings.HasPrefix(auth, "Basic ") {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid authorization format",
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

		username, password := pair[0], pair[1]

		// Validate user credentials from database
		user, err := userUsecase.GetUserByUsername(username)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
				Status:  fiber.StatusUnauthorized,
				Code:    "UNAUTHORIZED",
				Message: "Invalid user credentials",
			})
		}

		if userUsecase.ValidateUserCredentials(username, password) {
			// Store user info in context for template rendering
			c.Locals("user_id", user.ID)
			c.Locals("username", user.Username)
			
			logrus.Debugf("UserBasicAuth: Authenticated user %s (ID: %d)", user.Username, user.ID)
			return c.Next()
		}

		return c.Status(fiber.StatusUnauthorized).JSON(utils.ResponseData{
			Status:  fiber.StatusUnauthorized,
			Code:    "UNAUTHORIZED",
			Message: "Invalid user credentials",
		})
	}
}
