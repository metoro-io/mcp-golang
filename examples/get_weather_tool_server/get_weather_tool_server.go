package main

import (
	"fmt"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"io"
	"net/http"
)

type WeatherArguments struct {
	Longitude float64 `json:"longitude" jsonschema:"required,description=The longitude of the location to get the weather for"`
	Latitude  float64 `json:"latitude" jsonschema:"required,description=The latitude of the location to get the weather for"`
}

// This is explained in the docs at https://mcpgolang.com/tools
func main() {
	done := make(chan struct{})
	server := mcp_golang.NewServer(stdio.NewStdioServerTransport())
	err := server.RegisterTool("get_weather", "Get the weather forecast for temperature, wind speed and relative humidity", func(arguments WeatherArguments) (*mcp_golang.ToolResponse, error) {
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
		return mcp_golang.NewToolReponse(mcp_golang.NewTextContent(string(output))), nil
	})
	err = server.Serve()
	if err != nil {
		panic(err)
	}
	<-done
}
