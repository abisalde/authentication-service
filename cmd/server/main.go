package main

import (
	"log"

	server "github.com/abisalde/authentication-service/cmd"
	"github.com/abisalde/authentication-service/internal/utils"
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

	gqlSrv, auth, oauth := server.SetupGraphQLServer(db, redisClient, appCfgLoader)

	authService := server.SetupFiberApp(db, gqlSrv, auth, oauth)

	portHost := utils.GetListenAddress(appCfg)

	log.Printf("🐳 Hello, Authentication MicroService from Docker <3 🚀::: 🔐 at http://localhost:%s", appCfg.HTTPPort)
	log.Printf("〒 App Current Environment %s ㉿:", appCfg.AppEnv)
	log.Printf("☞ ☞ %s", portHost)
	log.Fatal(authService.Listen(portHost))
}
