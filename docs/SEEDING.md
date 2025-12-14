# Database Seeding

This directory contains scripts for seeding the database with mock data.

## Seeding Users

The `seed_users.go` script inserts 20 mock users into the database with diverse profiles.

### Mock Users Details

The seed includes:

- **18 regular users** with various authentication providers (EMAIL, GOOGLE, FACEBOOK)
- **2 admin users** for testing administrative features
- Mix of verified and unverified email addresses
- Diverse addresses from different US cities
- Various marketing opt-in preferences

### Default Credentials

For users with `EMAIL` provider:

- **Password:** `Password123!`

### Usage

#### Option 1: Using the Shell Script (Recommended)

```bash
# From the project root
./scripts/seed.sh
```

#### Option 2: Using Go Run Directly

```bash
# From the project root
go run ./scripts/seed_users.go
```

#### Option 3: Running from Docker

```bash
# First, exec into the auth-service container
docker exec -it authentication-service-auth-service-1 /bin/sh

# Then run the seed script
go run /app/scripts/seed_users.go
```

### Prerequisites

1. Docker containers must be running:

   ```bash
   cd deployments
   docker-compose up -d
   ```

2. Database migrations must be completed (this happens automatically when the service starts)

### Features

- ✅ Checks if users already exist (won't duplicate data)
- ✅ Creates users with proper password hashing
- ✅ Sets up OAuth users (Google and Facebook)
- ✅ Configures addresses for all users
- ✅ Sets email verification status
- ✅ Configures admin roles for testing
- ✅ Sets realistic timestamps (terms accepted, last login)

### Sample Users

| Email                      | Username      | Role  | Provider | Email Verified |
| -------------------------- | ------------- | ----- | -------- | -------------- |
| john.doe@example.com       | johndoe       | USER  | EMAIL    | ✅             |
| admin.user@example.com     | adminuser     | ADMIN | EMAIL    | ✅             |
| alice.williams@example.com | alicewilliams | USER  | GOOGLE   | ✅             |
| diana.jones@example.com    | dianajones    | USER  | FACEBOOK | ✅             |

_(See `seed_users.go` for the complete list)_

### Clearing Seeded Data

If you need to re-seed the database:

```bash
# Option 1: Use the reset cluster script
./scripts/reset-cluster.sh


### Troubleshooting

**Error: "Database already contains X users"**

- The seed script prevents duplicate data. Clear existing users first.

**Error: "Failed to connect to database"**

- Ensure Docker containers are running
- Check your config files in `internal/configs/`

**Error: "Failed to hash password"**

- Ensure the bcrypt package is properly installed
- Check Go version compatibility (requires Go 1.24.3+)
```
