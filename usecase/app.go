package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/libsignal/logger"
	"go.mau.fi/whatsmeow"
)

type serviceApp struct {
	chatStorageRepo domainChatStorage.IChatStorageRepository
}

func NewAppService(chatStorageRepo domainChatStorage.IChatStorageRepository) domainApp.IAppUsecaseWithContext {
	return &serviceApp{
		chatStorageRepo: chatStorageRepo,
	}
}

// getClientFromContext extracts WhatsApp client from app context for user-specific operations
func (service serviceApp) getClientFromContext(ctx context.Context) (*whatsmeow.Client, error) {
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
	// In multi-user system, all operations must have user context
	return nil, pkgError.ErrNotLoggedIn
}

// Context-aware methods for multi-user support

func (service *serviceApp) LoginWithContext(appCtx *domainApp.AppContext) (response domainApp.LoginResponse, err error) {
	if appCtx.UserID == 0 {
		return response, fmt.Errorf("user ID required for login in multi-user system")
	}

	// Get user-specific client
	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	// [DEBUG] Log database state before login
	logrus.Infof("[DEBUG] Starting login process for user %d...", appCtx.UserID)
	sessionManager := whatsapp.GetSessionManager()
	userDB := sessionManager.GetUserDB(appCtx.UserID)
	if userDB != nil {
		devices, dbErr := userDB.GetAllDevices(appCtx.Context)
		if dbErr != nil {
			logrus.Errorf("[DEBUG] Error getting devices before login: %v", dbErr)
		} else {
			logrus.Infof("[DEBUG] Devices before login: %d found", len(devices))
			for _, device := range devices {
				logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
			}
		}
	}

	// Rest of login logic similar to original Login method
	client.Disconnect()
	chImage := make(chan string)

	logrus.Infof("[DEBUG] Attempting to get QR channel for user %d...", appCtx.UserID)
	ch, err := client.GetQRChannel(appCtx.Context)
	if err != nil {
		logrus.Errorf("[DEBUG] GetQRChannel failed: %v", err)
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			logrus.Info("[DEBUG] Error is ErrQRStoreContainsID - attempting to connect")
			_ = client.Connect()
			if client.IsLoggedIn() {
				return response, pkgError.ErrAlreadyLoggedIn
			}
			return response, pkgError.ErrSessionSaved
		} else {
			return response, pkgError.ErrQrChannel
		}
	} else {
		logrus.Infof("[DEBUG] QR channel obtained successfully for user %d", appCtx.UserID)
		go func() {
			for evt := range ch {
				response.Code = evt.Code
				response.Duration = evt.Timeout / time.Second / 2
				if evt.Event == "code" {
					qrPath := fmt.Sprintf("%s/scan-qr-user-%d-%s.png", config.PathQrCode, appCtx.UserID, fiberUtils.UUIDv4())
					err = qrcode.WriteFile(evt.Code, qrcode.Medium, 512, qrPath)
					if err != nil {
						logrus.Error("Error when write qr code to file: ", err)
					}
					go func() {
						time.Sleep(response.Duration * time.Second)
						err := os.Remove(qrPath)
						if err != nil {
							if !os.IsNotExist(err) {
								logrus.Error("error when remove qrImage file", err.Error())
							}
						}
					}()
					chImage <- qrPath
				} else if evt.Event == "success" {
					logrus.Infof("[DEBUG] QR login successful for user %d", appCtx.UserID)
					// QR login was successful, break out of loop
					break
				} else {
					logrus.Errorf("QR code error for user %d: event=%s, error=%v", appCtx.UserID, evt.Event, evt.Error)
				}
			}
		}()
	}

	err = client.Connect()
	if err != nil {
		logger.Error("Error when connect to whatsapp", err)
		return response, pkgError.ErrReconnect
	}
	response.ImagePath = <-chImage

	logrus.Infof("[DEBUG] Login connection established for user %d - IsConnected: %v, IsLoggedIn: %v",
		appCtx.UserID, client.IsConnected(), client.IsLoggedIn())

	return response, nil
}

