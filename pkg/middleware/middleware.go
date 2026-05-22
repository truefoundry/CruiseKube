package middleware

import (
	"fmt"
	"net/http"

	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/config"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"
	"github.com/truefoundry/cruisekube/pkg/metrics"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// AuthBasic returns a gin.HandlerFunc that enforces HTTP Basic Auth when auth is enabled.
// When auth.Enabled is false, it passes all requests through without authentication.
func AuthBasic(auth config.AuthConfig) gin.HandlerFunc {
	if !auth.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	if auth.Username == "" || auth.Password == "" {
		return func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server authentication is enabled but credentials are not configured"})
			c.Abort()
		}
	}
	return gin.BasicAuth(gin.Accounts{
		auth.Username: auth.Password,
	})
}

func AuthWebhook() gin.HandlerFunc {
	return func(c *gin.Context) {
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
		route := "unmatched"
		if api, ok := contextutils.GetKey(ctx, contextutils.APIContextKey); ok {
			route = api
		}
		metrics.ObserveHTTPServerRequestDuration(route, param.Method, param.StatusCode, param.Latency)
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
