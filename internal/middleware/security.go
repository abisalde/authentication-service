package middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	// Rate Limiting
	EnableRateLimit bool
	RateLimit       int           // Requests per window
	RateWindow      time.Duration // Time window for rate limiting
	
	// Exponential Backoff
	EnableExponentialBackoff bool
	BackoffMultiplier       float64       // Multiplier for each violation (e.g., 2.0)
	MaxBackoffDuration      time.Duration // Maximum backoff time
	BackoffResetTime        time.Duration // Time before backoff resets
	
	// IP Filtering
	EnableAllowList bool
	AllowedIPs      []string // Allowed IP addresses/CIDR ranges
	EnableDenyList  bool
	DeniedIPs       []string // Denied IP addresses/CIDR ranges
	
	// SSRF Protection
	EnableSSRFProtection bool
	BlockPrivateIPs      bool
	BlockedNetworks      []string // Additional blocked CIDR ranges
	
	// Network Segmentation
	TrustedProxies []string // Trusted proxy IPs/CIDR ranges
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		// Rate Limiting
		EnableRateLimit: true,
		RateLimit:       100,             // 100 requests
		RateWindow:      1 * time.Minute, // per minute
		
		// Exponential Backoff
		EnableExponentialBackoff: true,
		BackoffMultiplier:       2.0,
		MaxBackoffDuration:      1 * time.Hour,
		BackoffResetTime:        24 * time.Hour,
		
		// IP Filtering
		EnableAllowList: false, // Disabled by default
		AllowedIPs:      []string{},
		EnableDenyList:  true,
		DeniedIPs:       []string{}, // Can be populated dynamically
		
		// SSRF Protection
		EnableSSRFProtection: true,
		BlockPrivateIPs:      true,
		BlockedNetworks: []string{
			"10.0.0.0/8",      // Private IPv4
			"172.16.0.0/12",   // Private IPv4
			"192.168.0.0/16",  // Private IPv4
			"127.0.0.0/8",     // Loopback
			"169.254.0.0/16",  // Link-local
			"::1/128",         // IPv6 loopback
			"fc00::/7",        // IPv6 private
			"fe80::/10",       // IPv6 link-local
		},
		
		// Network Segmentation
		TrustedProxies: []string{
			"172.18.0.0/16", // Docker network
		},
	}
}

// SecurityMiddleware provides comprehensive security features
type SecurityMiddleware struct {
	config      SecurityConfig
	redisClient *redis.Client
	
	// IP filtering
	allowedNets []*net.IPNet
	deniedNets  []*net.IPNet
	blockedNets []*net.IPNet
	trustedNets []*net.IPNet
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// In-memory cache for performance
	ipCache map[string]*ipCacheEntry
}

type ipCacheEntry struct {
	allowed  bool
	denied   bool
	cachedAt time.Time
}

// NewSecurityMiddleware creates a new security middleware with the given config
func NewSecurityMiddleware(config SecurityConfig, redisClient *redis.Client) *SecurityMiddleware {
	sm := &SecurityMiddleware{
		config:      config,
		redisClient: redisClient,
		ipCache:     make(map[string]*ipCacheEntry),
	}
	
	// Parse IP ranges
	sm.allowedNets = parseNetworks(config.AllowedIPs)
	sm.deniedNets = parseNetworks(config.DeniedIPs)
	sm.blockedNets = parseNetworks(config.BlockedNetworks)
	sm.trustedNets = parseNetworks(config.TrustedProxies)
	
	// Start cache cleanup goroutine
	go sm.cleanupCache()
	
	return sm
}

// Handler returns a Fiber middleware handler
func (sm *SecurityMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		
		// Get client IP (handles proxies correctly)
		clientIP := sm.getClientIP(c)
		
		// Check IP filtering (allow/deny lists)
		if err := sm.checkIPFiltering(clientIP); err != nil {
			log.Printf("IP filtering blocked: %s - %v", clientIP, err)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Forbidden",
				"message": "Access denied",
			})
		}
		
		// Check SSRF protection
		if sm.config.EnableSSRFProtection {
			if err := sm.checkSSRFProtection(clientIP); err != nil {
				log.Printf("SSRF protection blocked: %s - %v", clientIP, err)
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error":   "Forbidden",
					"message": "Access denied",
				})
			}
		}
		
		// Check rate limiting with exponential backoff
		if sm.config.EnableRateLimit {
			backoffDuration, err := sm.checkRateLimit(ctx, clientIP)
			if err != nil {
				log.Printf("Rate limit exceeded: %s - backoff: %v", clientIP, backoffDuration)
				
				// Set retry-after header
				c.Set("Retry-After", fmt.Sprintf("%d", int(backoffDuration.Seconds())))
				c.Set("X-RateLimit-Reset", time.Now().Add(backoffDuration).Format(time.RFC3339))
				
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"error":        "Too Many Requests",
					"message":      "Rate limit exceeded",
					"retryAfter":   int(backoffDuration.Seconds()),
					"resetAt":      time.Now().Add(backoffDuration).Format(time.RFC3339),
				})
			}
		}
		
		// Add security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		
		return c.Next()
	}
}

