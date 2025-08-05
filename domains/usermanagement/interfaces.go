package usermanagement

type IUserManagementRepository interface {
	Create(user *User) error
	GetByID(id int) (*User, error)
	GetByUsername(username string) (*User, error)
	GetAll() ([]User, error)
	Update(id int, user *UpdateUserRequest) error
	Delete(id int) error
	GetActiveUsers() ([]User, error)
	ValidateCredentials(username, password string) bool
}

type IUserManagementUsecase interface {
	CreateUser(request CreateUserRequest) (*UserResponse, error)
	GetUser(id int) (*UserResponse, error)
	GetAllUsers() ([]UserResponse, error)
	UpdateUser(id int, request UpdateUserRequest) (*UserResponse, error)
	DeleteUser(id int) error
	ValidateUserCredentials(username, password string) bool
	GetActiveUserCredentials() map[string]string
	// WhatsApp Session Management for Admin
	DisconnectWhatsAppSession(userID int) error
	ReconnectWhatsAppSession(userID int) error
	ClearWhatsAppSession(userID int) error
}
