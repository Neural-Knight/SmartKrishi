import os, requests, logging
log = logging.getLogger("WeatherAPI")

def weather_api(location: str) -> dict:
    key = os.getenv("WEATHERAPI_KEY")
    if not key:
        log.warning("WEATHERAPI_KEY not set, returning fallback data")
        return {"loc": location, "forecast": "Unavailable (no API key)", "source": "fallback"}
    url = "https://api.weatherapi.com/v1/current.json"
    try:
        params = {"key": key, "q": location, "aqi": "no"}
        r = requests.get(url, params=params, timeout=5)
        r.raise_for_status()
        data = r.json()
        # detect API error
        if "error" in data:
            err = data["error"].get("message", "Unknown error")
            log.warning("WeatherAPI returned error: %s", err)
            return {"loc": location, "forecast": f"Error: {err}", "source": "weatherapi.com"}
        loc_data = data.get("location", {})
        current = data.get("current", {})
        loc_name = loc_data.get("name") or location
        cond = current.get("condition", {}).get("text") or "Unknown"
        temp_c = current.get("temp_c")
        temp_str = f"{temp_c}°C" if temp_c is not None else "Unknown"
        result = {
            "loc": loc_name,
            "forecast": f"{cond} at {temp_str}",
            "temp_c": temp_c,
            "humidity": current.get("humidity"),
            "wind_kph": current.get("wind_kph"),
            "source": "weatherapi.com",
        }
        return result
    except Exception as e:
        log.warning("WeatherAPI exception: %s", e)
        return {"loc": location, "forecast": "Unavailable", "source": "fallback"}
