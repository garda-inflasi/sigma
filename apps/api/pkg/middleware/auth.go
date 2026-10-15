package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/garda-inflasi/api/pkg/logger"
	"github.com/garda-inflasi/api/pkg/token"
)

// JWTAuth returns a Gin middleware that validates JWT access tokens
// and sets user_id, role, and permissions in the Gin context.
func JWTAuth(tokenMgr *token.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		bearer := c.GetHeader("Authorization")
		if bearer == "" || len(bearer) < 7 || !strings.EqualFold(bearer[:7], "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "401",
				"message": "Unauthorized",
			})
			return
		}

		tokenStr := bearer[7:]
		claims, err := tokenMgr.VerifyToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "401",
				"message": "Token tidak valid",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Set("permissions", claims.Permissions)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// RequirePermission returns a Gin middleware that checks if the authenticated
// user has the specified permission. Must be used after JWTAuth middleware.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "403",
				"message": "Akses ditolak",
			})
			return
		}

		permList, ok := perms.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "403",
				"message": "Akses ditolak",
			})
			return
		}

		hasPermission := false
		for _, p := range permList {
			if p == permission {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "403",
				"message": "Akses ditolak",
			})
			return
		}

		c.Next()
	}
}

// AccessLog logs every request with its duration, status, and request ID.
func AccessLog(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Log after response
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		ip := c.ClientIP()
		reqID, _ := c.Get("request_id")

		fields := map[string]interface{}{
			"method":     method,
			"path":       path,
			"status":     status,
			"ip":         ip,
			"request_id": reqID,
		}

		if userID, exists := c.Get("user_id"); exists {
			fields["user_id"] = userID
		}

		if status >= 500 {
			log.Error("server error", fields)
		} else if status >= 400 {
			log.Warn("client error", fields)
		} else {
			log.Info("request", fields)
		}
	}
}
