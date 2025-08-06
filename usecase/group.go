package usecase

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type serviceGroup struct{}

func NewGroupService() domainGroup.IGroupUsecase {
	return &serviceGroup{}
}

// getClientFromContext extracts WhatsApp client from app context for user-specific operations
func (service serviceGroup) getClientFromContext(ctx context.Context) (*whatsmeow.Client, error) {
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

func (service serviceGroup) JoinGroupWithLink(ctx context.Context, request domainGroup.JoinGroupWithLinkRequest) (groupID string, err error) {
	if err = validations.ValidateJoinGroupWithLink(ctx, request); err != nil {
		return groupID, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return groupID, err
	}
	utils.MustLogin(client)

	jid, err := client.JoinGroupWithLink(request.Link)
	if err != nil {
		return
	}
	return jid.String(), nil
}

func (service serviceGroup) LeaveGroup(ctx context.Context, request domainGroup.LeaveGroupRequest) (err error) {
	if err = validations.ValidateLeaveGroup(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	JID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return err
	}

	return client.LeaveGroup(JID)
}

func (service serviceGroup) CreateGroup(ctx context.Context, request domainGroup.CreateGroupRequest) (groupID string, err error) {
	if err = validations.ValidateCreateGroup(ctx, request); err != nil {
		return groupID, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return groupID, err
	}
	utils.MustLogin(client)

	participantsJID, err := service.participantToJID(ctx, request.Participants)
	if err != nil {
		return
	}

	groupConfig := whatsmeow.ReqCreateGroup{
		Name:              request.Title,
		Participants:      participantsJID,
		GroupParent:       types.GroupParent{},
		GroupLinkedParent: types.GroupLinkedParent{},
	}

	groupInfo, err := client.CreateGroup(groupConfig)
	if err != nil {
		return
	}

	return groupInfo.JID.String(), nil
}

func (service serviceGroup) GetGroupInfoFromLink(ctx context.Context, request domainGroup.GetGroupInfoFromLinkRequest) (response domainGroup.GetGroupInfoFromLinkResponse, err error) {
	if err = validations.ValidateGetGroupInfoFromLink(ctx, request); err != nil {
		return response, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return response, err
	}
	utils.MustLogin(client)

	groupInfo, err := client.GetGroupInfoFromLink(request.Link)
	if err != nil {
		return response, err
	}

	response = domainGroup.GetGroupInfoFromLinkResponse{
		GroupID:          groupInfo.JID.String(),
		Name:             groupInfo.Name,
		Topic:            groupInfo.Topic,
		CreatedAt:        groupInfo.GroupCreated,
		ParticipantCount: len(groupInfo.Participants),
		IsLocked:         groupInfo.IsLocked,
		IsAnnounce:       groupInfo.IsAnnounce,
		IsEphemeral:      groupInfo.IsEphemeral,
		Description:      groupInfo.Topic, // Topic serves as description
	}

	return response, nil
}

func (service serviceGroup) ManageParticipant(ctx context.Context, request domainGroup.ParticipantRequest) (result []domainGroup.ParticipantStatus, err error) {
	if err = validations.ValidateParticipant(ctx, request); err != nil {
		return result, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return result, err
	}
	utils.MustLogin(client)

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return result, err
	}

	participantsJID, err := service.participantToJID(ctx, request.Participants)
	if err != nil {
		return result, err
	}

	participants, err := client.UpdateGroupParticipants(groupJID, participantsJID, request.Action)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		if participant.Error == 403 && participant.AddRequest != nil {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "error",
				Message:     "Failed to add participant",
			})
		} else {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "success",
				Message:     "Action success",
			})
		}
	}

	return result, nil
}

func (service serviceGroup) GetGroupRequestParticipants(ctx context.Context, request domainGroup.GetGroupRequestParticipantsRequest) (result []domainGroup.GetGroupRequestParticipantsResponse, err error) {
	if err = validations.ValidateGetGroupRequestParticipants(ctx, request); err != nil {
		return result, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return result, err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return result, err
	}

	participants, err := client.GetGroupRequestParticipants(groupJID)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		result = append(result, domainGroup.GetGroupRequestParticipantsResponse{
			JID:         participant.JID.String(),
			RequestedAt: participant.RequestedAt,
		})
	}

	return result, nil
}

