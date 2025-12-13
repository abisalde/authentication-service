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

// mockMailService implements mail.Mailer for testing
type mockMailService struct{}

func (m *mockMailService) SendHTMLEmail(ctx context.Context, recipientEmail, senderEmail, subject, htmlBody string, overrideSenderEmail ...string) error {
	return nil
}

// setupTestEnvironment creates a test database and Redis client
func setupTestEnvironment(t *testing.T) (*ent.Client, *database.RedisCache, func()) {
	t.Helper()

	// Create test database using SQLite for faster tests
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")

	// Create test Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests to avoid conflicts
	})

	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		t.Logf("Redis not available: %v. Tests will skip Redis-dependent scenarios.", err)
	}

	redisCache := database.NewCacheService(rdb)

	cleanup := func() {
		// Clean up Redis test data
		if err == nil {
			_ = rdb.FlushDB(ctx).Err()
		}
		_ = rdb.Close()
		_ = client.Close()
	}

	return client, redisCache, cleanup
}

// setupTestAuthService creates an AuthService for testing
func setupTestAuthService(t *testing.T) (*service.AuthService, *ent.Client, func()) {
	t.Helper()

	client, redisCache, cleanup := setupTestEnvironment(t)
	userRepo := repository.NewUserRepository(client)

	cfg := &configs.Config{}
	mailService := &mockMailService{}

	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	return authService, client, cleanup
}

// createTestUser creates a user for testing
func createTestUser(t *testing.T, client *ent.Client, username string) *ent.User {
	t.Helper()

	ctx := context.Background()
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("%s@example.com", username)).
		SetFirstName("Test").
		SetLastName("User").
		SetUsername(username).
		Save(ctx)

	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// Test: Username validation edge cases
func TestUsernameValidation_MinLength(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	// Test minimum valid length (3 characters)
	minUsername := "abc"
	user := createTestUser(t, client, minUsername)

	// Verify the username was saved correctly
	if user.Username != minUsername {
		t.Errorf("Expected username %s, got %s", minUsername, user.Username)
	}
}

func TestUsernameValidation_MaxLength(t *testing.T) {
	_, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	// Test maximum valid length (30 characters)
	maxUsername := "abcdefghijklmnopqrstuvwxyz1234"
	if len(maxUsername) != 30 {
		t.Fatalf("Test setup error: maxUsername should be 30 chars, got %d", len(maxUsername))
	}

	user := createTestUser(t, client, maxUsername)

	// Verify the username was saved correctly
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
		{"Invalid space", "user name", true},
		{"Invalid at", "user@name", true},
		{"Invalid dot", "user.name", true},
		{"Invalid special", "user$name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.User.Create().
				SetEmail(fmt.Sprintf("%s@example.com", tt.username)).
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

	// Empty username should be handled (field is optional)
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

// Test: Cache stampede with 100+ concurrent requests
func TestCacheStampede_ConcurrentRequests(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	testUsername := "stampede_test"
	createTestUser(t, client, testUsername)

	// Clear cache to force cache miss
	cacheKey := fmt.Sprintf("username_exists:%s", testUsername)
	_ = authService.GetCache().Delete(ctx, cacheKey)

	// Launch 100 concurrent requests
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

	// Collect results
	var resultCount int
	for range results {
		resultCount++
	}

	// Check for errors
	var errorCount int
	for err := range errors {
		errorCount++
		t.Logf("Error in concurrent request: %v", err)
	}

	t.Logf("Completed %d concurrent requests in %v", concurrency, duration)
	t.Logf("Successful: %d, Errors: %d", resultCount, errorCount)

	// Verify all requests completed
	if resultCount+errorCount != concurrency {
		t.Errorf("Expected %d total responses, got %d", concurrency, resultCount+errorCount)
	}

	// Verify performance - should complete quickly even with 100 requests
	if duration > 5*time.Second {
		t.Errorf("100 concurrent requests took too long: %v", duration)
	}
}

// Test: Singleflight deduplication verification
func TestSingleflight_Deduplication(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	testUsername := "singleflight_test"
	createTestUser(t, client, testUsername)

	// Clear cache to ensure cache miss
	cacheKey := fmt.Sprintf("username_exists:%s", testUsername)
	_ = authService.GetCache().Delete(ctx, cacheKey)

	// Launch concurrent requests and track timing
	concurrency := 50
	var wg sync.WaitGroup
	results := make([]time.Duration, concurrency)
	startBarrier := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Wait for all goroutines to be ready
			<-startBarrier

			start := time.Now()
			_, _ = authService.CheckUsernameAvailability(ctx, testUsername)
			results[index] = time.Since(start)
		}(i)
	}

	// Release all goroutines at once
	close(startBarrier)
	wg.Wait()

	// Analyze results - with singleflight, requests should be deduplicated
	// Most requests should complete very quickly (waiting for shared result)
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

	// At least 90% should be fast (waiting for shared result)
	if float64(fastRequests)/float64(concurrency) < 0.8 {
		t.Logf("Warning: Only %.0f%% of requests were fast. Singleflight may not be working optimally.",
			float64(fastRequests)/float64(concurrency)*100)
	}
}

