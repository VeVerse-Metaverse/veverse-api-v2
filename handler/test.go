package handler

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
)

func GetTestDownload1G(c *fiber.Ctx) error {
	size := uint64(1) << 31
	bytes := make([]byte, size)
	c.Set("Text-Length", fmt.Sprintf("%d", size))
	c.Set("Accept-Ranges", "none")
	return c.Send(bytes)
}
