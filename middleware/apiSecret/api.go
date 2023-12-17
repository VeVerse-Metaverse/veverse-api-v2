package apiSecret

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"os"
	"veverse-api/database"
	"veverse-api/model"
)

var PublicProtectionKey = os.Getenv("PUBLIC_PROTECTED_KEY")

func New() fiber.Handler {
	return func(c *fiber.Ctx) error {
		envSecret := os.Getenv("VE_SERVICE_SECRET")
		headers := c.GetReqHeaders()
		headerSecret := headers["X-Ve-Key"]

		if envSecret == headerSecret {
			return c.Next()
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "not authorized", "data": nil})
	}
}

func ProtectPublic() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		m := model.KeyRequestMetadata{}

		if err := ctx.ParamsParser(&m); err != nil {
			fmt.Println("ProtectPublic param parser err:", err.Error())
			return ctx.Next()
		}

		if m.Key == PublicProtectionKey {
			db := database.DB
			q := `SELECT 1 as num`

			var row pgx.Row
			row = db.QueryRow(ctx.UserContext(), q)

			var num int8
			if err := row.Scan(&num); err != nil {
				fmt.Println("ProtectPublic err:", err.Error())
				ctx.Status(fiber.StatusRequestTimeout)
				return ctx.JSON(fiber.Map{"status": "error"})
			}
		}

		return ctx.Next()
	}
}