func (service *serviceApp) LoginWithCodeAndContext(appCtx *domainApp.AppContext, phoneNumber string) (loginCode string, err error) {
	if appCtx.UserID == 0 {
		return loginCode, fmt.Errorf("user ID required for login with code in multi-user system")
	}

	if err = validations.ValidateLoginWithCode(appCtx.Context, phoneNumber); err != nil {
		logrus.Errorf("Error when validate login with code: %s", err.Error())
		return loginCode, err
	}

	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return loginCode, pkgError.ErrWaCLI
	}

	if client.Store.ID != nil {
		logrus.Warnf("User %d is already logged in", appCtx.UserID)
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	_ = service.ReconnectWithContext(appCtx)

	logrus.Infof("[DEBUG] Starting phone pairing for user %d, number: %s", appCtx.UserID, phoneNumber)
	loginCode, err = client.PairPhone(appCtx.Context, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logrus.Errorf("Error when pairing phone: %s", err.Error())
		return loginCode, err
	}

	logrus.Infof("[DEBUG] Phone pairing completed for user %d - IsConnected: %v, IsLoggedIn: %v",
		appCtx.UserID, client.IsConnected(), client.IsLoggedIn())

	logrus.Infof("Successfully paired phone for user %d with code: %s", appCtx.UserID, loginCode)
	return loginCode, nil
}

func (service *serviceApp) LogoutWithContext(appCtx *domainApp.AppContext) (err error) {
	if appCtx.UserID == 0 {
		return fmt.Errorf("user ID required for logout in multi-user system")
	}

	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return pkgError.ErrWaCLI
	}

	logrus.Infof("[DEBUG] Starting logout process for user %d...", appCtx.UserID)

	err = client.Logout(appCtx.Context)
	if err != nil {
		logrus.Errorf("[DEBUG] WhatsApp logout failed for user %d: %v", appCtx.UserID, err)
	} else {
		logrus.Infof("[DEBUG] WhatsApp logout completed successfully for user %d", appCtx.UserID)
	}

	// Remove user session
	sessionManager := whatsapp.GetSessionManager()
	if removeErr := sessionManager.RemoveUserSession(appCtx.UserID); removeErr != nil {
		logrus.Errorf("[DEBUG] Failed to remove user session for user %d: %v", appCtx.UserID, removeErr)
		return removeErr
	}

	logrus.Infof("[DEBUG] Logout process completed successfully for user %d", appCtx.UserID)
	return nil
}

func (service *serviceApp) ReconnectWithContext(appCtx *domainApp.AppContext) (err error) {
	if appCtx.UserID == 0 {
		return fmt.Errorf("user ID required for reconnect in multi-user system")
	}

	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return pkgError.ErrWaCLI
	}

	logrus.Infof("[DEBUG] Starting reconnect process for user %d...", appCtx.UserID)

	client.Disconnect()
	err = client.Connect()

	if err != nil {
		logrus.Errorf("[DEBUG] Reconnect failed for user %d: %v", appCtx.UserID, err)
		return err
	}

	logrus.Infof("[DEBUG] Reconnection completed for user %d - IsConnected: %v, IsLoggedIn: %v",
		appCtx.UserID, client.IsConnected(), client.IsLoggedIn())

	logrus.Infof("[DEBUG] Reconnect process completed successfully for user %d", appCtx.UserID)
	return err
}

func (service *serviceApp) FirstDeviceWithContext(appCtx *domainApp.AppContext) (response domainApp.DevicesResponse, err error) {
	if appCtx.UserID == 0 {
		return response, fmt.Errorf("user ID required for device access in multi-user system")
	}

	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	sessionManager := whatsapp.GetSessionManager()
	userDB := sessionManager.GetUserDB(appCtx.UserID)
	if userDB == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := userDB.GetFirstDevice(appCtx.Context)
	if err != nil {
		return response, err
	}

	response.Device = devices.ID.String()
	if devices.PushName != "" {
		response.Name = devices.PushName
	} else {
		response.Name = devices.BusinessName
	}

	return response, nil
}

func (service *serviceApp) FetchDevicesWithContext(appCtx *domainApp.AppContext) (response []domainApp.DevicesResponse, err error) {
	if appCtx.UserID == 0 {
		return response, fmt.Errorf("user ID required for device access in multi-user system")
	}

	client := whatsapp.GetClientForUser(appCtx.UserID)
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	sessionManager := whatsapp.GetSessionManager()
	userDB := sessionManager.GetUserDB(appCtx.UserID)
	if userDB == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := userDB.GetAllDevices(appCtx.Context)
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		var d domainApp.DevicesResponse
		d.Device = device.ID.String()
		if device.PushName != "" {
			d.Name = device.PushName
		} else {
			d.Name = device.BusinessName
		}

		response = append(response, d)
	}

	return response, nil
}
