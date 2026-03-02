package config

import (
	"github.com/gin-gonic/gin"
)

// GetConfigFromGinContext retrieves the application config from the Gin context.
// MustGet is intentional: "appConfig" is always set by the Dependencies middleware
// (wired via middleware.Common). A missing key is a programmer error, not a runtime
// condition; gin.Recovery() catches the panic and returns HTTP 500.
func GetConfigFromGinContext(c *gin.Context) *Config {
	return c.MustGet("appConfig").(*Config)
}
