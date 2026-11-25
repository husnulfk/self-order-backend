package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port                 string
	AppEnv               string
	DatabaseURL          string
	JWTSecret            string
	JWTExpiryHours       int
	AllowedOrigins       []string
	BaseURL              string
	PaymentGateway       string
	PaymentWebhookSecret string
	MidtransServerKey    string
	MidtransProduction   bool
	OrderExpiryMinutes   int
	AutoMigrate          bool
	Seed                 bool
	AdminUsername        string
	AdminPassword        string
}

func Load() *Config {
	loadDotEnv(".env")
	return &Config{
		Port:                 getEnv("PORT", ""),
		AppEnv:               getEnv("APP_ENV", ""),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		JWTSecret:            getEnv("JWT_SECRET", ""),
		JWTExpiryHours:       getEnvInt("JWT_EXPIRY_HOURS", 0),
		AllowedOrigins:       splitCSV(getEnv("ALLOWED_ORIGINS", "")),
		BaseURL:              getEnv("BASE_URL", ""),
		PaymentGateway:       getEnv("PAYMENT_GATEWAY", ""),
		PaymentWebhookSecret: getEnv("PAYMENT_WEBHOOK_SECRET", ""),
		MidtransServerKey:    getEnv("MIDTRANS_SERVER_KEY", ""),
		MidtransProduction:   getEnvBool("MIDTRANS_PRODUCTION", false),
		OrderExpiryMinutes:   getEnvInt("ORDER_EXPIRY_MINUTES", 15),
		AutoMigrate:          getEnvBool("AUTO_MIGRATE", true),
		Seed:                 getEnvBool("SEED", true),
		AdminUsername:        getEnv("ADMIN_USERNAME", ""),
		AdminPassword:        getEnv("ADMIN_PASSWORD", ""),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// loadDotEnv reads simple KEY=VALUE pairs from path and sets them as
// environment variables. Real environment variables always win, so
// container/production configuration is never overridden by a stray file.
// A missing file is not an error.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip matching surrounding quotes, if any.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
