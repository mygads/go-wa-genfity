package app

import (
	"time"
)

// IAppUsecaseWithContext interface for multi-user WhatsApp operations
type IAppUsecaseWithContext interface {
	LoginWithContext(appCtx *AppContext) (response LoginResponse, err error)
	LoginWithCodeAndContext(appCtx *AppContext, phoneNumber string) (loginCode string, err error)
	LogoutWithContext(appCtx *AppContext) (err error)
	ReconnectWithContext(appCtx *AppContext) (err error)
	FirstDeviceWithContext(appCtx *AppContext) (response DevicesResponse, err error)
	FetchDevicesWithContext(appCtx *AppContext) (response []DevicesResponse, err error)
}

type DevicesResponse struct {
	Name   string `json:"name"`
	Device string `json:"device"`
}

type LoginResponse struct {
	ImagePath string        `json:"image_path"`
	Duration  time.Duration `json:"duration"`
	Code      string        `json:"code"`
}
