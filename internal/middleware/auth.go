package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/database"
	"github.com/gemone/model-router/internal/model"
	"github.com/gemone/model-router/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// AdminAuth admin authentication middleware
func AdminAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		cfg := config.GetConfig()

		// Check if AdminToken is configured
		if cfg.Security.AdminToken == "" {
			// If AdminToken is not configured, log warning and allow access (for development)
			WarnLog("AdminToken not configured - skipping authentication (OK for development)")
			return c.Next()
		}

		// Login and auth status endpoints don't require authentication
		path := c.Path()
		if path == "/api/admin/login" || path == "/api/admin/auth/status" {
			return c.Next()
		}

		// Get token from Authorization header or query parameter
		token := c.Get("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// Remove "Bearer " prefix
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// Use constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Security.AdminToken)) != 1 {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": "invalid or missing admin token",
			})
		}

		return c.Next()
	}
}

// ValidateAdminToken validates admin token (used for login)
func ValidateAdminToken(token string) bool {
	cfg := config.GetConfig()

	if cfg.Security.AdminToken == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(cfg.Security.AdminToken)) == 1
}

// ProfileAuth profile authentication middleware
// Validates that the Authorization token in the request matches the profile's configured API token
func ProfileAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get profile name from path parameter
		profilePath := c.Params("profile")
		if profilePath == "" {
			// No profile parameter, skip authentication (for default routes)
			return c.Next()
		}

		// Get token from request
		token := c.Get("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// Remove "Bearer " prefix
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// If no token provided, check if profile requires authentication
		if token == "" {
			// Query profile
			db := database.GetDB()
			var profile model.Profile
			if err := db.Where("path = ?", profilePath).First(&profile).Error; err == nil {
				// If profile has token configured, require authentication
				if profile.APITokenEnc != "" {
					return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
						"error":   "unauthorized",
						"message": "profile requires authentication",
					})
				}
			}
			// Profile doesn't have token configured, allow access
			return c.Next()
		}

		// Verify token matches profile's token
		db := database.GetDB()
		var profile model.Profile
		if err := db.Where("path = ?", profilePath).First(&profile).Error; err != nil {
			// Profile doesn't exist, continue processing (will return error later)
			return c.Next()
		}

		// If profile doesn't have token configured, allow access (backward compatibility)
		if profile.APITokenEnc == "" {
			return c.Next()
		}

		// Decrypt and verify token
		decryptedToken, err := utils.Decrypt(profile.APITokenEnc)
		if err != nil {
			ErrorLog("Failed to decrypt profile API token: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error":   "internal_error",
				"message": "failed to verify credentials",
			})
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(decryptedToken)) != 1 {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error":   "unauthorized",
				"message": "invalid profile token",
			})
		}

		return c.Next()
	}
}
