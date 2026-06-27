package auth

import "net/http"

// ApplyV2Headers sets V2/V3 request auth and metadata headers.
func ApplyV2Headers(req *http.Request, creds Credentials, resourceID string) {
	headers := BuildV2WSHeaders(creds, resourceID, "")
	for key, values := range headers {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
}

// BuildV2WSHeaders builds V2/V3 WebSocket auth and metadata headers.
func BuildV2WSHeaders(creds Credentials, resourceID, connectID string) http.Header {
	headers := http.Header{}

	if creds.AppID != "" {
		headers.Set("X-Api-App-Id", creds.AppID)
	}
	if creds.APIKey != "" {
		headers.Set("X-Api-Key", creds.APIKey)
	}

	resolvedResourceID := resourceID
	if resolvedResourceID == "" {
		resolvedResourceID = creds.DefaultResourceID
	}
	if resolvedResourceID != "" {
		headers.Set("X-Api-Resource-Id", resolvedResourceID)
	}
	if connectID != "" {
		headers.Set("X-Api-Connect-Id", connectID)
	}

	return headers
}
