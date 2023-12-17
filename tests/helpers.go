package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"veverse-api/aws/s3"
	"veverse-api/aws/ses"
	"veverse-api/database"
	"veverse-api/router"
)

func createApp() *fiber.App {
	app := fiber.New()
	app.Use(logger.New())

	err := database.Setup()
	if err != nil {
		log.Fatal(err)
	}

	if err = s3.Setup(); err != nil {
		log.Fatal(err)
	}

	if err = ses.Setup(); err != nil {
		log.Fatal(err)
	}

	app.Use(database.NewMiddleware())
	app.Use(database.NewClickhouseMiddleware())

	router.SetupRoutes(app)

	app.Get("/test/email", func(c *fiber.Ctx) error {
		htmlTemplate := `<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Hello, Package</body></html>`
		if err = ses.Send("VeVerse - Test Email", "Hello, Package", htmlTemplate, []string{}, []string{}, []string{}, ""); err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(fmt.Sprintln(err.Error())))

		}
		return c.Status(fiber.StatusOK).Send([]byte{})
	})

	return app
}

func login(app *fiber.App, admin bool) (string, error) {
	var (
		requestBody []byte
		err         error
	)
	if admin {
		requestBody, err = json.Marshal(map[string]string{
			"email":    os.Getenv("TEST_ADMIN_EMAIL"),
			"password": os.Getenv("TEST_ADMIN_PASSWORD"),
		})
	} else {
		requestBody, err = json.Marshal(map[string]string{
			"email":    os.Getenv("TEST_USER_EMAIL"),
			"password": os.Getenv("TEST_USER_PASSWORD"),
		})
	}

	if err != nil {
		log.Fatalln(err)
	}

	req := httptest.NewRequest("POST", "/v2/auth/login", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var v map[string]string
	if err = json.Unmarshal(body, &v); err != nil {
		return "", err
	}

	if v["status"] == "error" {
		return "", errors.New(fmt.Sprintf("authentication error %d: %s\n", resp.StatusCode, v["message"]))
	} else if v["status"] == "ok" {
		return v["data"], nil
	}

	return "", errors.New(v["message"])
}
