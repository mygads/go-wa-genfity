package app

import (
	"context"
	"time"
)

type IAppUsecase interface {
	Login(ctx context.Context) (response LoginResponse, err error)
	LoginWithCode(ctx context.Context, phoneNumber string) (loginCode string, err error)
	Logout(ctx context.Context) (err error)
	Reconnect(ctx context.Context) (err error)
	FirstDevice(ctx context.Context) (response DevicesResponse, err error)
	FetchDevices(ctx context.Context) (response []DevicesResponse, err error)
}

// IAppUsecaseWithContext extends IAppUsecase with context-aware methods
type IAppUsecaseWithContext interface {
	IAppUsecase
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
