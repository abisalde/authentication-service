package utils

import (
	"fmt"
	"log"
	"strconv"

	server "github.com/abisalde/authentication-service/cmd"
)

func getPort(c *server.AppConfig) int {
	port, err := strconv.Atoi(c.HTTPPort)
	if err != nil {
		log.Printf("⚠️ Invalid port '%s', defaulting to 8080. Error: %v", c.HTTPPort, err)
		return 8080
	}

	if port < 10 || port > 65535 {
		log.Printf("⚠️ Port %d out of range (10-65535), defaulting to 8080", port)
		return 8080
	}

	return port
}

func GetListenAddress(c *server.AppConfig) string {
	port := getPort(c)

	if c.AppEnv == "production" {
		return fmt.Sprintf("0.0.0.0:%d", port)
	}
	return fmt.Sprintf(":%d", port)
}
