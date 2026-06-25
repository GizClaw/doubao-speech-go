package auth

import "net/http"

// Credentials is the minimal credential set for authentication.
type Credentials struct {
	AppID             string
	APIKey            string
	DefaultResourceID string
}

// ApplyV1Headers sets V1 request authentication headers.
func ApplyV1Headers(req *http.Request, creds Credentials) {
	if creds.APIKey != "" {
		req.Header.Set("X-Api-Key", creds.APIKey)
	}
	if creds.AppID != "" {
		req.Header.Set("X-Api-App-Id", creds.AppID)
	}
}
