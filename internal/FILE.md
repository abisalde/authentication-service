# The Code File Tree

- This file provides an overview of the code structure for the authentication service.
- It includes the main directories, their purposes, and key files.
- The service is designed to handle user authentication, including email/password and OAuth methods.
- It also includes GraphQL handlers, a database layer, and utility functions.

### What You're Getting

- **authentication-service**: The main directory containing the authentication service code

```bash
authentication-service/
    ├── cmd
    │   └── server                      # Application entry point ✅
    │       └── main.go                 # Initializes and runs server
    ├── deployments                     # Deployment configurations ✅
    │   ├── docker-compose.yml
    │   └── nginx.conf
    ├── Dockerfile                      # Multistage Dockerfile for building and running the service ✅
    ├── .dockerignore                   # Files to ignore in Docker builds ✅
    ├── .env                            # Environment variables for local development ✅
    ├── go.mod
    ├── internal
    │   ├── auth                        # Authentication logic ✅
    │   │   ├── handler                 # Transport handlers
    │   │   │   ├── http                # GraphQL handlers for auth
    │   │   │   └── oauth               # OAuth handlers
    │   │   ├── repository              # Data access layer
    │   │   │   ├── cache               # Cache repository for user sessions, tokens, etc.
    │   │   │   ├── mysql
    │   │   │   └── user.go
    │   │   └── service
    │   │       ├── auth.go              # Auth service interface
    │   │       ├── email.go             # Email/Password auth
    │   │       └── oauth.go             # OAuth service interface (Facebook, Google, etc.)
    │   ├── configs                      # Configuration files ✅
    │   │   ├── config.go
    │   │   ├── dev.yml
    │   │   └── prod.yml
    │   ├── database
    │   │   ├── database.go
    │   │   └── ent
    │   ├── graph                          # GraphQL Implementation ✅
    │   │   ├── generated
    │   │   ├── model
    │   │   ├── resolver.go
    │   │   └── schema                     # .graphql schema files
    │   ├── middleware
    │   ├── resolver
    │   │   └── user_resolver.go
    │   └── utils                          # Utility functions and helpers ✅
    ├── LICENSE
    ├── migrations
    │   ├── 0001_init.down.sql
    │   └── 0001_init.up.sql
    ├── pkg
    │   ├── jwt                             # JWT utilities ✅
    │   ├── logger                          # Logging utilities
    │   ├── oauth
    │   ├── password
    │   └── validator
    ├── README.md
    ├── scripts                             # Helper scripts ✅
    │   ├── migrate.sh                      # Script to run database migrations ✅
    │   ├── run-service.sh                  # Script to run the service locally with Docker (e.g., `run-service.sh -d` to run docker)
    │   ├── reset-cluster.sh                # Script to reset the Docker cluster
    │   └── setup.sh                        # Script to set up the development environment
    └── tools.go

# Code File Tree for the Authentication Service
```
