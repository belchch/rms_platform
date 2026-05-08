package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port             string
	DatabaseURL      string
	JWTSecret        string
	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	if s3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required")
	}
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	s3Access := os.Getenv("S3_ACCESS_KEY")
	s3Secret := os.Getenv("S3_SECRET_KEY")
	if s3Access == "" || s3Secret == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:             port,
		DatabaseURL:      databaseURL,
		JWTSecret:        jwtSecret,
		S3Endpoint:       s3Endpoint,
		S3PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		S3AccessKey:      s3Access,
		S3SecretKey:      s3Secret,
		S3Bucket:         s3Bucket,
	}, nil
}
