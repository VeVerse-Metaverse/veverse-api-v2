package ai

// change to main package to run this file

import (
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"github.com/gofiber/fiber/v2"
	"os"
)

var driver *Driver

func Setup() error {
	apiKey := os.Getenv("OPENAI_API_KEY")
	driver = MakeDriver(apiKey)
	return nil
}

func NewMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), vContext.AI, driver))

		return c.Next()
	}
}
