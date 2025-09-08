package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	grpctransport "github.com/metoro-io/mcp-golang/transport/grpc"
)

// HelloArgs represents the arguments for the hello tool
type HelloArgs struct {
	Name string `json:"name" jsonschema:"required,description=The name to say hello to"`
}

type WeatherArguments struct {
	Longitude float64 `json:"longitude" jsonschema:"required,description=The longitude of the location to get the weather for"`
	Latitude  float64 `json:"latitude" jsonschema:"required,description=The latitude of the location to get the weather for"`
}

func main() {
	// Create a new server with grpc transport
	server := mcp.NewServer(grpctransport.NewGrpcServerTransport())

	// Register a simple tool with the server
	err := server.RegisterTool("hello", "Says hello", func(args HelloArgs) (*mcp.ToolResponse, error) {
		message := fmt.Sprintf("Hello, %s!", args.Name)
		return mcp.NewToolResponse(mcp.NewTextContent(message)), nil
	})
	if err != nil {
		panic(err)
	}

	err = server.RegisterTool("get_weather", "Get the weather forecast for temperature, wind speed and relative humidity", func(arguments WeatherArguments) (*mcp.ToolResponse, error) {
		url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current=temperature_2m,wind_speed_10m&hourly=temperature_2m,relative_humidity_2m,wind_speed_10m", arguments.Latitude, arguments.Longitude)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		output, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(output))), nil
	})

	// Start the server
	err = server.Serve()
	if err != nil {
		panic(err)
	}

	// Keep the server running
	// select {}
	time.Sleep(5 * time.Second)
}
