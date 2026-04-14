package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/truefoundry/cruisekube/pkg/cluster"
	"github.com/truefoundry/cruisekube/pkg/contextutils"
	"github.com/truefoundry/cruisekube/pkg/logging"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthWebhook() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func AuthAPI(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format, expected: Bearer <token>"})
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
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
