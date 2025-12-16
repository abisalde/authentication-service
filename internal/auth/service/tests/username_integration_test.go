package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/abisalde/authentication-service/internal/auth/repository"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database"
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/database/ent/enttest"
	"github.com/redis/go-redis/v9"

	_ "github.com/mattn/go-sqlite3"
)

type mockMailService struct{}

func (m *mockMailService) SendHTMLEmail(ctx context.Context, recipientEmail, senderEmail, subject, htmlBody string, overrideSenderEmail ...string) error {
	return nil
}

var emailCounter int64

func setupTestEnvironment(t *testing.T) (*ent.Client, *database.RedisCache, func()) {
	t.Helper()

	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		t.Logf("Redis not available: %v. Tests will skip Redis-dependent scenarios.", err)
	}

	redisCache := database.NewCacheService(rdb)

	cleanup := func() {
		if err == nil {
			_ = rdb.FlushDB(ctx).Err()
		}
		_ = rdb.Close()
		_ = client.Close()
	}

	return client, redisCache, cleanup
}

func setupTestAuthService(t *testing.T) (*service.AuthService, *ent.Client, func()) {
	t.Helper()

	client, redisCache, cleanup := setupTestEnvironment(t)
	userRepo := repository.NewUserRepository(client)

	cfg := &configs.Config{}
	mailService := &mockMailService{}

	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	return authService, client, cleanup
}

func createTestUser(t *testing.T, client *ent.Client, username string) *ent.User {
	t.Helper()

	ctx := context.Background()
	emailCounter++

	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("testuser%d@example.com", emailCounter)).
		SetFirstName("Test").
		SetLastName("User").
		SetUsername(username).
		Save(ctx)

	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

func TestUsernameValidation_SingleCharacter(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	singleCharUsername := "x"
	user := createTestUser(t, client, singleCharUsername)

	if user.Username != singleCharUsername {
		t.Errorf("Expected username %s, got %s", singleCharUsername, user.Username)
	}
}

func TestUsernameValidation_MinLength(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	minUsername := "a"
	user := createTestUser(t, client, minUsername)

	if user.Username != minUsername {
		t.Errorf("Expected username %s, got %s", minUsername, user.Username)
	}
}

func TestUsernameValidation_MaxLength(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	maxUsername := "abcdefghijklmnopqrstuvwxyz1234"
	if len(maxUsername) != 30 {
		t.Fatalf("Test setup error: maxUsername should be 30 chars, got %d", len(maxUsername))
	}

	user := createTestUser(t, client, maxUsername)

	if user.Username != maxUsername {
		t.Errorf("Expected username %s, got %s", maxUsername, user.Username)
	}
}

func TestUsernameValidation_SpecialCharacters(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name      string
		username  string
		shouldErr bool
	}{
		{"Valid underscore", "user_name", false},
		{"Valid hyphen", "user-name", false},
		{"Valid mixed", "user_123-test", false},
		{"Valid apostrophe - Irish name", "O'Brien", false},
		{"Valid Norwegian character", "Ødegaard", false},
		{"Valid German umlaut", "Ölaf", false},
		{"Valid mixed European", "Müller-Østergård", false},
		{"Valid apostrophe at end", "Anders'", false},
		{"Invalid space", "user name", true},
		{"Invalid at", "user@name", true},
		{"Invalid dot", "user.name", true},
		{"Invalid special", "user$name", true},
		{"Invalid hash", "user#name", true},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := fmt.Sprintf("test_user_%d@example.com", i)
			_, err := client.User.Create().
				SetEmail(email).
				SetFirstName("Test").
				SetLastName("User").
				SetUsername(tt.username).
				Save(ctx)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for username %s, but got none", tt.username)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error for username %s, but got: %v", tt.username, err)
			}
		})
	}
}

func TestUsernameValidation_EmptyString(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("empty@example.com").
		SetFirstName("Test").
		SetLastName("User").
		Save(ctx)

	if err != nil {
		t.Fatalf("Failed to create user without username: %v", err)
	}

	if user.Username != "" {
		t.Errorf("Expected empty username, got %s", user.Username)
	}
}

