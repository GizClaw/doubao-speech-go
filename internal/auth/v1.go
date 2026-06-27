package auth

import "net/http"

// Credentials carries API-key auth plus request metadata headers.
type Credentials struct {
	AppID             string
	APIKey            string
	DefaultResourceID string
}

// ApplyV1Headers sets V1 request auth and metadata headers.
func ApplyV1Headers(req *http.Request, creds Credentials) {
	if creds.APIKey != "" {
		req.Header.Set("X-Api-Key", creds.APIKey)
	}
	if creds.AppID != "" {
		req.Header.Set("X-Api-App-Id", creds.AppID)
	}
}
