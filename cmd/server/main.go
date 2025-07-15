package main

import (
	"log"

	server "github.com/abisalde/authentication-service/cmd"
)

func main() {

	appCfgLoader, appCfg, err := server.InitConfig()
	if err != nil {
		log.Fatalf("❌ Failed to initialize configuration: %v", err)
	}

	db, redisClient, err := server.SetupDatabase(appCfgLoader)
	if err != nil {
		log.Fatalf("❌ Failed to setup database: %v", err)
	}
	defer db.Close()

	gqlSrv, auth := server.SetupGraphQLServer(db, redisClient, appCfgLoader)

	authService := server.SetupFiberApp(db, gqlSrv, auth)

	log.Printf("🐳 Hello, Authentication MicroService from Docker <3 🚀::: 🔐 at http://localhost:%s", appCfg.HTTPPort)
	log.Fatal(authService.Listen(":" + appCfg.HTTPPort))
}