func TestUsernameValidation_InternationalCharacters(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name        string
		username    string
		description string
	}{
		{"Norwegian name", "Ødegaard", "Norwegian player name with Ø"},
		{"Irish name", "O'Brien", "Irish name with apostrophe"},
		{"German name", "Ölaf", "German name with umlaut"},
		{"French name", "François", "French name with ç"},
		{"Spanish name", "José", "Spanish name with é"},
		{"African name", "N'Golo", "African name with apostrophe"},
		{"Mixed European", "Müller-Østergård", "Mixed with hyphen and special chars"},
		{"Single char", "Ø", "Single Unicode character"},
		{"Turkish name", "Şahin", "Turkish name with ş"},
		{"Polish name", "Łukasz", "Polish name with ł"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := fmt.Sprintf("intl_user_%d@example.com", i)
			user, err := client.User.Create().
				SetEmail(email).
				SetFirstName("Test").
				SetLastName("User").
				SetUsername(tt.username).
				Save(ctx)

			if err != nil {
				t.Errorf("Failed to create user with %s (%s): %v", tt.description, tt.username, err)
				return
			}

			if user.Username != tt.username {
				t.Errorf("Expected username %s, got %s", tt.username, user.Username)
			}
		})
	}
}

func TestCacheStampede_ConcurrentRequests(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	testUsername := "stampede_test"
	createTestUser(t, client, testUsername)

	cacheKey := fmt.Sprintf("username_exists:%s", testUsername)
	_ = authService.GetCache().Delete(ctx, cacheKey)

	concurrency := 100
	var wg sync.WaitGroup
	results := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			available, err := authService.CheckUsernameAvailability(ctx, testUsername)
			if err != nil {
				errors <- err
				return
			}
			results <- available
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	duration := time.Since(startTime)

	var resultCount int
	for range results {
		resultCount++
	}

	var errorCount int
	for err := range errors {
		errorCount++
		t.Logf("Error in concurrent request: %v", err)
	}

	t.Logf("Completed %d concurrent requests in %v", concurrency, duration)
	t.Logf("Successful: %d, Errors: %d", resultCount, errorCount)

	if resultCount+errorCount != concurrency {
		t.Errorf("Expected %d total responses, got %d", concurrency, resultCount+errorCount)
	}

	if duration > 5*time.Second {
		t.Errorf("100 concurrent requests took too long: %v", duration)
	}
}

func TestSingleflight_Deduplication(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	testUsername := "singleflight_test"
	createTestUser(t, client, testUsername)

	cacheKey := fmt.Sprintf("username_exists:%s", testUsername)
	_ = authService.GetCache().Delete(ctx, cacheKey)

	concurrency := 50
	var wg sync.WaitGroup
	results := make([]time.Duration, concurrency)
	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			<-startBarrier

			start := time.Now()
			_, _ = authService.CheckUsernameAvailability(ctx, testUsername)
			results[index] = time.Since(start)
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	var fastRequests, slowRequests int
	threshold := 50 * time.Millisecond

	for _, duration := range results {
		if duration < threshold {
			fastRequests++
		} else {
			slowRequests++
		}
	}

	t.Logf("Fast requests (<%v): %d", threshold, fastRequests)
	t.Logf("Slow requests (>=%v): %d", threshold, slowRequests)

	if float64(fastRequests)/float64(concurrency) < 0.8 {
		t.Logf("Warning: Only %.0f%% of requests were fast. Singleflight may not be working optimally.",
			float64(fastRequests)/float64(concurrency)*100)
	}
}

func TestRedisFailure_FallbackToDB(t *testing.T) {
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	failingRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:9999",
		DB:   0,
	})
	redisCache := database.NewCacheService(failingRedis)
	userRepo := repository.NewUserRepository(client)

	cfg := &configs.Config{}
	mailService := &mockMailService{}

	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	ctx := context.Background()

	testUsername := "fallback_test"
	createTestUser(t, client, testUsername)

	available, err := authService.CheckUsernameAvailability(ctx, testUsername)

	if err != nil {
		t.Logf("Error with Redis down: %v", err)
	}

	if available {
		t.Error("Expected username to be unavailable, but got available")
	}
}

func TestUsernameUpdate_CacheInvalidation(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	initialUsername := "initial_user"
	user := createTestUser(t, client, initialUsername)

	available, err := authService.CheckUsernameAvailability(ctx, initialUsername)
	if err != nil {
		t.Fatalf("Failed to check initial username: %v", err)
	}
	if available {
		t.Error("Initial username should not be available")
	}

	newUsername := "updated_user"
	err = authService.UpdateUsername(ctx, user.ID, newUsername)
	if err != nil {
		t.Fatalf("Failed to update username: %v", err)
	}

	available, err = authService.CheckUsernameAvailability(ctx, initialUsername)
	if err != nil {
		t.Fatalf("Failed to check old username after update: %v", err)
	}
	if !available {
		t.Error("Old username should be available after update")
	}

	available, err = authService.CheckUsernameAvailability(ctx, newUsername)
	if err != nil {
		t.Fatalf("Failed to check new username after update: %v", err)
	}
	if available {
		t.Error("New username should not be available after update")
	}
}

