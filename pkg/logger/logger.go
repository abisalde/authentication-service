package app_logger

import (
	"log"
)

func LogGraphQLRequest(clientIP, remoteAddr string) {
	log.Printf("GraphQL request from IP: %s, RemoteAddr: %s", clientIP, remoteAddr)
}

func LogGraphQLField(object, field string) {
	log.Printf("GraphQL: %s.%s called", object, field)
}