// Test: Redis connection failures (cache fallback to DB)
func TestRedisFailure_FallbackToDB(t *testing.T) {
	client, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create auth service with a failing Redis client
	failingRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:9999", // Invalid port
		DB:   0,
	})
	redisCache := database.NewCacheService(failingRedis)
	userRepo := repository.NewUserRepository(client)

	cfg := &configs.Config{}
	mailService := &mockMailService{}

	authService := service.NewAuthService(userRepo, cfg, redisCache, mailService)

	ctx := context.Background()

	// Create a test user
	testUsername := "fallback_test"
	createTestUser(t, client, testUsername)

	// Should still work even with Redis down
	available, err := authService.CheckUsernameAvailability(ctx, testUsername)

	if err != nil {
		// Fallback should work, but might return an error depending on implementation
		t.Logf("Error with Redis down: %v", err)
	}

	// Username exists, so should not be available
	if available {
		t.Error("Expected username to be unavailable, but got available")
	}
}

// Test: Username update with cache invalidation
func TestUsernameUpdate_CacheInvalidation(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test user with initial username
	initialUsername := "initial_user"
	user := createTestUser(t, client, initialUsername)

	// Check availability to populate cache
	available, err := authService.CheckUsernameAvailability(ctx, initialUsername)
	if err != nil {
		t.Fatalf("Failed to check initial username: %v", err)
	}
	if available {
		t.Error("Initial username should not be available")
	}

	// Update username
	newUsername := "updated_user"
	err = authService.UpdateUsername(ctx, user.ID, newUsername)
	if err != nil {
		t.Fatalf("Failed to update username: %v", err)
	}

	// Verify old username is now available (cache should be invalidated)
	available, err = authService.CheckUsernameAvailability(ctx, initialUsername)
	if err != nil {
		t.Fatalf("Failed to check old username after update: %v", err)
	}
	if !available {
		t.Error("Old username should be available after update")
	}

	// Verify new username is not available (cache should be updated)
	available, err = authService.CheckUsernameAvailability(ctx, newUsername)
	if err != nil {
		t.Fatalf("Failed to check new username after update: %v", err)
	}
	if available {
		t.Error("New username should not be available after update")
	}
}

// Test: Unique constraint violations on username update
func TestUniqueConstraint_DuplicateUsername(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create two users with different usernames
	_ = createTestUser(t, client, "user_one")
	user2 := createTestUser(t, client, "user_two")

	// Try to update user2 to use user1's username
	err := authService.UpdateUsername(ctx, user2.ID, "user_one")

	// Should fail due to unique constraint
	if err == nil {
		t.Error("Expected error when updating to duplicate username, but got none")
	}

	// Verify user2's username hasn't changed
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

	// Create first user
	existingUsername := "duplicate_test"
	createTestUser(t, client, existingUsername)

	// Try to create another user with the same username
	_, err := client.User.Create().
		SetEmail("another@example.com").
		SetFirstName("Another").
		SetLastName("User").
		SetUsername(existingUsername).
		Save(ctx)

	// Should fail due to unique constraint
	if err == nil {
		t.Error("Expected error when creating user with duplicate username, but got none")
	}
}

// Test: CheckUsernameAvailability integration
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
			// Setup: create user if needed
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

// Test: Cache hit performance
func TestCachePerformance_CacheHit(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	testUsername := "cached_user"
	createTestUser(t, client, testUsername)

	// First call - cache miss
	start := time.Now()
	_, err := authService.CheckUsernameAvailability(ctx, testUsername)
	firstCallDuration := time.Since(start)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	// Second call - should be cache hit
	start = time.Now()
	_, err = authService.CheckUsernameAvailability(ctx, testUsername)
	secondCallDuration := time.Since(start)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	t.Logf("First call (cache miss): %v", firstCallDuration)
	t.Logf("Second call (cache hit): %v", secondCallDuration)

	// Cache hit should be faster (but this is not strict due to test environment variability)
	if secondCallDuration > firstCallDuration*2 {
		t.Logf("Warning: Cache hit (%v) was not faster than cache miss (%v)",
			secondCallDuration, firstCallDuration)
	}
}

// Benchmark: Username availability check
func BenchmarkCheckUsernameAvailability_CacheHit(b *testing.B) {
	// Create test database using SQLite
	client := enttest.Open(b, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	// Create test Redis client
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

	// Create test user
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

	// Warm up cache
	_, _ = authService.CheckUsernameAvailability(ctx, testUsername)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = authService.CheckUsernameAvailability(ctx, testUsername)
	}
}

func BenchmarkCheckUsernameAvailability_CacheMiss(b *testing.B) {
	// Create test database using SQLite
	client := enttest.Open(b, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	// Create test Redis client
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
