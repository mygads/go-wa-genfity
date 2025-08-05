package main

import (
	"fmt"
	"log"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	infraUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
)

func main() {
	// Set the database URI (relative to src folder)
	config.UserManagementDBURI = "file:../storages/usermanagement.db?_foreign_keys=on"

	// Initialize user management system
	userManagementRepo, err := infraUserManagement.NewUserManagementRepository(config.UserManagementDBURI)
	if err != nil {
		log.Fatalf("Failed to initialize user management repository: %v", err)
	}

	userManagementUsecase := usecase.NewUserManagementUsecase(userManagementRepo, nil)

	// Create initial users
	users := []domainUserManagement.CreateUserRequest{
		{Username: "user1", Password: "password1"},
		{Username: "user2", Password: "password2"},
		{Username: "admin_user", Password: "admin123"},
		{Username: "mobile_user", Password: "mobile456"},
	}

	fmt.Println("Creating initial users...")
	for _, user := range users {
		response, err := userManagementUsecase.CreateUser(user)
		if err != nil {
			log.Printf("Failed to create user %s: %v", user.Username, err)
			continue
		}
		fmt.Printf("✓ Created user: %s (ID: %d)\n", response.Username, response.ID)
	}

	fmt.Println("\nInitial users created successfully!")
	fmt.Println("You can now remove the APP_BASIC_AUTH from .env file")
	fmt.Println("Use the following API endpoints to manage users:")
	fmt.Println("- POST /admin/users - Create user")
	fmt.Println("- GET /admin/users - Get all users")
	fmt.Println("- GET /admin/users/:id - Get user by ID")
	fmt.Println("- PUT /admin/users/:id - Update user")
	fmt.Println("- DELETE /admin/users/:id - Delete user")
	fmt.Println("\nAdmin credentials (set in .env):")
	fmt.Println("- ADMIN_USERNAME=admin")
	fmt.Println("- ADMIN_PASSWORD=admin123")
}
