package websocket

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

const (
	UserIDKey   = "user_id"
	UsernameKey = "username"
)

// Helper functions to extract user info from fiber context
func getUserIDFromContext(c *fiber.Ctx) (int, bool) {
	userID, ok := c.Locals(UserIDKey).(int)
	return userID, ok
}

func getUsernameFromContext(c *fiber.Ctx) (string, bool) {
	username, ok := c.Locals(UsernameKey).(string)
	return username, ok
}

type client struct {
	userID   int
	username string
}

type BroadcastMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  any    `json:"result"`
}

var (
	Clients    = make(map[*websocket.Conn]client)
	Register   = make(chan *websocket.Conn)
	Broadcast  = make(chan BroadcastMessage)
	Unregister = make(chan *websocket.Conn)
)

func handleRegister(conn *websocket.Conn) {
	// Extract user info from connection
	var userID int
	var username string

	if locals := conn.Locals("user_id"); locals != nil {
		if id, ok := locals.(int); ok {
			userID = id
		}
	}
	if locals := conn.Locals("username"); locals != nil {
		if name, ok := locals.(string); ok {
			username = name
		}
	}

	Clients[conn] = client{
		userID:   userID,
		username: username,
	}
	logrus.Printf("WebSocket connection registered for user %s (ID: %d)", username, userID)
}

func handleUnregister(conn *websocket.Conn) {
	if client, exists := Clients[conn]; exists {
		logrus.Printf("WebSocket connection unregistered for user %s (ID: %d)", client.username, client.userID)
	}
	delete(Clients, conn)
}

func broadcastMessage(message BroadcastMessage) {
	marshalMessage, err := json.Marshal(message)
	if err != nil {
		logrus.Println("marshal error:", err)
		return
	}

	for conn := range Clients {
		if err := conn.WriteMessage(websocket.TextMessage, marshalMessage); err != nil {
			logrus.Println("write error:", err)
			closeConnection(conn)
		}
	}
}

func closeConnection(conn *websocket.Conn) {
	if err := conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
		logrus.Println("write close message error:", err)
	}
	if err := conn.Close(); err != nil {
		logrus.Println("close connection error:", err)
	}
	delete(Clients, conn)
}

func RunHub() {
	for {
		select {
		case conn := <-Register:
			handleRegister(conn)

		case conn := <-Unregister:
			handleUnregister(conn)

		case message := <-Broadcast:
			logrus.Println("message received:", message)
			broadcastMessage(message)
		}
	}
}

func RegisterRoutes(app fiber.Router, service domainApp.IAppUsecaseWithContext) {
	// WebSocket route with user authentication middleware
	app.Get("/ws", func(c *fiber.Ctx) error {
		// Extract user information before upgrading to WebSocket
		userID, hasUserID := getUserIDFromContext(c)
		username, _ := getUsernameFromContext(c)

		if !websocket.IsWebSocketUpgrade(c) {
			return c.SendStatus(fiber.StatusUpgradeRequired)
		}

		return websocket.New(func(conn *websocket.Conn) {
			// Store user info in global clients map during registration
			Clients[conn] = client{
				userID:   userID,
				username: username,
			}

			defer func() {
				Unregister <- conn
				_ = conn.Close()
			}()

			Register <- conn

			for {
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logrus.Println("read error:", err)
					}
					return
				}

				if messageType == websocket.TextMessage {
					var messageData BroadcastMessage
					if err := json.Unmarshal(message, &messageData); err != nil {
						logrus.Println("unmarshal error:", err)
						return
					}

					if messageData.Code == "FETCH_DEVICES" {
						var devices []domainApp.DevicesResponse
						var err error

						// Use context-aware method if user info is available
						if hasUserID && userID > 0 {
							logrus.Infof("[WebSocket] Fetching devices for user %s (ID: %d)", username, userID)

							// Create app context for the user
							appCtx := &domainApp.AppContext{
								Context:  context.Background(),
								UserID:   userID,
								Username: username,
							}
							devices, err = service.FetchDevicesWithContext(appCtx)
						} else {
							logrus.Info("[WebSocket] Fetching devices using fallback method")
							devices, err = service.FetchDevices(context.Background())
						}

						if err != nil {
							logrus.Errorf("[WebSocket] Error fetching devices: %v", err)
							Broadcast <- BroadcastMessage{
								Code:    "FETCH_DEVICES_ERROR",
								Message: "Failed to fetch devices: " + err.Error(),
								Result:  nil,
							}
						} else {
							logrus.Infof("[WebSocket] Successfully fetched %d devices for user %s", len(devices), username)
							Broadcast <- BroadcastMessage{
								Code:    "LIST_DEVICES",
								Message: "Device found",
								Result:  devices,
							}
						}
					}
				} else {
					logrus.Println("unsupported message type:", messageType)
				}
			}
		})(c)
	})
}
