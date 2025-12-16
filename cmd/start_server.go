package server

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/abisalde/authentication-service/internal/auth/handler/oauth"
	"github.com/abisalde/authentication-service/internal/auth/repository"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database"
	"github.com/abisalde/authentication-service/internal/graph"
	"github.com/abisalde/authentication-service/internal/graph/directives"
	"github.com/abisalde/authentication-service/internal/graph/resolvers"
	"github.com/abisalde/authentication-service/internal/handlers"
	"github.com/abisalde/authentication-service/internal/middleware"
	"github.com/abisalde/authentication-service/internal/worker"
	"github.com/abisalde/authentication-service/pkg/mail"
	"github.com/joho/godotenv"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

const defaultPort = "8080"

type AppConfig struct {
	HTTPPort string
	AppEnv   string
}

func InitConfig() (*configs.Config, *AppConfig, error) {

	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg, err := configs.Load(os.Getenv("APP_ENV"))
	if err != nil {
		return nil, nil, err
	}

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = defaultPort
	}

	appConfig := &AppConfig{
		HTTPPort: httpPort,
		AppEnv:   os.Getenv("APP_ENV"),
	}

	mail.NewMailerService(cfg)

	return cfg, appConfig, nil
}

func SetupDatabase(cfg *configs.Config) (*database.Database, *database.RedisCache, error) {
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, nil, err
	}
	ctx := context.Background()

	if err := db.HealthCheck(ctx); err != nil {
		db.Close()
		return nil, nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	redisCache, redisErr := database.InitRedis(ctxWithTimeout, cfg)
	if redisErr != nil {
		db.Close()
		return nil, nil, redisErr
	}

	return db, redisCache, nil
}

func SetupGraphQLServer(db *database.Database, redisClient *database.RedisCache, cfg *configs.Config) (server *handler.Server, authResult *service.AuthService, oauth *service.OAuthService) {

	mailerService := mail.NewMailerService(cfg)
	cacheService := database.NewCacheService(redisClient.RawClient())
	userRepo := repository.NewUserRepository(db.Client)

	authService := service.NewAuthService(
		userRepo,
		cfg,
		cacheService,
		mailerService,
	)

	oauthService := service.NewOAuthService(authService)

	worker := worker.NewLastLoginWorker(redisClient.RawClient(), authService)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	go worker.Start(consumerCtx)
	defer consumerCancel()

	resolver := resolvers.NewResolver(db.Client, authService, oauthService)
	auth := directives.NewAuthDirective()
	rateLimit := directives.NewRateLimitDirective(redisClient)
	constraint := directives.NewConstraint()
	defaultDirective := directives.NewDefaultDirective()

	srv := handler.New(graph.NewExecutableSchema(graph.Config{
		Resolvers: resolver,
		Directives: graph.DirectiveRoot{
			Auth:       auth.Auth,
			RateLimit:  rateLimit.RateLimit,
			Constraint: constraint.Constraints,
			Default:    defaultDirective.Default,
		},
	}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.SetErrorPresenter(middleware.ErrorPresenter)

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		op := graphql.GetOperationContext(ctx)
		log.Println("Operation Name:::::", op.OperationName)
		log.Println("GraphQL operation received")
		return next(ctx)
	})

	return srv, authService, oauthService
}

func SetupFiberApp(db *database.Database, gqlSrv *handler.Server, auth *service.AuthService, oauthService *service.OAuthService) *fiber.App {
	env := os.Getenv("APP_ENV")
	trustedDockerNetworkCIDR := "172.18.0.0/16"

	authService := fiber.New(fiber.Config{
		AppName:                 "Authentication Service",
		ProxyHeader:             fiber.HeaderXForwardedFor,
		CaseSensitive:           true,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{trustedDockerNetworkCIDR},
	})

	authService.Use(func(c *fiber.Ctx) error {
		if c.UserContext() == nil {
			c.SetUserContext(context.Background())
		}
		return c.Next()
	})

	authService.Use(healthcheck.New(healthcheck.Config{
		LivenessProbe: func(c *fiber.Ctx) bool {
			return true
		},
		LivenessEndpoint: "/health",
	}))

	authService.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))

	authService.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:8080,http://localhost:3000",
		AllowMethods:     "GET,POST,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))

	oauthHandler := oauth.NewOAuthHandler(oauthService)
	oauthHandler.RegisterRoutes(authService)

	authService.Get("/health", func(c *fiber.Ctx) error {
		if err := db.HealthCheck(context.Background()); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).SendString("UNHEALTHY")
		}
		return c.SendString("OK")
	})

	authService.Use(adaptor.HTTPMiddleware(middleware.AuthMiddleware(db.Client, auth)))
	authService.Use(middleware.FiberWebMiddleware)

	authService.All("/graphql", handlers.GraphQLHandler(gqlSrv))

	if env == "production" {
		authService.All("/", func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "backend NotFound",
				"message": "service rules for the path non-existent",
			})
		})
	} else {
		authService.Get("/", adaptor.HTTPHandlerFunc(
			playground.ApolloSandboxHandler("Authentication Service Playground", "/graphql"),
		))
		authService.Get("/dashboard", adaptor.HTTPHandlerFunc(
			playground.ApolloSandboxHandler("Authentication Service Playground", "/graphql"),
		))
	}

	return authService
}