func (service serviceGroup) ManageGroupRequestParticipants(ctx context.Context, request domainGroup.GroupRequestParticipantsRequest) (result []domainGroup.ParticipantStatus, err error) {
	if err = validations.ValidateManageGroupRequestParticipants(ctx, request); err != nil {
		return result, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return result, err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return result, err
	}

	participantsJID, err := service.participantToJID(ctx, request.Participants)
	if err != nil {
		return result, err
	}

	participants, err := client.UpdateGroupRequestParticipants(groupJID, participantsJID, request.Action)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		if participant.Error != 0 {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "error",
				Message:     fmt.Sprintf("Action %s failed (code %d)", request.Action, participant.Error),
			})
		} else {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("Action %s success", request.Action),
			})
		}
	}

	return result, nil
}

func (service serviceGroup) participantToJID(ctx context.Context, participants []string) ([]types.JID, error) {
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var participantsJID []types.JID
	for _, participant := range participants {
		formattedParticipant := participant + config.WhatsappTypeUser

		if !utils.IsOnWhatsapp(client, formattedParticipant) {
			return nil, pkgError.ErrUserNotRegistered
		}

		if participantJID, err := types.ParseJID(formattedParticipant); err == nil {
			participantsJID = append(participantsJID, participantJID)
		}
	}
	return participantsJID, nil
}

func (service serviceGroup) SetGroupPhoto(ctx context.Context, request domainGroup.SetGroupPhotoRequest) (pictureID string, err error) {
	if err = validations.ValidateSetGroupPhoto(ctx, request); err != nil {
		return pictureID, err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return pictureID, err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return pictureID, err
	}

	var photoBytes []byte
	if request.Photo != nil {
		// Process the image for WhatsApp group photo requirements
		logrus.Printf("Processing group photo: %s (size: %d bytes)", request.Photo.Filename, request.Photo.Size)

		processedImageBuffer, err := utils.ProcessGroupPhoto(request.Photo)
		if err != nil {
			logrus.Printf("Failed to process group photo: %v", err)
			return pictureID, err
		}

		logrus.Printf("Successfully processed group photo: %d bytes -> %d bytes",
			request.Photo.Size, processedImageBuffer.Len())

		// Convert buffer to byte slice
		photoBytes = processedImageBuffer.Bytes()
	}

	pictureID, err = client.SetGroupPhoto(groupJID, photoBytes)
	if err != nil {
		logrus.Printf("Failed to set group photo: %v", err)
		return pictureID, err
	}

	return pictureID, nil
}

func (service serviceGroup) SetGroupName(ctx context.Context, request domainGroup.SetGroupNameRequest) (err error) {
	if err = validations.ValidateSetGroupName(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return err
	}

	return client.SetGroupName(groupJID, request.Name)
}

func (service serviceGroup) SetGroupLocked(ctx context.Context, request domainGroup.SetGroupLockedRequest) (err error) {
	if err = validations.ValidateSetGroupLocked(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return err
	}

	return client.SetGroupLocked(groupJID, request.Locked)
}

func (service serviceGroup) SetGroupAnnounce(ctx context.Context, request domainGroup.SetGroupAnnounceRequest) (err error) {
	if err = validations.ValidateSetGroupAnnounce(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return err
	}

	return client.SetGroupAnnounce(groupJID, request.Announce)
}

func (service serviceGroup) SetGroupTopic(ctx context.Context, request domainGroup.SetGroupTopicRequest) (err error) {
	if err = validations.ValidateSetGroupTopic(ctx, request); err != nil {
		return err
	}

	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return err
	}

	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return err
	}

	// SetGroupTopic with auto-generated IDs (previousID and newID will be handled automatically)
	return client.SetGroupTopic(groupJID, "", "", request.Topic)
}

// GroupInfo retrieves detailed information about a WhatsApp group
func (service serviceGroup) GroupInfo(ctx context.Context, request domainGroup.GroupInfoRequest) (response domainGroup.GroupInfoResponse, err error) {
	// Validate the incoming request
	if err = validations.ValidateGroupInfo(ctx, request); err != nil {
		return response, err
	}

	// Get user-specific client from context
	client, err := service.getClientFromContext(ctx)
	if err != nil {
		return response, err
	}

	// Ensure we are logged in
	utils.MustLogin(client)

	// Validate and parse the provided group JID / ID
	groupJID, err := utils.ValidateJidWithLogin(client, request.GroupID)
	if err != nil {
		return response, err
	}

	// Fetch group information from WhatsApp
	groupInfo, err := client.GetGroupInfo(groupJID)
	if err != nil {
		return response, err
	}

	// Map the response
	if groupInfo != nil {
		response.Data = *groupInfo
	}

	return response, nil
}
