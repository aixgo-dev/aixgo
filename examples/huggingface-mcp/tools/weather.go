package tools

import (
	"context"
	"fmt"

	"github.com/aixgo-dev/aixgo/pkg/mcp"
)

// RegisterWeatherTools registers weather-related tools
func RegisterWeatherTools(server *mcp.Server) error {
	return server.RegisterTool(mcp.Tool{
		Name:        "get_weather",
		Description: "Get current weather for a city",
		Handler:     getWeather,
		Schema: mcp.Schema{
			"city": mcp.SchemaField{
				Type:        "string",
				Description: "City name",
				Required:    true,
			},
			"unit": mcp.SchemaField{
				Type:        "string",
				Description: "Temperature unit (celsius or fahrenheit)",
				Default:     "celsius",
			},
		},
	})
}

// getTemperatureForUnit returns an appropriate temperature for the given unit
func getTemperatureForUnit(unit string) int {
	if unit == "fahrenheit" {
		return 72 // 72°F
	}
	return 22 // 22°C (approximately 72°F)
}

func getWeather(ctx context.Context, args mcp.Args) (any, error) {
	city := args.String("city")
	unit := args.String("unit")

	if city == "" {
		return nil, fmt.Errorf("city is required")
	}

	if unit == "" {
		unit = "celsius"
	}

	// Mock weather data with unit-appropriate temperature
	weather := map[string]any{
		"city":        city,
		"temperature": getTemperatureForUnit(unit),
		"unit":        unit,
		"condition":   "sunny",
		"humidity":    65,
		"wind_speed":  10,
	}

	return weather, nil
}
