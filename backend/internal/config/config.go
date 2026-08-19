package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Port                     string
	DatabaseURL              string
	SecretKey                string
	Algorithm                string
	AccessTokenExpireMinutes int
	APIV1Str                 string
	FrontendURL              string
	FirebaseCredentials      string
	FirebaseProjectID        string
	GeminiAPIKey             string
	SMSAPIBaseURL            string
	TelegramServiceURL       string
	WeatherAPIKey            string
	DataGovKey               string
	AgmarknetID              string
	Environment              string
	CORSOrigins              []string

	// Agent pipeline model names, configurable per role (Step 6).
	AgentPlannerModel string
	AgentModel        string
	AgentCheckerModel string

	// UploadsDir is where uploaded file bytes are stored on local disk (Step 9),
	// matching the Python uploads/ convention. Files are also uploaded to the
	// Gemini File API; the local copy supports re-upload after Gemini's 48h TTL.
	UploadsDir string
}

// Load reads configuration from the environment.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                     getEnv("PORT", "8000"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		SecretKey:                os.Getenv("SECRET_KEY"),
		Algorithm:                getEnv("ALGORITHM", "HS256"),
		AccessTokenExpireMinutes: getEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES", 60),
		APIV1Str:                 getEnv("API_V1_STR", "/api/v1"),
		FrontendURL:              os.Getenv("FRONTEND_URL"),
		FirebaseCredentials:      os.Getenv("FIREBASE_CREDENTIALS"),
		FirebaseProjectID:        getEnv("FIREBASE_PROJECT_ID", "smartkrishi-83352"),
		GeminiAPIKey:             firstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY")),
		SMSAPIBaseURL:            os.Getenv("SMS_API_BASE_URL"),
		TelegramServiceURL:       os.Getenv("TELEGRAM_SERVICE_URL"),
		WeatherAPIKey:            os.Getenv("WEATHERAPI_KEY"),
		DataGovKey:               os.Getenv("DATA_GOV_KEY"),
		AgmarknetID:              os.Getenv("AGMARKNET_ID"),
		Environment:              getEnv("ENVIRONMENT", "development"),
		AgentPlannerModel:        getEnv("AGENT_PLANNER_MODEL", "gemini-2.5-flash"),
		AgentModel:               getEnv("AGENT_MODEL", "gemini-2.5-flash"),
		AgentCheckerModel:        getEnv("AGENT_CHECKER_MODEL", "gemini-2.5-flash"),
		UploadsDir:               getEnv("UPLOADS_DIR", "uploads"),
	}

	cfg.CORSOrigins = defaultCORSOrigins(cfg.FrontendURL)
	return cfg, nil
}

func defaultCORSOrigins(frontendURL string) []string {
	origins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
		"https://smart-krishi-nine.vercel.app",
		"https://smart-krishi-website.vercel.app",
	}
	if frontendURL != "" && !contains(origins, frontendURL) {
		origins = append(origins, frontendURL)
	}
	return origins
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Addr returns the HTTP listen address.
func (c *Config) Addr() string {
	port := c.Port
	if port == "" {
		port = "8000"
	}
	return fmt.Sprintf(":%s", port)
}
