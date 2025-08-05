package cmd

import (
	"fmt"
	"net/http"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	infraUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/websocket"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var restCmd = &cobra.Command{
	Use:   "rest",
	Short: "Send whatsapp API over http",
	Long:  `This application is from clone https://github.com/aldinokemal/go-whatsapp-web-multidevice`,
	Run:   restServer,
}

func init() {
	rootCmd.AddCommand(restCmd)
}
func restServer(_ *cobra.Command, _ []string) {
	// Initialize user management system
	userManagementRepo, err := infraUserManagement.NewUserManagementRepository(config.UserManagementDBURI)
	if err != nil {
		logrus.Fatalf("Failed to initialize user management repository: %v", err)
	}
	userManagementUsecase := usecase.NewUserManagementUsecase(userManagementRepo, chatStorageRepo)

	engine := html.NewFileSystem(http.FS(EmbedIndex), ".html")
	engine.AddFunc("isEnableBasicAuth", func(token any) bool {
		return token != nil
	})
	app := fiber.New(fiber.Config{
		Views:     engine,
		BodyLimit: int(config.WhatsappSettingMaxVideoSize),
		Network:   "tcp",
	})

	app.Static(config.AppBasePath+"/statics", "./statics")
	app.Use(config.AppBasePath+"/components", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/components",
		Browse:     true,
	}))
	app.Use(config.AppBasePath+"/assets", filesystem.New(filesystem.Config{
		Root:       http.FS(EmbedViews),
		PathPrefix: "views/assets",
		Browse:     true,
	}))

	app.Use(middleware.Recovery())
	if config.AppDebug {
		app.Use(logger.New())
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Create base path group or use app directly
	var apiGroup fiber.Router = app
	if config.AppBasePath != "" {
		apiGroup = app.Group(config.AppBasePath)
	}

	// Admin routes with admin authentication
	adminGroup := apiGroup.Group("/admin", middleware.AdminBasicAuth())
	rest.InitRestUserManagement(adminGroup, userManagementUsecase)

	// Homepage route (protected with basic user authentication but not session middleware)
	apiGroup.Get("/", middleware.UserBasicAuth(userManagementUsecase), func(c *fiber.Ctx) error {
		return c.Render("views/index", fiber.Map{
			"AppHost":        fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname()),
			"AppVersion":     config.AppVersion,
			"AppBasePath":    config.AppBasePath,
			"BasicAuthToken": c.UserContext().Value(middleware.AuthorizationValue("BASIC_AUTH")),
			"MaxFileSize":    humanize.Bytes(uint64(config.WhatsappSettingMaxFileSize)),
			"MaxVideoSize":   humanize.Bytes(uint64(config.WhatsappSettingMaxVideoSize)),
		})
	})

	// Routes with basic user authentication only (for login, status, etc.)
	basicUserRoutes := apiGroup.Group("/", middleware.UserBasicAuth(userManagementUsecase))

	// App routes that need session middleware (for operations requiring active WhatsApp session)
	sessionUserRoutes := apiGroup.Group("/", middleware.UserSessionMiddleware(userManagementUsecase, chatStorageRepo))

	// Initialize REST routes with appropriate middleware
	rest.InitRestApp(basicUserRoutes, appUsecase)                 // Login doesn't need session
	rest.InitRestChat(sessionUserRoutes, chatUsecase)             // Chat operations need session
	rest.InitRestSend(sessionUserRoutes, sendUsecase)             // Send operations need session
	rest.InitRestUser(basicUserRoutes, userUsecase)               // User info doesn't need session
	rest.InitRestMessage(sessionUserRoutes, messageUsecase)       // Message operations need session
	rest.InitRestGroup(sessionUserRoutes, groupUsecase)           // Group operations need session
	rest.InitRestNewsletter(sessionUserRoutes, newsletterUsecase) // Newsletter operations need session

	websocket.RegisterRoutes(basicUserRoutes, appUsecase)
	go websocket.RunHub()

	// Set auto reconnect to whatsapp server after booting
	go helpers.SetAutoConnectAfterBooting(appUsecase)
	// Set auto reconnect checking
	go helpers.SetAutoReconnectChecking(whatsappCli)

	if err := app.Listen(":" + config.AppPort); err != nil {
		logrus.Fatalln("Failed to start: ", err.Error())
	}
}
