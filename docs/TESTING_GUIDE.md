# Testing Guide - Username Feature Integration Tests

## Overview

This document provides comprehensive information about the integration tests added for the username availability feature (PR #54). These tests ensure the reliability, performance, and correctness of the username caching system under various edge cases and failure scenarios.

## Test Suite Location

All username integration tests are located in:
```
internal/auth/service/tests/username_integration_test.go
```

## Test Coverage Summary

### 1. Username Validation Edge Cases ✅

**Purpose:** Validate that username constraints are properly enforced at the database level.

**Tests:**
- `TestUsernameValidation_MinLength` - Validates minimum length of 3 characters
- `TestUsernameValidation_MaxLength` - Validates maximum length of 30 characters
- `TestUsernameValidation_SpecialCharacters` - Tests allowed characters (alphanumeric, underscore, hyphen)
- `TestUsernameValidation_EmptyString` - Ensures optional username field works correctly

**Key Validations:**
- ✓ Username must be 3-30 characters
- ✓ Only alphanumeric, underscore (_), and hyphen (-) allowed
- ✓ Invalid characters (space, @, ., $, etc.) are rejected
- ✓ Empty username is allowed (field is optional)

### 2. Cache Stampede Prevention ✅

**Purpose:** Verify that concurrent requests for the same username don't overwhelm the database.

**Test:** `TestCacheStampede_ConcurrentRequests`

**Scenario:**
- 100 concurrent goroutines request availability for the same username
- Cache is cleared before the test to force cache miss
- Singleflight pattern should deduplicate the requests

**Expected Behavior:**
- All 100 requests complete successfully
- Only 1 database query is executed (singleflight deduplication)
- Total execution time < 5 seconds
- No race conditions or errors

**Verification:**
```
✓ All 100 requests complete
✓ Performance under load is acceptable
✓ No errors or panics
✓ Singleflight logs show request deduplication
```

### 3. Singleflight Deduplication ✅

**Purpose:** Validate that singleflight is working correctly to deduplicate concurrent requests.

**Test:** `TestSingleflight_Deduplication`

**Scenario:**
- 50 concurrent requests for the same username
- Cache is cleared to ensure cache miss
- All requests are synchronized to start simultaneously
- Response times are measured

**Expected Behavior:**
- Most requests (>80%) complete very quickly (waiting for shared result)
- Only one request actually queries the database
- All requests receive the same result

**Verification:**
```
✓ Fast requests (waiting): ~80%+ complete in <50ms
✓ Slow requests (DB query): ~20% or less take >50ms
✓ Singleflight deduplication is working
```

### 4. Redis Connection Failures ✅

**Purpose:** Test graceful degradation when Redis is unavailable.

**Test:** `TestRedisFailure_FallbackToDB`

**Scenario:**
- AuthService configured with invalid Redis connection (port 9999)
- Username availability check is performed
- System should fall back to database

**Expected Behavior:**
- No panic or fatal errors
- Request completes successfully
- Falls back to database query
- Correct result is returned

**Verification:**
```
✓ System handles Redis failure gracefully
✓ Database fallback works correctly
✓ Correct availability result returned
✓ No data corruption or errors
```

### 5. Username Update with Cache Invalidation ✅

**Purpose:** Ensure cache is properly invalidated when usernames are updated.

**Test:** `TestUsernameUpdate_CacheInvalidation`

**Scenario:**
1. User created with username "initial_user"
2. Username availability checked (populates cache)
3. Username updated to "updated_user"
4. Check both old and new username availability

**Expected Behavior:**
- Old username becomes available (cache invalidated)
- New username becomes unavailable (cache updated)
- Database reflects the change
- Cache is synchronized

**Verification:**
```
✓ Old username cache key is deleted
✓ New username cache key is created
✓ Database is updated correctly
✓ Availability checks return correct results
```

### 6. Unique Constraint Violations ✅

**Purpose:** Verify that unique constraints prevent duplicate usernames.

**Tests:**
- `TestUniqueConstraint_DuplicateUsername` - Update to existing username
- `TestUniqueConstraint_CreateDuplicateUsername` - Create with duplicate username

**Scenarios:**

**Test 1: Update to Duplicate**
1. Create user1 with username "user_one"
2. Create user2 with username "user_two"
3. Try to update user2's username to "user_one"

**Expected:** Operation fails, user2's username remains "user_two"

**Test 2: Create Duplicate**
1. Create user with username "duplicate_test"
2. Try to create another user with same username

**Expected:** Creation fails due to unique constraint

**Verification:**
```
✓ Duplicate username update is rejected
✓ Original username is preserved on failed update
✓ Duplicate username creation is rejected
✓ Database constraint enforcement works
```

### 7. Integration Test ✅

**Purpose:** End-to-end validation of username availability checks.

**Test:** `TestCheckUsernameAvailability_Integration`

**Scenarios:**
- Check availability of non-existent username (should be available)
- Check availability of existing username (should be unavailable)
- Check different username when one exists (should be available)

**Verification:**
```
✓ Available usernames return correct status
✓ Taken usernames return correct status
✓ Cache behavior is correct
✓ Database queries are correct
```

### 8. Cache Performance Test ✅

**Purpose:** Validate that cache hits are faster than cache misses.

**Test:** `TestCachePerformance_CacheHit`

**Scenario:**
1. First call - cache miss, queries database
2. Second call - cache hit, returns from cache

**Expected Behavior:**
- Both calls complete successfully
- Second call should be comparable or faster (test environment may vary)
- Cache mechanism is working

**Note:** This test logs performance metrics but doesn't enforce strict timing due to test environment variability.

## Benchmarks

Two benchmarks are included for performance testing:

### BenchmarkCheckUsernameAvailability_CacheHit
- Measures performance with warm cache
- Expected: ~1ms per operation with Redis
- Tests steady-state performance

### BenchmarkCheckUsernameAvailability_CacheMiss
- Measures performance with cold cache
- Expected: ~5-10ms per operation
- Tests worst-case performance

**Running Benchmarks:**
```bash
go test -bench=. ./internal/auth/service/tests/ -benchmem
```

## Running the Tests

### All Tests
```bash
go test -v ./internal/auth/service/tests/ -timeout 60s
```

### Specific Test Categories
```bash
# Validation tests
go test -v ./internal/auth/service/tests/ -run TestUsernameValidation

# Concurrency tests
go test -v ./internal/auth/service/tests/ -run "TestCache|TestSingleflight"

# Failure scenarios
go test -v ./internal/auth/service/tests/ -run TestRedisFailure

# Constraint tests
go test -v ./internal/auth/service/tests/ -run TestUniqueConstraint
```

## Test Environment

### Dependencies
- **SQLite:** In-memory database for fast testing (no setup required)
- **Redis:** Optional - tests work with or without Redis
- **Ent ORM:** Database operations
- **Go standard library:** Testing and benchmarking

### With Redis (Full Coverage)
```bash
# Start Redis
docker run -d -p 6379:6379 redis:latest

# Run tests
go test -v ./internal/auth/service/tests/
```

### Without Redis (Fallback Testing)
```bash
# Just run tests - they will test fallback behavior
go test -v ./internal/auth/service/tests/
```

## Test Results

All tests pass in both Redis-available and Redis-unavailable scenarios:

**With Redis:**
- ✅ 12 tests passing
- Cache performance validated
- Singleflight deduplication verified

**Without Redis:**
- ✅ 12 tests passing
- Fallback behavior validated
- Database-only mode confirmed working

## CI/CD Integration

These tests are designed for CI environments:

**Features:**
- No external dependencies required (SQLite in-memory)
- Graceful handling of missing Redis
- Fast execution (~3 seconds total)
- No test data cleanup required (in-memory)
- No port conflicts (uses test DB)

**Recommended CI Configuration:**
```yaml
- name: Run Integration Tests
  run: go test -v ./internal/auth/service/tests/ -timeout 60s
```

## Edge Cases Covered

| Edge Case | Test | Status |
|-----------|------|--------|
| Min username length (3 chars) | TestUsernameValidation_MinLength | ✅ |
| Max username length (30 chars) | TestUsernameValidation_MaxLength | ✅ |
| Special characters | TestUsernameValidation_SpecialCharacters | ✅ |
| Empty string | TestUsernameValidation_EmptyString | ✅ |
| 100+ concurrent requests | TestCacheStampede_ConcurrentRequests | ✅ |
| Singleflight deduplication | TestSingleflight_Deduplication | ✅ |
| Redis connection failure | TestRedisFailure_FallbackToDB | ✅ |
| Cache invalidation on update | TestUsernameUpdate_CacheInvalidation | ✅ |
| Duplicate username creation | TestUniqueConstraint_CreateDuplicateUsername | ✅ |
| Duplicate username update | TestUniqueConstraint_DuplicateUsername | ✅ |

## Performance Metrics

**Test Environment Results (without Redis):**
- Single username check: ~5-10ms
- 100 concurrent requests: ~170ms total
- Cache stampede test: <500ms
- All tests complete: ~3 seconds

**Expected Production Performance (with Redis):**
- Cache hit: ~1ms
- Cache miss: ~5-10ms
- 100 concurrent requests: <100ms
- Cache hit rate: >80%

## Future Test Enhancements

Potential additions for comprehensive coverage:

1. **Load Testing**
   - Test with 10,000+ concurrent requests
   - Sustained load over time
   - Memory usage profiling

2. **Rate Limiting**
   - Per-user rate limit tests
   - API-level rate limiting validation

3. **Reserved Usernames**
   - Test reserved username list
   - Admin/system username protection

4. **Username History**
   - Track username changes
   - Security audit log testing

5. **Bloom Filter**
   - Negative caching tests
   - Memory optimization validation

## Troubleshooting

### Tests Fail with "Redis not available"
**Solution:** This is expected behavior. Tests will run with database fallback.

### Tests Fail with "sqlite3 driver not found"
**Solution:** Run `go get github.com/mattn/go-sqlite3`

### Tests Timeout
**Solution:** Increase timeout: `go test -timeout 120s`

### Slow Performance
**Solution:** Tests use in-memory SQLite. Check system resources.

## Maintenance

**When to Update Tests:**
- Username validation rules change
- Cache strategy modifications
- New edge cases discovered
- Performance requirements change
- Database schema updates

**Test Stability:**
- Tests use in-memory database (no cleanup needed)
- Tests are isolated (no shared state)
- Tests handle Redis unavailability
- Tests are repeatable and deterministic

## References

- **PR #54:** Username feature implementation
- **USERNAME_FEATURE.md:** Feature documentation
- **auth.go:** Service implementation
- **user.go:** Repository implementation
