package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleRoot_AuthEnabledTrue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := HandleRoot(true)

	r := gin.New()
	r.GET("/", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body rootResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Message != "cruisekube API Server" {
		t.Errorf("expected message 'cruisekube API Server', got %q", body.Message)
	}
	if !body.AuthEnabled {
		t.Errorf("expected auth_enabled=true, got %v", body.AuthEnabled)
	}
	if len(body.Endpoints) == 0 {
		t.Error("expected non-empty endpoints map")
	}
}

func TestHandleRoot_AuthEnabledFalse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := HandleRoot(false)

	r := gin.New()
	r.GET("/", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body rootResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Message != "cruisekube API Server" {
		t.Errorf("expected message 'cruisekube API Server', got %q", body.Message)
	}
	if body.AuthEnabled {
		t.Errorf("expected auth_enabled=false, got %v", body.AuthEnabled)
	}
	if len(body.Endpoints) == 0 {
		t.Error("expected non-empty endpoints map")
	}
}
