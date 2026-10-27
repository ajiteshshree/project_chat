package main

import "os"

type Config struct {
	Addr        string
	JWTSecret   []byte
	MinIOBucket string
}

type minioCfg struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

func loadConfig() Config {
	return Config{
		Addr:        getenv("PORT", "8080"),
		JWTSecret:   []byte(getenv("JWT_SECRET", "dev-secret-change-me")),
		MinIOBucket: getenv("MINIO_BUCKET", "chat-files"),
	}
}

func loadMinIO() minioCfg {
	return minioCfg{
		Endpoint:  getenv("MINIO_ENDPOINT", "minio:9000"),
		AccessKey: getenv("MINIO_ACCESS_KEY", getenv("MINIO_ROOT_USER", "minioadmin")),
		SecretKey: getenv("MINIO_SECRET_KEY", getenv("MINIO_ROOT_SECRET", "minioadmin")),
		Bucket:    getenv("MINIO_BUCKET", "chat-files"),
		UseSSL:    os.Getenv("MINIO_USE_SSL") == "true",
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}