// getClientIP extracts the real client IP, handling proxies correctly
func (sm *SecurityMiddleware) getClientIP(c *fiber.Ctx) string {
	// Check X-Forwarded-For header (for trusted proxies only)
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		// Get the first IP in the chain
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			// Verify the request came through a trusted proxy
			remoteIP := c.IP()
			if sm.isTrustedProxy(remoteIP) {
				return clientIP
			}
		}
	}
	
	// Check X-Real-IP header
	realIP := c.Get("X-Real-IP")
	if realIP != "" && sm.isTrustedProxy(c.IP()) {
		return realIP
	}
	
	// Fall back to direct connection IP
	return c.IP()
}

// checkIPFiltering checks allow and deny lists
func (sm *SecurityMiddleware) checkIPFiltering(ipStr string) error {
	// Check cache first
	if entry := sm.getCachedIP(ipStr); entry != nil {
		if entry.denied {
			return fmt.Errorf("IP is in deny list")
		}
		if sm.config.EnableAllowList && !entry.allowed {
			return fmt.Errorf("IP not in allow list")
		}
		return nil
	}
	
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address")
	}
	
	// Check deny list first (highest priority)
	if sm.config.EnableDenyList {
		for _, network := range sm.deniedNets {
			if network.Contains(ip) {
				sm.cacheIP(ipStr, false, true)
				return fmt.Errorf("IP is in deny list")
			}
		}
	}
	
	// Check allow list (if enabled)
	if sm.config.EnableAllowList {
		allowed := false
		for _, network := range sm.allowedNets {
			if network.Contains(ip) {
				allowed = true
				break
			}
		}
		if !allowed {
			sm.cacheIP(ipStr, false, false)
			return fmt.Errorf("IP not in allow list")
		}
		sm.cacheIP(ipStr, true, false)
	}
	
	return nil
}

// checkSSRFProtection prevents requests from private/internal networks
func (sm *SecurityMiddleware) checkSSRFProtection(ipStr string) error {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address")
	}
	
	// Check against blocked networks
	for _, network := range sm.blockedNets {
		if network.Contains(ip) {
			return fmt.Errorf("IP in blocked network range")
		}
	}
	
	// Block private IPs if configured
	if sm.config.BlockPrivateIPs {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("private/internal IP not allowed")
		}
	}
	
	return nil
}

// checkRateLimit implements rate limiting with exponential backoff
func (sm *SecurityMiddleware) checkRateLimit(ctx context.Context, ipStr string) (time.Duration, error) {
	now := time.Now()
	windowStart := now.Truncate(sm.config.RateWindow)
	
	// Keys for Redis
	countKey := fmt.Sprintf("ratelimit:count:%s:%d", ipStr, windowStart.Unix())
	violationKey := fmt.Sprintf("ratelimit:violations:%s", ipStr)
	backoffKey := fmt.Sprintf("ratelimit:backoff:%s", ipStr)
	
	// Check if currently in backoff period
	if sm.config.EnableExponentialBackoff {
		backoffUntil, err := sm.redisClient.Get(ctx, backoffKey).Int64()
		if err == nil && backoffUntil > now.Unix() {
			remaining := time.Unix(backoffUntil, 0).Sub(now)
			return remaining, fmt.Errorf("in backoff period")
		}
	}
	
	// Increment request counter
	pipe := sm.redisClient.TxPipeline()
	incrCmd := pipe.Incr(ctx, countKey)
	pipe.Expire(ctx, countKey, sm.config.RateWindow)
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("Redis error in rate limiter: %v", err)
		// Fail open on Redis errors
		return 0, nil
	}
	
	count := incrCmd.Val()
	
	// Check if rate limit exceeded
	if count > int64(sm.config.RateLimit) {
		// Record violation
		violations, _ := sm.redisClient.Incr(ctx, violationKey).Result()
		sm.redisClient.Expire(ctx, violationKey, sm.config.BackoffResetTime)
		
		// Apply exponential backoff
		if sm.config.EnableExponentialBackoff && violations > 0 {
			backoffDuration := sm.calculateBackoff(violations)
			backoffUntil := now.Add(backoffDuration)
			sm.redisClient.Set(ctx, backoffKey, backoffUntil.Unix(), backoffDuration)
			return backoffDuration, fmt.Errorf("rate limit exceeded")
		}
		
		return sm.config.RateWindow, fmt.Errorf("rate limit exceeded")
	}
	
	// Request allowed - reset violations on successful request after threshold
	if count == 1 {
		// First request in window - reset violations
		sm.redisClient.Del(ctx, violationKey)
	}
	
	return 0, nil
}

