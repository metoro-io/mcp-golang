package main

import (
	"log"
	"os"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func init() {
	logFile, _ := os.OpenFile("./server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	log.SetOutput(logFile)
}

// HelloArgs ...
type HelloArgs struct {
	Name string `json:"name" jsonschema:"required,description=The name to say hello to"`
}

func main() {
	// Create a transport for the server
	serverTransport := stdio.NewStdioServerTransport()

	// Create a new server with the transport
	server := mcp.NewServer(serverTransport, mcp.WithLoggingCapability())

	if err := server.RegisterTool("log", "get some log", func(k HelloArgs) (*mcp.ToolResponse, error) {
		if err := server.SendLogMessageNotification(mcp.LevelDebug, "server", "debug"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelInfo, "server", "info"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelNotice, "server", "notice"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelWarning, "server", "warning"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelError, "server", "error"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelCritical, "server", "critical"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelAlert, "server", "alert"); err != nil {
			log.Panic(err)
		}
		if err := server.SendLogMessageNotification(mcp.LevelEmergency, "server", "emergency"); err != nil {
			log.Panic(err)
		}
		return &mcp.ToolResponse{}, nil
	}); err != nil {
		log.Panic(err)
	}

	// Start the server
	if err := server.Serve(); err != nil {
		log.Printf("failed to serve: %v\n", err)
		panic(err)
	}
	log.Println("server running")
	// Keep the server running
	select {}
}
