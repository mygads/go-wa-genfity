package rest

import (
	"fmt"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/gofiber/fiber/v2"
)

type App struct {
	Service domainApp.IAppUsecaseWithContext
}

func InitRestApp(app fiber.Router, service domainApp.IAppUsecaseWithContext) App {
	rest := App{Service: service}
	app.Get("/app/login", rest.Login)
	app.Get("/app/login-with-code", rest.LoginWithCode)
	app.Get("/app/logout", rest.Logout)
	app.Get("/app/reconnect", rest.Reconnect)
	app.Get("/app/devices", rest.Devices)
	app.Get("/app/status", rest.ConnectionStatus)

	return App{Service: service}
}

func (handler *App) Login(c *fiber.Ctx) error {
	// Create app context with user information
	appCtx := domainApp.NewAppContext(c.UserContext(), c)

	response, err := handler.Service.LoginWithContext(appCtx)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login success",
		Results: map[string]any{
			"qr_link":     fmt.Sprintf("%s://%s%s/%s", c.Protocol(), c.Hostname(), config.AppBasePath, response.ImagePath),
			"qr_duration": response.Duration,
		},
	})
}

func (handler *App) LoginWithCode(c *fiber.Ctx) error {
	// Create app context with user information
	appCtx := domainApp.NewAppContext(c.UserContext(), c)

	pairCode, err := handler.Service.LoginWithCodeAndContext(appCtx, c.Query("phone"))
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Login with code success",
		Results: map[string]any{
			"pair_code": pairCode,
		},
	})
}

func (handler *App) Logout(c *fiber.Ctx) error {
	// Create app context with user information
	appCtx := domainApp.NewAppContext(c.UserContext(), c)

	err := handler.Service.LogoutWithContext(appCtx)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Success logout",
		Results: nil,
	})
}

func (handler *App) Reconnect(c *fiber.Ctx) error {
	// Create app context with user information
	appCtx := domainApp.NewAppContext(c.UserContext(), c)

	err := handler.Service.ReconnectWithContext(appCtx)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Reconnect success",
		Results: nil,
	})
}

func (handler *App) Devices(c *fiber.Ctx) error {
	// Create app context with user information
	appCtx := domainApp.NewAppContext(c.UserContext(), c)

	devices, err := handler.Service.FetchDevicesWithContext(appCtx)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Fetch device success",
		Results: devices,
	})
}

func (handler *App) ConnectionStatus(c *fiber.Ctx) error {
	// Get user-specific connection status
	userID, hasUserID := middleware.GetUserIDFromContext(c)

	var isConnected, isLoggedIn bool
	var deviceID string

	if hasUserID {
		// Get status for specific user
		sessionManager := whatsapp.GetSessionManager()
		isConnected, isLoggedIn, deviceID = sessionManager.GetUserConnectionStatus(userID)
	} else {
		// Fallback to global status
		isConnected, isLoggedIn, deviceID = whatsapp.GetConnectionStatus()
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Connection status retrieved",
		Results: map[string]any{
			"is_connected": isConnected,
			"is_logged_in": isLoggedIn,
			"device_id":    deviceID,
		},
	})
}
