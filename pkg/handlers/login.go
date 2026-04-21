package handlers

import (
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/truefoundry/cruisekube/pkg/config"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	// Token is the base64 payload for Authorization: Basic <Token> (RFC 7617).
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
}

// HandleLogin validates admin credentials for the login screen. The API uses HTTP Basic auth;
// clients should send Authorization: Basic <token> on subsequent requests.
func HandleLogin(auth config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.Username == "" || auth.Password == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server authentication is not configured"})
			return
		}

		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(req.Username), []byte(auth.Username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(req.Password), []byte(auth.Password)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		basic := base64.StdEncoding.EncodeToString([]byte(req.Username + ":" + req.Password))
		c.JSON(http.StatusOK, loginResponse{
			Token:     basic,
			TokenType: "Basic",
		})
	}
}
