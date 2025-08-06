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
	// Fallback for backwards compatibility (should not happen in production)
	return whatsapp.GetClient(), nil
}

func (service *serviceApp) Login(ctx context.Context) (response domainApp.LoginResponse, err error) {
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return response, err
	}
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	// [DEBUG] Log database state before login
	logrus.Info("[DEBUG] Starting login process...")
	devices, dbErr := whatsapp.GetDB().GetAllDevices(context.Background())
	if dbErr != nil {
		logrus.Errorf("[DEBUG] Error getting devices before login: %v", dbErr)
	} else {
		logrus.Infof("[DEBUG] Devices before login: %d found", len(devices))
		for _, device := range devices {
			logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
		}
	}

	// [DEBUG] Log client state
	if client.Store.ID != nil {
		logrus.Infof("[DEBUG] Client has existing store ID: %s", client.Store.ID.String())
	} else {
		logrus.Info("[DEBUG] Client has no store ID")
	}

	// Disconnect for reconnecting
	client.Disconnect()

	chImage := make(chan string)

	logrus.Info("[DEBUG] Attempting to get QR channel...")
	ch, err := client.GetQRChannel(context.Background())
	if err != nil {
		logrus.Errorf("[DEBUG] GetQRChannel failed: %v", err)
		logrus.Error(err.Error())
		// This error means that we're already logged in, so ignore it.
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			logrus.Info("[DEBUG] Error is ErrQRStoreContainsID - attempting to connect")
			_ = client.Connect() // just connect to websocket
			if client.IsLoggedIn() {
				return response, pkgError.ErrAlreadyLoggedIn
			}
			return response, pkgError.ErrSessionSaved
		} else {
			return response, pkgError.ErrQrChannel
		}
	} else {
		logrus.Info("[DEBUG] QR channel obtained successfully")
		go func() {
			for evt := range ch {
				response.Code = evt.Code
				response.Duration = evt.Timeout / time.Second / 2
				if evt.Event == "code" {
					qrPath := fmt.Sprintf("%s/scan-qr-%s.png", config.PathQrCode, fiberUtils.UUIDv4())
					err = qrcode.WriteFile(evt.Code, qrcode.Medium, 512, qrPath)
					if err != nil {
						logrus.Error("Error when write qr code to file: ", err)
					}
					go func() {
						time.Sleep(response.Duration * time.Second)
						err := os.Remove(qrPath)
						if err != nil {
							// Only log if it's not a "file not found" error
							if !os.IsNotExist(err) {
								logrus.Error("error when remove qrImage file", err.Error())
							}
						}
					}()
					chImage <- qrPath
				} else {
					logrus.Error("error when get qrCode", evt.Event, evt.Error)
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

	// [DEBUG] Verify connection state and sync global client
	logrus.Infof("[DEBUG] Login connection established - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	return response, nil
}

func (service *serviceApp) LoginWithCode(ctx context.Context, phoneNumber string) (loginCode string, err error) {
	if err = validations.ValidateLoginWithCode(ctx, phoneNumber); err != nil {
		logrus.Errorf("Error when validate login with code: %s", err.Error())
		return loginCode, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return loginCode, err
	}
	// detect is already logged in
	if client.Store.ID != nil {
		logrus.Warn("User is already logged in")
		return loginCode, pkgError.ErrAlreadyLoggedIn
	}

	// reconnect first
	_ = service.Reconnect(ctx)

	logrus.Infof("[DEBUG] Starting phone pairing for number: %s", phoneNumber)
	loginCode, err = client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logrus.Errorf("Error when pairing phone: %s", err.Error())
		return loginCode, err
	}

	// [DEBUG] Verify pairing state and sync global client
	logrus.Infof("[DEBUG] Phone pairing completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	logrus.Infof("Successfully paired phone with code: %s", loginCode)
	return loginCode, nil
}

func (service *serviceApp) Logout(ctx context.Context) (err error) {
	// [DEBUG] Log database state before logout
	logrus.Info("[DEBUG] Starting logout process...")
	devices, dbErr := whatsapp.GetDB().GetAllDevices(ctx)
	if dbErr != nil {
		logrus.Errorf("[DEBUG] Error getting devices before logout: %v", dbErr)
	} else {
		logrus.Infof("[DEBUG] Devices before logout: %d found", len(devices))
		for _, device := range devices {
			logrus.Infof("[DEBUG] Device ID: %s, PushName: %s", device.ID.String(), device.PushName)
		}
	}

	// [DEBUG] Call WhatsApp client logout first to disconnect from server
	logrus.Info("[DEBUG] Calling WhatsApp client logout...")
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}
	err = client.Logout(ctx)
	if err != nil {
		logrus.Errorf("[DEBUG] WhatsApp logout failed: %v", err)
		// Continue with cleanup even if logout fails
	} else {
		logrus.Info("[DEBUG] WhatsApp logout completed successfully")
	}

	// [DEBUG] Verify devices after logout
	devices, dbErr = whatsapp.GetDB().GetAllDevices(ctx)
	if dbErr != nil {
		logrus.Errorf("[DEBUG] Error getting devices after logout: %v", dbErr)
	} else {
		logrus.Infof("[DEBUG] Devices after logout: %d found", len(devices))
	}

	// Perform complete cleanup with global client synchronization
	newDB, newCli, err := whatsapp.PerformCleanupAndUpdateGlobals(ctx, "MANUAL_LOGOUT", service.chatStorageRepo)
	if err != nil {
		logrus.Errorf("[DEBUG] Cleanup failed: %v", err)
		return err
	}

	// Update service references
	whatsapp.UpdateGlobalClient(newCli, newDB)

	logrus.Info("[DEBUG] Logout process completed successfully")
	return nil
}

func (service *serviceApp) Reconnect(ctx context.Context) (err error) {
	logrus.Info("[DEBUG] Starting reconnect process...")

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}
	client.Disconnect()
	err = client.Connect()

	if err != nil {
		logrus.Errorf("[DEBUG] Reconnect failed: %v", err)
		return err
	}

	// [DEBUG] Verify reconnection state and sync global client
	logrus.Infof("[DEBUG] Reconnection completed - IsConnected: %v, IsLoggedIn: %v",
		client.IsConnected(), client.IsLoggedIn())

	// Ensure global client is synchronized with service client
	whatsapp.UpdateGlobalClient(client, whatsapp.GetDB())

	logrus.Info("[DEBUG] Reconnect process completed successfully")
	return err
}

func (service *serviceApp) FirstDevice(ctx context.Context) (response domainApp.DevicesResponse, err error) {
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return response, err
	}
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := whatsapp.GetDB().GetFirstDevice(ctx)
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

func (service *serviceApp) FetchDevices(ctx context.Context) (response []domainApp.DevicesResponse, err error) {
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return response, err
	}
	if client == nil {
		return response, pkgError.ErrWaCLI
	}

	devices, err := whatsapp.GetDB().GetAllDevices(ctx)
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

// Context-aware methods for multi-user support

func (service *serviceApp) LoginWithContext(appCtx *domainApp.AppContext) (response domainApp.LoginResponse, err error) {
	if appCtx.UserID == 0 {
		return service.Login(appCtx.Context)
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
				} else {
					logrus.Error("error when get qrCode", evt.Event, evt.Error)
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
		return service.LoginWithCode(appCtx.Context, phoneNumber)
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
		return service.Logout(appCtx.Context)
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
		return service.Reconnect(appCtx.Context)
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
		return service.FirstDevice(appCtx.Context)
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
		return service.FetchDevices(appCtx.Context)
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
