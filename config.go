package main

import (
	"os"
)

type Config struct {
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
	DBKhanzaHost     string
	DBKhanzaPort     string
	DBKhanzaUser     string
	DBKhanzaPassword string
	DBKhanzaName     string
	OrthancURL       string
	OHIFURL          string
}

func LoadConfig() Config {
	return Config{
		DBHost:           os.Getenv("MIDDLEWARE_DB_HOST"),
		DBPort:           os.Getenv("MIDDLEWARE_DB_PORT"),
		DBUser:           os.Getenv("MIDDLEWARE_DB_USER"),
		DBPassword:       os.Getenv("MIDDLEWARE_DB_PASSWORD"),
		DBName:           os.Getenv("MIDDLEWARE_DB_NAME"),
		DBKhanzaHost:     os.Getenv("KHANZA_DB_HOST"),
		DBKhanzaPort:     os.Getenv("KHANZA_DB_PORT"),
		DBKhanzaUser:     os.Getenv("KHANZA_DB_USER"),
		DBKhanzaPassword: os.Getenv("KHANZA_DB_PASSWORD"),
		DBKhanzaName:     os.Getenv("KHANZA_DB_NAME"),
		OrthancURL:       os.Getenv("ORTHANC_URL"),
		OHIFURL:          os.Getenv("OHIF_URL"),
	}
}
