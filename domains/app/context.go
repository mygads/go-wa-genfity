package app

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// AppContext wraps context with additional user information
type AppContext struct {
	context.Context
	FiberCtx *fiber.Ctx
	UserID   int
	Username string
}

// NewAppContext creates a new app context from fiber context
func NewAppContext(ctx context.Context, fiberCtx *fiber.Ctx) *AppContext {
	appCtx := &AppContext{
		Context:  ctx,
		FiberCtx: fiberCtx,
	}

	// Extract user information if available
	if userID, ok := fiberCtx.Locals("user_id").(int); ok {
		appCtx.UserID = userID
	}

	if username, ok := fiberCtx.Locals("username").(string); ok {
		appCtx.Username = username
	}

	return appCtx
}
