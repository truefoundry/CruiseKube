package config

import (
	"github.com/gin-gonic/gin"
)

// GetConfigFromGinContext retrieves the application config from the Gin context.
// It uses MustGet rather than Get because "appConfig" is guaranteed to be set by
// the Dependencies middleware, which is always wired via middleware.Common() in
// SetupServerEngine. A missing key indicates a programmer error (e.g. a route
// registered outside the common middleware chain), and MustGet surfaces that
// immediately. gin.Default() includes gin.Recovery(), so any resulting panic is
// caught and returns HTTP 500 without crashing the server.
func GetConfigFromGinContext(c *gin.Context) *Config {
	return c.MustGet("appConfig").(*Config)
}
