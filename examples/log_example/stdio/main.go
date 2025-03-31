package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func main() {
	// Start the server process
	cmd := exec.Command("go", "run", "./server/main.go")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	clientTransport := stdio.NewStdioServerTransportWithIO(stdout, stdin)
	client := mcp.NewClient(clientTransport, mcp.WithNotificationHandler("notifications/message", func(notification *transport.BaseJSONRPCNotification) error {
		var params struct {
			Level  string      `json:"level" yaml:"level" mapstructure:"level"`
			Logger string      `json:"logger" yaml:"logger" mapstructure:"logger"`
			Data   interface{} `json:"data" yaml:"data" mapstructure:"data"`
		}
		if err := json.Unmarshal(notification.Params, &params); err != nil {
			log.Println("failed to unmarshal log_example params:", err.Error())
			return fmt.Errorf("failed to unmarshal log_example params: %w", err)
		}
		log.Printf("[%s] Notification: %s", params.Level, params.Data)
		return nil
	}))

	if _, err := client.Initialize(context.Background()); err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	for _, level := range []mcp.Level{
		mcp.LevelDebug,
		mcp.LevelInfo,
		mcp.LevelNotice,
		mcp.LevelWarning,
		mcp.LevelError,
		mcp.LevelCritical,
		mcp.LevelAlert,
		mcp.LevelEmergency,
	} {
		if err := client.SetLoggingLevel(context.Background(), level); err != nil {
			log.Fatalf("Failed to set logging level: %v", err)
		}
		args := map[string]interface{}{
			"name": "World",
		}
		_, err := client.CallTool(context.Background(), "log", args)
		if err != nil {
			log.Printf("Failed to call log tool: %v", err)
		}
		// wait all notifications arrive
		time.Sleep(3 * time.Second)
		log.Println("----------------------------------")
	}
}
