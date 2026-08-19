package tools

import (
	"context"
	"fmt"
	"net/url"
)

const weatherAPICurrentURL = "https://api.weatherapi.com/v1/current.json"

// Weather returns current conditions for a location via weatherapi.com. It
// ports Python weather_api(location). On a missing key or any error it returns
// a fallback payload (never an error) so the pipeline degrades gracefully, just
// like the Python version.
//
// The returned map mirrors the Python dict keys (loc, forecast, temp_c,
// humidity, wind_kph, source) so downstream prompt serialization is unchanged.
func (r *Registry) Weather(ctx context.Context, location string) map[string]any {
	if r.cfg.WeatherAPIKey == "" {
		return map[string]any{"loc": location, "forecast": "Unavailable (no API key)", "source": "fallback"}
	}

	base := r.cfg.WeatherBaseURL
	if base == "" {
		base = weatherAPICurrentURL
	}
	q := url.Values{}
	q.Set("key", r.cfg.WeatherAPIKey)
	q.Set("q", location)
	q.Set("aqi", "no")
	full := base + "?" + q.Encode()

	var data weatherResponse
	if err := r.getJSON(ctx, full, &data); err != nil {
		return map[string]any{"loc": location, "forecast": "Unavailable", "source": "fallback"}
	}
	if data.Error != nil && data.Error.Message != "" {
		return map[string]any{"loc": location, "forecast": "Error: " + data.Error.Message, "source": "weatherapi.com"}
	}

	locName := location
	if data.Location.Name != "" {
		locName = data.Location.Name
	}
	cond := "Unknown"
	if data.Current.Condition.Text != "" {
		cond = data.Current.Condition.Text
	}
	tempStr := "Unknown"
	if data.Current.TempC != nil {
		tempStr = fmt.Sprintf("%g°C", *data.Current.TempC)
	}

	return map[string]any{
		"loc":      locName,
		"forecast": fmt.Sprintf("%s at %s", cond, tempStr),
		"temp_c":   data.Current.TempC,
		"humidity": data.Current.Humidity,
		"wind_kph": data.Current.WindKph,
		"source":   "weatherapi.com",
	}
}

type weatherResponse struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Current struct {
		TempC     *float64 `json:"temp_c"`
		Humidity  *float64 `json:"humidity"`
		WindKph   *float64 `json:"wind_kph"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
	} `json:"current"`
}
