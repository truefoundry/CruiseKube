package config

import (
	"github.com/gin-gonic/gin"
)

func GetConfigFromGinContext(c *gin.Context) (*Config, bool) {
	appConfig, ok := c.MustGet("appConfig").(*Config)
	return appConfig, ok
}
