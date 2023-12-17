package middleware

import (
	"github.com/gofiber/fiber/v2"
	jwtMiddleware "github.com/gofiber/jwt/v2"
	"os"
	"veverse-api/middleware/apiSecret"
)

var secret = os.Getenv("AUTH_SECRET")

func ProtectedApi() func(*fiber.Ctx) error {
	return apiSecret.New()
}

func ProtectedPublicApi() func(*fiber.Ctx) error {
	return apiSecret.ProtectPublic()
}

// ProtectedJwt Protected protect routes
func ProtectedJwt() func(*fiber.Ctx) error {
	return jwtMiddleware.New(jwtMiddleware.Config{
		SigningKey:   []byte(secret),
		ErrorHandler: jwtError,
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		c.Status(fiber.StatusBadRequest)
		return c.JSON(fiber.Map{"status": "error", "message": "Missing or malformed JWT", "data": nil})
	} else {
		c.Status(fiber.StatusUnauthorized)
		return c.JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
}
