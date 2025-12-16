package session

import (
	"net"
	"net/http"
	"strings"
)

// DeviceInfo represents device information extracted from HTTP request
type DeviceInfo struct {
	Type      string // Desktop, Mobile, Tablet
	Name      string // Browser/App name
	IPAddress string
	UserAgent string
}

// ExtractDeviceInfo extracts device information from HTTP request
func ExtractDeviceInfo(r *http.Request) *DeviceInfo {
	userAgent := r.Header.Get("User-Agent")

	deviceType := "Desktop"
	deviceName := "Unknown"

	// Simple device detection (use a library like mssola/user_agent for production)
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		deviceType = "Mobile"
		if strings.Contains(ua, "android") {
			deviceName = "Android Device"
		} else if strings.Contains(ua, "iphone") {
			deviceName = "iPhone"
		}
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		deviceType = "Tablet"
		if strings.Contains(ua, "ipad") {
			deviceName = "iPad"
		}
	} else {
		// Desktop detection
		if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edge") {
			deviceName = "Chrome Browser"
		} else if strings.Contains(ua, "firefox") {
			deviceName = "Firefox Browser"
		} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
			deviceName = "Safari Browser"
		} else if strings.Contains(ua, "edge") {
			deviceName = "Edge Browser"
		}

		// Add OS info if available
		if strings.Contains(ua, "windows") {
			deviceName += " on Windows"
		} else if strings.Contains(ua, "mac") {
			deviceName += " on macOS"
		} else if strings.Contains(ua, "linux") {
			deviceName += " on Linux"
		}
	}

	// Get IP address
	ip := GetClientIP(r)

	return &DeviceInfo{
		Type:      deviceType,
		Name:      deviceName,
		IPAddress: ip,
		UserAgent: userAgent,
	}
}

// GetClientIP extracts the real client IP from the request
func GetClientIP(r *http.Request) string {
	// Try X-Forwarded-For header (set by proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}

// DeviceFingerprint represents a unique device identifier for enhanced security
type DeviceFingerprint struct {
	UserAgent      string
	IPAddress      string
	AcceptLanguage string
	// Add more fields for stronger fingerprinting in production
	// ScreenResolution string
	// TimeZone string
	// Platform string
}

// GenerateFingerprint creates a hash from device attributes
// This can be used to bind tokens to specific devices
func GenerateFingerprint(fp *DeviceFingerprint) string {
	// Simple implementation - in production, use more sophisticated method
	data := fp.UserAgent + ":" + fp.IPAddress + ":" + fp.AcceptLanguage
	return HashToken(data)
}

// ExtractFingerprint extracts device fingerprint from HTTP request
func ExtractFingerprint(r *http.Request) *DeviceFingerprint {
	return &DeviceFingerprint{
		UserAgent:      r.Header.Get("User-Agent"),
		IPAddress:      GetClientIP(r),
		AcceptLanguage: r.Header.Get("Accept-Language"),
	}
}
