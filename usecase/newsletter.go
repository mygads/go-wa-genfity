package usecase

import (
	"context"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"go.mau.fi/whatsmeow"
)

type serviceNewsletter struct{}

func NewNewsletterService() domainNewsletter.INewsletterUsecase {
	return &serviceNewsletter{}
}

// getClientFromContext extracts WhatsApp client from app context for user-specific operations
func (service serviceNewsletter) getClientFromContext(ctx context.Context) (*whatsmeow.Client, error) {
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

func (service serviceNewsletter) Unfollow(ctx context.Context, request domainNewsletter.UnfollowRequest) (err error) {
	if err = validations.ValidateUnfollowNewsletter(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	JID, err := utils.ValidateJidWithLogin(client, request.NewsletterID)
	if err != nil {
		return err
	}

	return client.UnfollowNewsletter(JID)
}
