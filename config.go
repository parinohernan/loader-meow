package main

import (
	"fmt"
	"os"
)

// DatabaseConfig contiene la configuración de la base de datos MySQL
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Charset  string
}

// GetDatabaseConfig obtiene la configuración de la base de datos desde variables de entorno
func GetDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "3306"),
		User:     getEnv("DB_USER", "root"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "caricaloader"),
		Charset:  getEnv("DB_CHARSET", "utf8mb4"),
	}
}

// GetConnectionString construye la cadena de conexión MySQL
func (c *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=true&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database, c.Charset)
}

// getEnv obtiene una variable de entorno o retorna un valor por defecto
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
