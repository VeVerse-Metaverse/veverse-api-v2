package handler

import (
	"github.com/gofiber/fiber/v2"
	"strings"
	"veverse-api/database"
)

type SignupRequestMetadata struct {
	Name          string  `json:"name,omitempty"`
	Email         string  `json:"email,omitempty"`
	Type          string  `json:"type,omitempty"`
	Area          *string `json:"area,omitempty"`
	Company       *string `json:"company,omitempty"`
	WebsiteUrl    *string `json:"websiteUrl,omitempty"`
	SocialUrl     *string `json:"socialUrl,omitempty"`
	Message       *string `json:"message,omitempty"`
	RequestedSdk  *bool   `json:"requestedSdk,omitempty"`
	AgreedToTerms bool    `json:"agreedToTerms,omitempty"`
}

// Signup godoc
// @Summary      Signup
// @Description  Signup user for the mailing list
// @Tags         signup
// @Accept       json
// @Produce      json
// @Param        request body handler.SignupRequestMetadata true "Request JSON"
// @Success      200  {object}  string
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /signup [post]
func Signup(c *fiber.Ctx) (err error) {

	m := SignupRequestMetadata{}
	if err = c.BodyParser(&m); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if m.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid type", "data": nil})
	}

	if m.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid name", "data": nil})
	}

	if m.Email == "" || (!strings.Contains(m.Email, "@") && !strings.Contains(m.Email, ".")) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid email", "data": nil})
	}

	if m.AgreedToTerms == false {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "not agreed to terms and conditions", "data": nil})
	}

	db := database.DB
	q := `SELECT COUNT(*) FROM signups WHERE type = $1 AND email = $2`
	row := db.QueryRow(c.UserContext(), q, m.Type, m.Email)

	var total int64
	if err = row.Scan(&total); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if total > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "already subscribed", "data": nil})
	}

	if m.Type == "creator" {
		if m.Area == nil || *m.Area == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid interest area", "data": nil})
		}

		q = `INSERT INTO signups (
                     name,
                     email,
                     area,
                     company,
                     website_url,
                     social_url,
                     message,
                     requested_sdk,
                     agreed_to_terms,
                     type) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

		var (
			company      string
			websiteUrl   string
			socialUrl    string
			message      string
			requestedSdk bool
		)

		if m.Company == nil {
			company = ""
		} else {
			company = *m.Company
		}

		if m.WebsiteUrl == nil {
			websiteUrl = ""
		} else {
			websiteUrl = *m.WebsiteUrl
		}

		if m.SocialUrl == nil {
			socialUrl = ""
		} else {
			socialUrl = *m.SocialUrl
		}

		if m.Message == nil {
			message = ""
		} else {
			message = *m.Message
		}

		if m.RequestedSdk == nil {
			requestedSdk = false
		} else {
			requestedSdk = *m.RequestedSdk
		}

		if _, err = db.Query(c.UserContext(), q, m.Name, m.Email, m.Area, company, websiteUrl, socialUrl, message, requestedSdk, m.AgreedToTerms, m.Type); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else if m.Type == "user" {
		q = `INSERT INTO signups (
                     name,
                     email,
                     agreed_to_terms,
                     type) VALUES ($1, $2, $3, $4)`

		if _, err = db.Query(c.UserContext(), q, m.Name, m.Email, m.AgreedToTerms, m.Type); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid signup type", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": nil, "data": nil})
}
