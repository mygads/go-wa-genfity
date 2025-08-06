package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	infraUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/usermanagement"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/mcp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/helpers"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/usecase"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start WhatsApp MCP server using SSE",
	Long:  `Start a WhatsApp MCP (Model Context Protocol) server using Server-Sent Events (SSE) transport. This allows AI agents to interact with WhatsApp through a standardized protocol.`,
	Run:   mcpServer,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().StringVar(&config.McpPort, "port", "8080", "Port for the SSE MCP server")
	mcpCmd.Flags().StringVar(&config.McpHost, "host", "localhost", "Host for the SSE MCP server")
}

func mcpServer(_ *cobra.Command, _ []string) {
	// Initialize user management system for MCP server (required for multi-user system)
	userManagementRepo, err := infraUserManagement.NewUserManagementRepository(config.UserManagementDBURI)
	if err != nil {
		logrus.Errorf("Failed to initialize user management repository: %v", err)
		logrus.Error("User management is required for multi-user system. Please check database configuration.")
		return
	}

	userManagementUsecase := usecase.NewUserManagementUsecase(userManagementRepo, chatStorageRepo)
	// Set auto reconnect to whatsapp server after booting with user management support
	go helpers.SetAutoConnectAfterBootingWithUserManagement(appUsecase, userManagementUsecase, chatStorageRepo)
	// Set auto reconnect checking for all user sessions
	go helpers.SetAutoReconnectCheckingForAllUsers()

	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"WhatsApp Web Multidevice MCP Server",
		config.AppVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	// Add all WhatsApp tools
	sendHandler := mcp.InitMcpSend(sendUsecase)
	sendHandler.AddSendTools(mcpServer)

	// Create SSE server
	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(fmt.Sprintf("http://%s:%s", config.McpHost, config.McpPort)),
		server.WithKeepAlive(true),
	)

	// Start the SSE server
	addr := fmt.Sprintf("%s:%s", config.McpHost, config.McpPort)
	logrus.Printf("Starting WhatsApp MCP SSE server on %s", addr)
	logrus.Printf("SSE endpoint: http://%s:%s/sse", config.McpHost, config.McpPort)
	logrus.Printf("Message endpoint: http://%s:%s/message", config.McpHost, config.McpPort)

	if err := sseServer.Start(addr); err != nil {
		logrus.Fatalf("Failed to start SSE server: %v", err)
	}
}
