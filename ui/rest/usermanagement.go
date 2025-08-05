package rest

import (
	"strconv"

	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type UserManagementRest struct {
	userUsecase domainUserManagement.IUserManagementUsecase
}

func NewUserManagementRest(userUsecase domainUserManagement.IUserManagementUsecase) UserManagementRest {
	return UserManagementRest{userUsecase: userUsecase}
}

func InitRestUserManagement(app fiber.Router, userUsecase domainUserManagement.IUserManagementUsecase) {
	rest := NewUserManagementRest(userUsecase)
	app.Post("/users", rest.CreateUser)
	app.Get("/users", rest.GetAllUsers)
	app.Get("/users/:id", rest.GetUser)
	app.Put("/users/:id", rest.UpdateUser)
	app.Delete("/users/:id", rest.DeleteUser)

	// Admin WhatsApp Session Management
	app.Post("/users/:id/whatsapp/disconnect", rest.DisconnectWhatsAppSession)
	app.Post("/users/:id/whatsapp/reconnect", rest.ReconnectWhatsAppSession)
	app.Post("/users/:id/whatsapp/clear", rest.ClearWhatsAppSession)
}

func (r *UserManagementRest) CreateUser(c *fiber.Ctx) error {
	var request domainUserManagement.CreateUserRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_REQUEST",
			Message: "Invalid request body",
		})
	}

	response, err := r.userUsecase.CreateUser(request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "CREATE_USER_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "User created successfully",
		Results: response,
	})
}

func (r *UserManagementRest) GetAllUsers(c *fiber.Ctx) error {
	users, err := r.userUsecase.GetAllUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ResponseData{
			Status:  fiber.StatusInternalServerError,
			Code:    "GET_USERS_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Users retrieved successfully",
		Results: users,
	})
}

func (r *UserManagementRest) GetUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	user, err := r.userUsecase.GetUser(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(utils.ResponseData{
			Status:  fiber.StatusNotFound,
			Code:    "USER_NOT_FOUND",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "User retrieved successfully",
		Results: user,
	})
}

func (r *UserManagementRest) UpdateUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	var request domainUserManagement.UpdateUserRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_REQUEST",
			Message: "Invalid request body",
		})
	}

	user, err := r.userUsecase.UpdateUser(id, request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "UPDATE_USER_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "User updated successfully",
		Results: user,
	})
}

func (r *UserManagementRest) DeleteUser(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	err = r.userUsecase.DeleteUser(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(utils.ResponseData{
			Status:  fiber.StatusNotFound,
			Code:    "DELETE_USER_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "User deleted successfully",
	})
}

// DisconnectWhatsAppSession disconnects WhatsApp session for specific user (admin only)
func (r *UserManagementRest) DisconnectWhatsAppSession(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	err = r.userUsecase.DisconnectWhatsAppSession(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "DISCONNECT_SESSION_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "WhatsApp session disconnected successfully",
		Results: map[string]any{
			"user_id": id,
			"action":  "disconnect",
			"status":  "disconnected",
		},
	})
}

// ReconnectWhatsAppSession reconnects WhatsApp session for specific user (admin only)
func (r *UserManagementRest) ReconnectWhatsAppSession(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	err = r.userUsecase.ReconnectWhatsAppSession(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "RECONNECT_SESSION_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "WhatsApp session reconnected successfully",
		Results: map[string]any{
			"user_id": id,
			"action":  "reconnect",
			"status":  "connected",
		},
	})
}

// ClearWhatsAppSession completely clears WhatsApp session and forces logout (admin only)
func (r *UserManagementRest) ClearWhatsAppSession(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID",
		})
	}

	err = r.userUsecase.ClearWhatsAppSession(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Status:  fiber.StatusBadRequest,
			Code:    "CLEAR_SESSION_FAILED",
			Message: err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "WhatsApp session cleared successfully",
		Results: map[string]any{
			"user_id": id,
			"action":  "clear",
			"status":  "cleared",
			"note":    "All session data removed, ready for new login",
		},
	})
}
