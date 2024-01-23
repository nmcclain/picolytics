package picolytics

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestGetFile tests the getFile function
func TestGetFile(t *testing.T) {
	// Setup a temporary directory with a test file
	tempDir := t.TempDir()
	testFileName := "testfile.txt"
	testFilePath := filepath.Join(tempDir, testFileName)
	if err := os.WriteFile(testFilePath, []byte("test content"), 0666); err != nil {
		t.Fatalf("getFile unable to create tmp testfile: %v", err)
	}
	// Create an http.FileSystem using the temp directory
	fs := http.Dir(tempDir)

	// Test getFile with the test file
	file, err := getFile(fs, testFileName)
	if err != nil {
		t.Fatalf("getFile returned an error: %v", err)
	}
	if file == nil {
		t.Fatal("getFile returned a nil file")
	}
	file.Close()

	// Test getFile with a non-existent file
	_, err = getFile(fs, "nonexistent.txt")
	if err == nil {
		t.Fatal("Expected an error for non-existent file, got nil")
	}
}

func TestHandleStatic(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/testfile.txt", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("*")
	c.SetParamValues("testfile.txt")

	// Setup a temporary directory with a test file
	tempDir := t.TempDir()
	testFileName := "testfile.txt"
	testFilePath := filepath.Join(tempDir, testFileName)
	if err := os.WriteFile(testFilePath, []byte("test content"), 0666); err != nil {
		t.Fatalf("Unable to create tmp testfile: %v", err)
	}

	o11yMock := &PicolyticsO11y{
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Metrics: setupMetrics(1, "", "", ""),
	}

	// Initialize EchoAPI with the FileSystem
	echoAPI, err := NewEchoAPI(&Config{
		IPExtractor: "direct",
		Debug:       true,
	}, o11yMock)
	if err != nil {
		t.Fatalf("NewEchoAPI returned an error: %v", err)
	}
	//echoAPI.staticFS = http.FS(os.DirFS(tempDir))
	var usingEmbeddedStaticFiles UsingEmbeddedStaticFiles
	echoAPI.staticFS, usingEmbeddedStaticFiles, err = setupStaticFS(nil, tempDir)
	if err != nil {
		t.Fatalf("NewEchoAPI setupStaticFS returned an error: %v", err)
	}
	if usingEmbeddedStaticFiles {
		t.Fatalf("NewEchoAPI test should not be using embedded static files: %v", err)
	}

	if err := echoAPI.HandleStatic(c); err != nil {
		t.Fatalf("handleStatic returned an error: %v", err)
	}

	// Assert the response status code and body content
	if status := rec.Code; status != http.StatusOK {
		t.Errorf("handleStatic returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Checking the body content
	if body := rec.Body.String(); body != "test content" {
		t.Errorf("handleStatic returned unexpected body: got %v want %v", body, "test content")
	}
}

// TestProxySetup tests the proxySetup function
func TestProxySetup(t *testing.T) {
	o11yMock := &PicolyticsO11y{
		Logger:  slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})),
		Metrics: setupMetrics(1, "", "", ""),
	}
	tests := []struct {
		name           string
		trustedProxies []string
		ipExtractor    string
		wantErr        error
	}{
		{
			name:           "Valid XFF extractor with trusted proxies",
			trustedProxies: []string{"192.168.1.0/24", "10.0.0.0/8"},
			ipExtractor:    "xff",
			wantErr:        nil,
		},
		{
			name:        "Valid RealIP extractor with no trusted proxies",
			ipExtractor: "realip",
			wantErr:     nil,
		},
		{
			name:        "Direct IP extractor",
			ipExtractor: "direct",
			wantErr:     nil,
		},
		{
			name:        "Invalid IP extractor",
			ipExtractor: "invalid",
			wantErr:     errors.New("unknown IP extractor: invalid"),
		},
		{
			name:           "Invalid CIDR in trusted proxies",
			trustedProxies: []string{"invalidCIDR"},
			ipExtractor:    "xff",
			wantErr:        errors.New(`error parsing trustedProxies CIDR "invalidCIDR": invalid CIDR address: invalidCIDR`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			config := &Config{
				TrustedProxies: tt.trustedProxies,
				IPExtractor:    tt.ipExtractor,
			}
			err := proxySetup(e, config, o11yMock)
			if (err != nil) && err.Error() != tt.wantErr.Error() {
				t.Errorf("proxySetup error: want=%v, got=%v", tt.wantErr, err)
			}
		})
	}
}