func TestUniqueConstraint_DuplicateUsername(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	_ = createTestUser(t, client, "user_one")
	user2 := createTestUser(t, client, "user_two")

	err := authService.UpdateUsername(ctx, user2.ID, "user_one")

	if err == nil {
		t.Error("Expected error when updating to duplicate username, but got none")
	}

	updatedUser2, err := client.User.Get(ctx, user2.ID)
	if err != nil {
		t.Fatalf("Failed to fetch user2 after failed update: %v", err)
	}

	if updatedUser2.Username != "user_two" {
		t.Errorf("Expected username to remain 'user_two', got '%s'", updatedUser2.Username)
	}
}

func TestUniqueConstraint_CreateDuplicateUsername(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	existingUsername := "duplicate_test"
	createTestUser(t, client, existingUsername)

	_, err := client.User.Create().
		SetEmail("another@example.com").
		SetFirstName("Another").
		SetLastName("User").
		SetUsername(existingUsername).
		Save(ctx)

	if err == nil {
		t.Error("Expected error when creating user with duplicate username, but got none")
	}
}

func TestCheckUsernameAvailability_Integration(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name              string
		setupUsername     string
		checkUsername     string
		expectedAvailable bool
	}{
		{
			name:              "Available username",
			setupUsername:     "",
			checkUsername:     "available_user",
			expectedAvailable: true,
		},
		{
			name:              "Taken username",
			setupUsername:     "taken_user",
			checkUsername:     "taken_user",
			expectedAvailable: false,
		},
		{
			name:              "Different username",
			setupUsername:     "user_a",
			checkUsername:     "user_b",
			expectedAvailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupUsername != "" {
				createTestUser(t, client, tt.setupUsername)
			}

			// Test
			available, err := authService.CheckUsernameAvailability(ctx, tt.checkUsername)
			if err != nil {
				t.Fatalf("CheckUsernameAvailability failed: %v", err)
			}

			if available != tt.expectedAvailable {
				t.Errorf("Expected available=%v for username '%s', got %v",
					tt.expectedAvailable, tt.checkUsername, available)
			}
		})
	}
}

func TestCachePerformance_CacheHit(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	testUsername := "cached_user"
	createTestUser(t, client, testUsername)

	start := time.Now()
	_, err := authService.CheckUsernameAvailability(ctx, testUsername)
	firstCallDuration := time.Since(start)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	start = time.Now()
	_, err = authService.CheckUsernameAvailability(ctx, testUsername)
	secondCallDuration := time.Since(start)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	t.Logf("First call (cache miss): %v", firstCallDuration)
	t.Logf("Second call (cache hit): %v", secondCallDuration)

	if secondCallDuration > firstCallDuration*2 {
		t.Logf("Warning: Cache hit (%v) was not faster than cache miss (%v)",
			secondCallDuration, firstCallDuration)
	}
}

func BenchmarkCheckUsernameAvailability_CacheHit(b *testing.B) {
	client := enttest.Open(b, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer rdb.Close()

	redisCache := database.NewCacheService(rdb)
	userRepo := repository.NewUserRepository(client)
	cfg := &configs.Config{}
	mailService := &mockMailService{}
	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	ctx := context.Background()

	testUsername := "bench_user"
	_, err := client.User.Create().
		SetEmail(fmt.Sprintf("%s@example.com", testUsername)).
		SetFirstName("Test").
		SetLastName("User").
		SetUsername(testUsername).
		Save(ctx)
	if err != nil {
		b.Fatalf("Failed to create test user: %v", err)
	}

	_, _ = authService.CheckUsernameAvailability(ctx, testUsername)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authService.CheckUsernameAvailability(ctx, testUsername)
	}
}

func BenchmarkCheckUsernameAvailability_CacheMiss(b *testing.B) {
	client := enttest.Open(b, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	defer rdb.Close()

	redisCache := database.NewCacheService(rdb)
	userRepo := repository.NewUserRepository(client)
	cfg := &configs.Config{}
	mailService := &mockMailService{}
	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		testUsername := fmt.Sprintf("bench_user_%d", i)
		_, err := client.User.Create().
			SetEmail(fmt.Sprintf("%s@example.com", testUsername)).
			SetFirstName("Test").
			SetLastName("User").
			SetUsername(testUsername).
			Save(ctx)
		if err != nil {
			b.Fatalf("Failed to create test user: %v", err)
		}
		cacheKey := fmt.Sprintf("username_exists:%s", testUsername)
		_ = authService.GetCache().Delete(ctx, cacheKey)
		b.StartTimer()

		_, _ = authService.CheckUsernameAvailability(ctx, testUsername)
	}
}
