# Username Feature Integration Tests

This directory contains comprehensive integration tests for the username availability feature implemented in PR #54.

## Overview

These tests validate the following edge cases and scenarios:

1. **Username Validation Edge Cases**

   - Single character usernames (like Twitter "x")
   - Minimum length (1 character)
   - Maximum length (30 characters)
   - Special characters (valid: Unicode letters, numbers, underscore, hyphen, apostrophe)
   - International characters (European: Ødegaard, Ölaf; Irish: O'Brien; African: N'Golo)
   - Invalid characters (spaces, @, ., $, #, etc.)
   - Empty string handling

2. **Cache Stampede Prevention**

   - 100+ concurrent requests for the same username
   - Singleflight deduplication verification
   - Performance under load

3. **Redis Connection Failures**

   - Fallback to database when Redis is unavailable
   - Graceful degradation

4. **Username Updates with Cache Invalidation**

   - Old username cache key removal
   - New username cache key creation
   - Database update verification

5. **Unique Constraint Violations**
   - Duplicate username creation attempts
   - Username update to existing username

## Running the Tests

### Prerequisites

The tests use SQLite for the database (no setup required) and optionally Redis for caching tests.

**Optional: Start Redis for full test coverage**

```bash
docker run -d -p 6379:6379 redis:latest
```

If Redis is not available, tests will run in fallback mode and validate the database-only behavior.

### Run All Tests

```bash
go test -v ./internal/auth/service/tests/ -timeout 60s
```

### Run Specific Tests

```bash
# Username validation tests
go test -v ./internal/auth/service/tests/ -run TestUsernameValidation

# Cache stampede tests
go test -v ./internal/auth/service/tests/ -run TestCacheStampede

# Redis failure tests
go test -v ./internal/auth/service/tests/ -run TestRedisFailure

# Unique constraint tests
go test -v ./internal/auth/service/tests/ -run TestUniqueConstraint

# Singleflight tests
go test -v ./internal/auth/service/tests/ -run TestSingleflight
```

### Run Benchmarks

```bash
go test -bench=. ./internal/auth/service/tests/ -benchmem
```

## Test Results

All tests are passing as of the latest commit:

```
✓ TestUsernameValidation_SingleCharacter
✓ TestUsernameValidation_MinLength
✓ TestUsernameValidation_MaxLength
✓ TestUsernameValidation_SpecialCharacters
✓ TestUsernameValidation_InternationalCharacters
✓ TestUsernameValidation_EmptyString
✓ TestCacheStampede_ConcurrentRequests
✓ TestSingleflight_Deduplication
✓ TestRedisFailure_FallbackToDB
✓ TestUsernameUpdate_CacheInvalidation
✓ TestUniqueConstraint_DuplicateUsername
✓ TestUniqueConstraint_CreateDuplicateUsername
✓ TestCheckUsernameAvailability_Integration
✓ TestCachePerformance_CacheHit
```

## Test Coverage

These tests cover:

- ✅ Username validation (length from 1-30 chars, format, special characters)
- ✅ Single character usernames (like Twitter)
- ✅ International character support (European, African, Asian names)
- ✅ Unicode letter support (Ø, Ö, ç, é, ş, ł, etc.)
- ✅ Apostrophes in names (O'Brien, N'Golo)
- ✅ Cache stampede with 100+ concurrent requests
- ✅ Singleflight deduplication
- ✅ Redis connection failures and fallback behavior
- ✅ Cache invalidation on username updates
- ✅ Unique constraint violations
- ✅ Integration with database and cache layers
- ✅ Performance benchmarks

## Architecture

The tests use:

- **SQLite** for in-memory database testing (fast, no setup)
- **Redis** for cache testing (optional, graceful fallback)
- **Ent** ORM for database operations
- **Table-driven tests** for comprehensive coverage
- **Concurrent testing** to validate race conditions and performance

## Performance Expectations

With Redis available:

- Cache hit: ~1ms response time
- Cache miss: ~5-10ms response time
- 100 concurrent requests: <5 seconds total

Without Redis (fallback):

- All requests: ~5-10ms response time
- Graceful degradation with no failures

## Continuous Integration

These tests are designed to run in CI environments with or without Redis:

- Tests pass without Redis (validates fallback behavior)
- Tests pass with Redis (validates full caching behavior)
- No external dependencies required for basic test execution

## Future Enhancements

Potential additional tests to consider:

- Rate limiting tests
- Reserved username validation
- Username history tracking
- Bloom filter integration tests
- Load testing with 10,000+ concurrent users
