package evacuator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"log/slog"
	"testing"
)

// mockTransport redirects requests to the test server
type mockTransport struct {
	serverURL string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect all requests to the test server
	req.URL.Scheme = "http"
	req.URL.Host = ""
	req.Host = ""

	// Create a new request to the test server
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, m.serverURL+req.URL.Path, req.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers
	for k, v := range req.Header {
		newReq.Header[k] = v
	}

	return http.DefaultClient.Do(newReq)
}

func TestGcpProvider_IsSpotTerminationDetected(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
		wantErr  bool
	}{
		{"termination detected", "TRUE", true, false},
		{"no termination", "FALSE", false, false},
		{"empty response", "", false, false},
		{"unexpected value", "MAYBE", false, false},
		{"whitespace response", "  ", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns the specified response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET request, got %s", r.Method)
				}
				if r.Header.Get("Metadata-Flavor") != "Google" {
					t.Errorf("expected Metadata-Flavor header to be 'Google', got %s", r.Header.Get("Metadata-Flavor"))
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create provider with mock transport that redirects to test server
			client := &http.Client{
				Transport: &mockTransport{serverURL: server.URL},
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			p := &GcpProvider{
				httpClient: client,
				logger:     logger,
			}

			// Call the method with a context that uses the test server
			ctx := context.Background()
			got, err := p.isSpotTerminationDetected(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("isSpotTerminationDetected() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isSpotTerminationDetected() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGcpProvider_IsSpotTerminationDetected_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := &GcpProvider{
		httpClient: client,
		logger:     logger,
	}

	ctx := context.Background()
	got, err := p.isSpotTerminationDetected(ctx)

	// Should return false and an error on HTTP error
	if err == nil {
		t.Error("expected error for non-OK status, got nil")
	}
	if got != false {
		t.Errorf("expected false on error, got %v", got)
	}
}

func TestGcpProvider_DoMetadataRequest(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    bool
	}{
		{"success", http.StatusOK, "test-response", false},
		{"not found", http.StatusNotFound, "", true},
		{"unauthorized", http.StatusUnauthorized, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET request, got %s", r.Method)
				}
				if r.Header.Get("Metadata-Flavor") != "Google" {
					t.Errorf("expected Metadata-Flavor header to be 'Google', got %s", r.Header.Get("Metadata-Flavor"))
				}
				w.WriteHeader(tt.statusCode)
				if tt.response != "" {
					w.Write([]byte(tt.response))
				}
			}))
			defer server.Close()

			client := &http.Client{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			p := &GcpProvider{
				httpClient: client,
				logger:     logger,
			}

			ctx := context.Background()
			got, err := p.doMetadataRequest(ctx, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("doMetadataRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.response {
				t.Errorf("doMetadataRequest() = %v, want %v", got, tt.response)
			}
		})
	}
}
