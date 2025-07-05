package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database"
	"github.com/abisalde/authentication-service/internal/graph"
	"github.com/abisalde/authentication-service/internal/graph/resolvers"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

const defaultPort = "1010"

func main() {

	cfg, err := configs.Load(os.Getenv("APP_ENV"))
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = defaultPort
	}

	log.Printf("This is the APP_ENV:%v", cfg.DB.Port)
	log.Printf("This is the PORT:%v", httpPort)

	db, err := database.Connect(cfg)

	if err != nil {
		log.Fatalf("🧲 Failed to connect to database: %v", err)
	}

	defer db.Close()

	if err := db.HealthCheck(context.Background()); err != nil {
		log.Fatalf("Database health check failed: %v", err)
	}

	authRepo := resolvers.NewResolver(db.Client)

	auth_service := fiber.New(fiber.Config{
		AppName:                 "Authentication Service",
		ProxyHeader:             fiber.HeaderXForwardedFor,
		CaseSensitive:           true,
		EnableTrustedProxyCheck: true,
	})

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: authRepo}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		log.Println("GraphQL operation received")
		return next(ctx)
	})

	auth_service.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/health",
	}))

	auth_service.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))
	auth_service.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:8080",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	auth_service.All("/graphql", func(c *fiber.Ctx) error {
		clientIP := c.IP()
		remoteAddr := c.Context().RemoteAddr().String()
		log.Printf("GraphQL request from IP: %s, RemoteAddr: %s", clientIP, remoteAddr)
		return adaptor.HTTPHandler(srv)(c)
	})

	auth_service.Get("/", adaptor.HTTPHandlerFunc(
		playground.ApolloSandboxHandler("Authentication Service Playground", "/graphql"),
	))

	log.Printf("Hello, Authentication MicroService from Docker <3; 🚀 at http://localhost:%p", httpPort)
	log.Fatal(http.ListenAndServe(":"+httpPort, nil))
}