// calculateBackoff calculates exponential backoff duration
func (sm *SecurityMiddleware) calculateBackoff(violations int64) time.Duration {
	// Exponential: duration = base * (multiplier ^ violations)
	base := sm.config.RateWindow
	multiplier := sm.config.BackoffMultiplier
	
	// Calculate backoff
	backoff := float64(base) * pow(multiplier, float64(violations-1))
	duration := time.Duration(backoff)
	
	// Cap at max backoff duration
	if duration > sm.config.MaxBackoffDuration {
		duration = sm.config.MaxBackoffDuration
	}
	
	return duration
}

// pow calculates x^y (simple implementation)
func pow(x, y float64) float64 {
	if y == 0 {
		return 1
	}
	result := x
	for i := 1; i < int(y); i++ {
		result *= x
	}
	return result
}

// isTrustedProxy checks if an IP is a trusted proxy
func (sm *SecurityMiddleware) isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	
	for _, network := range sm.trustedNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Helper functions for IP caching
func (sm *SecurityMiddleware) getCachedIP(ipStr string) *ipCacheEntry {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	entry, exists := sm.ipCache[ipStr]
	if !exists {
		return nil
	}
	
	// Check if cache entry is still valid (5 minutes)
	if time.Since(entry.cachedAt) > 5*time.Minute {
		return nil
	}
	
	return entry
}

func (sm *SecurityMiddleware) cacheIP(ipStr string, allowed, denied bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.ipCache[ipStr] = &ipCacheEntry{
		allowed:  allowed,
		denied:   denied,
		cachedAt: time.Now(),
	}
}

func (sm *SecurityMiddleware) cleanupCache() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for ip, entry := range sm.ipCache {
			if now.Sub(entry.cachedAt) > 15*time.Minute {
				delete(sm.ipCache, ip)
			}
		}
		sm.mu.Unlock()
	}
}

// parseNetworks parses a list of IP addresses/CIDR ranges
func parseNetworks(cidrs []string) []*net.IPNet {
	var networks []*net.IPNet
	
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		
		// Check if it's a CIDR or just an IP
		if !strings.Contains(cidr, "/") {
			// Single IP - convert to /32 or /128
			ip := net.ParseIP(cidr)
			if ip != nil {
				if ip.To4() != nil {
					cidr += "/32"
				} else {
					cidr += "/128"
				}
			}
		}
		
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Printf("Invalid CIDR range: %s - %v", cidr, err)
			continue
		}
		networks = append(networks, network)
	}
	
	return networks
}

// AddToDenyList adds an IP or CIDR range to the deny list dynamically
func (sm *SecurityMiddleware) AddToDenyList(cidr string) error {
	networks := parseNetworks([]string{cidr})
	if len(networks) == 0 {
		return fmt.Errorf("invalid CIDR range: %s", cidr)
	}
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.deniedNets = append(sm.deniedNets, networks...)
	
	// Clear cache for affected IPs
	sm.ipCache = make(map[string]*ipCacheEntry)
	
	return nil
}

// RemoveFromDenyList removes an IP or CIDR range from the deny list
func (sm *SecurityMiddleware) RemoveFromDenyList(cidr string) error {
	networks := parseNetworks([]string{cidr})
	if len(networks) == 0 {
		return fmt.Errorf("invalid CIDR range: %s", cidr)
	}
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Remove matching networks
	var filtered []*net.IPNet
	for _, existing := range sm.deniedNets {
		match := false
		for _, toRemove := range networks {
			if existing.String() == toRemove.String() {
				match = true
				break
			}
		}
		if !match {
			filtered = append(filtered, existing)
		}
	}
	
	sm.deniedNets = filtered
	
	// Clear cache
	sm.ipCache = make(map[string]*ipCacheEntry)
	
	return nil
}
