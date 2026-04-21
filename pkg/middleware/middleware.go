package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func AuthWebhook() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func AuthAPI(username, password string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if username == "" || password == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "server authentication is not configured"})
			return
		}

		authHeader := c.GetHeader("Authorization")
		const prefix = "Basic "
		if !strings.HasPrefix(authHeader, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header, expected: Basic <credentials>"})
			return
		}

		raw, err := base64.StdEncoding.DecodeString(authHeader[len(prefix):])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid basic authorization encoding"})
			return
		}

		pair := strings.SplitN(string(raw), ":", 2)
		if len(pair) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid basic authorization format"})
			return
		}

		givenUser, givenPass := pair[0], pair[1]
		if subtle.ConstantTimeCompare([]byte(givenUser), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(givenPass), []byte(password)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		c.Next()
	}
}

func CorsMiddleware() gin.HandlerFunc {
	cfg := cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With"},
		AllowCredentials: true,
	}
	return cors.New(cfg)
}

func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if fullPath := c.FullPath(); fullPath != "" {
			ctx = contextutils.WithAPI(ctx, fullPath)
		}

		if clusterID := c.Param("clusterID"); clusterID != "" {
			ctx = contextutils.WithCluster(ctx, clusterID)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		ctx := param.Request.Context()
		logging.Infof(ctx, "HTTP %s %s - %d %dbytes %s",
			param.Method,
			param.Path,
			param.StatusCode,
			param.BodySize,
			param.Latency,
		)
		return ""
	})
}

func EnsureClusterExists(mgr cluster.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("clusterID") == cluster.SingleClusterID {
			c.Next()
			return
		}

		clusterID := c.Param("clusterID")

		if clusterID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cluster endpoint format"})
			c.Abort()
			return
		}

		if _, err := mgr.GetClusterClients(clusterID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Cluster %s not found", clusterID)})
			c.Abort()
			return
		}

		c.Next()
	}
}

func Common() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		Logger(),
		gin.Recovery(),
		CorsMiddleware(),
		RequestContext(),
	}
}
