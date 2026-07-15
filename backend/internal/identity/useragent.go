package identity

import "strings"

// Device-type buckets returned by parseUserAgent (mapped to an icon on the UI).
const (
	deviceDesktop = "desktop"
	deviceMobile  = "mobile"
	deviceTablet  = "tablet"
	deviceUnknown = "unknown"
)

// parseUserAgent extracts a friendly browser, OS, and device-type from a raw
// User-Agent header via lightweight substring matching — enough for a
// human-readable "Chrome · macOS" session label without pulling in a UA
// database. Order matters: more specific tokens are checked before the generic
// ones they embed (Edge/Opera before Chrome; iPad tablets before iPhones;
// Android tablets before Android phones). An empty or unrecognizable UA
// degrades gracefully rather than crashing.
func parseUserAgent(ua string) (browser, os, deviceType string) {
	if strings.TrimSpace(ua) == "" {
		return "", "", deviceUnknown
	}
	return detectBrowser(ua), detectOS(ua), detectDevice(ua)
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "Edg"): // Edg/, EdgA/, EdgiOS/
		return "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "SamsungBrowser"):
		return "Samsung Internet"
	case strings.Contains(ua, "Firefox"), strings.Contains(ua, "FxiOS"):
		return "Firefox"
	case strings.Contains(ua, "Chrome"), strings.Contains(ua, "CriOS"):
		return "Chrome"
	case strings.Contains(ua, "Safari"):
		return "Safari"
	default:
		return ""
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows"):
		return "Windows"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		return "iOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Linux"), strings.Contains(ua, "X11"):
		return "Linux"
	default:
		return ""
	}
}

func detectDevice(ua string) string {
	switch {
	case strings.Contains(ua, "iPad"), strings.Contains(ua, "Tablet"),
		(strings.Contains(ua, "Android") && !strings.Contains(ua, "Mobile")):
		return deviceTablet
	case strings.Contains(ua, "Mobile"), strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPod"):
		return deviceMobile
	default:
		return deviceDesktop
	}
}
