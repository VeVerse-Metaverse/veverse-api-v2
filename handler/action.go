package handler

//
//import (
//	"fmt"
//	"github.com/gofiber/fiber/v2"
//	"veverse-api/database"
//	"veverse-api/model"
//)
//
//func createApiAction(c *fiber.Ctx, requester *model.User, action *model.Action) {
//	if requester == nil {
//		fmt.Printf("failed to report api action: no requester")
//	}
//
//	db := database.DB
//
//	rows, err := db.Query(c.UserContext(), `
//INSERT INTO api_actions (id, method, route, params, result)
//VALUES ($1, $2, $3, $4, $5)
//`)
//
//	if err != nil {
//		fmt.Printf("failed to report api action: %s", err.Error())
//	}
//}
