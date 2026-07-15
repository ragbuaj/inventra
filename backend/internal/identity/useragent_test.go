package identity

import "testing"

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name       string
		ua         string
		browser    string
		os         string
		deviceType string
	}{
		{
			name:       "chrome on macos",
			ua:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
			browser:    "Chrome",
			os:         "macOS",
			deviceType: deviceDesktop,
		},
		{
			name:       "edge on windows (Edg before Chrome)",
			ua:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36 Edg/120.0",
			browser:    "Edge",
			os:         "Windows",
			deviceType: deviceDesktop,
		},
		{
			name:       "safari on iphone is mobile",
			ua:         "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			browser:    "Safari",
			os:         "iOS",
			deviceType: deviceMobile,
		},
		{
			name:       "safari on ipad is tablet",
			ua:         "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/604.1",
			browser:    "Safari",
			os:         "iOS",
			deviceType: deviceTablet,
		},
		{
			name:       "chrome on android phone is mobile",
			ua:         "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Mobile Safari/537.36",
			browser:    "Chrome",
			os:         "Android",
			deviceType: deviceMobile,
		},
		{
			name:       "chrome on android tablet (no Mobile token) is tablet",
			ua:         "Mozilla/5.0 (Linux; Android 14; SM-X200) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
			browser:    "Chrome",
			os:         "Android",
			deviceType: deviceTablet,
		},
		{
			name:       "firefox on linux",
			ua:         "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			browser:    "Firefox",
			os:         "Linux",
			deviceType: deviceDesktop,
		},
		{
			name:       "samsung internet on android",
			ua:         "Mozilla/5.0 (Linux; Android 13; SAMSUNG SM-S911B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/115.0 Mobile Safari/537.36",
			browser:    "Samsung Internet",
			os:         "Android",
			deviceType: deviceMobile,
		},
		{
			name:       "empty ua degrades to unknown",
			ua:         "",
			browser:    "",
			os:         "",
			deviceType: deviceUnknown,
		},
		{
			name:       "unrecognized ua yields blanks but desktop default",
			ua:         "SomeBot/1.0",
			browser:    "",
			os:         "",
			deviceType: deviceDesktop,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser, os, deviceType := parseUserAgent(tt.ua)
			if browser != tt.browser {
				t.Errorf("browser: got %q want %q", browser, tt.browser)
			}
			if os != tt.os {
				t.Errorf("os: got %q want %q", os, tt.os)
			}
			if deviceType != tt.deviceType {
				t.Errorf("deviceType: got %q want %q", deviceType, tt.deviceType)
			}
		})
	}
}
