package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/stripe/stripe-go/v73"
	checkoutSession "github.com/stripe/stripe-go/v73/checkout/session"
	"github.com/stripe/stripe-go/v73/webhook"
	"log"
	"os"
	"strings"
	"veverse-api/database"
	"veverse-api/model"
)

type CheckoutSessionInput struct {
	UserId           uuid.UUID `json:"user_id"`
	PriceId          string    `json:"price_id"`
	EventType        string    `json:"event_type"`
	EventTitle       string    `json:"event_title"`
	EventSummary     string    `json:"event_summary"`
	EventDescription string    `json:"event_description"`
	EventSpaceId     string    `json:"event_space_id"`
	EventStartsAt    string    `json:"event_starts_at"`
	EventEndsAt      string    `json:"event_ends_at"`
}

type WebhookPayload struct {
	Id      string `json:"id"`
	Created int    `json:"created"`
	Data    struct {
		Object struct {
			Id                 string      `json:"id"`
			Object             string      `json:"object"`
			Amount             int         `json:"amount"`
			AmountCaptured     int         `json:"amount_captured"`
			BalanceTransaction string      `json:"balance_transaction"`
			Captured           bool        `json:"captured"`
			Created            int         `json:"created"`
			Currency           string      `json:"currency"`
			Invoice            interface{} `json:"invoice"`
			BillingDetails     struct {
				Email string `json:"email"`
			} `json:"billing_Details"`
			Metadata struct {
				UserId           uuid.UUID `json:"user_id"`
				PriceId          string    `json:"price_id"`
				EventType        string    `json:"event_type"`
				EventTitle       string    `json:"event_title"`
				EventSummary     string    `json:"event_summary"`
				EventDescription string    `json:"event_description"`
				EventSpaceId     uuid.UUID `json:"event_space_id"`
				EventStartsAt    string    `json:"event_starts_at"`
				EventEndsAt      string    `json:"event_ends_at"`
			} `json:"metadata"`
			Paid                bool        `json:"paid"`
			PaymentIntent       string      `json:"payment_intent"`
			PaymentIntentMethod string      `json:"payment_intent_method"`
			PaymentMethod       string      `json:"payment_method"`
			ReceiptEmail        interface{} `json:"receipt_email"`
			ReceiptNumber       interface{} `json:"receipt_number"`
			ReceiptUrl          string      `json:"receipt_url"`
			Status              string      `json:"status"`
		} `json:"object"`
	} `json:"data"`
	PendingWebhooks int `json:"pending_webhooks"`
	Request         struct {
		Id             string `json:"id"`
		IdempotencyKey string `json:"idempotency_key"`
	} `json:"request"`
	Type string `json:"type"`
}

func CreateEventHook(c *fiber.Ctx) error {
	var payload WebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "error", "message": "Error reading request body"})
	}

	p, _ := json.Marshal(payload.Data.Object)
	fmt.Println("Creating event from metadata...", string(p))

	// This is your Stripe CLI webhook secret for testing your endpoint locally.
	endpointSecret := model.STRIPE_EVENT_PAYMENT_WEBHOOK_SECRET

	// Pass the request body and Stripe-Signature header to ConstructEvent, along
	// with the webhook signing key.
	signature := c.Request().Header.Peek("Stripe-Signature")
	event, err := webhook.ConstructEvent(c.Body(), string(signature), endpointSecret)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Error verifying webhook signature"})
	}

	// Unmarshal the event data into an appropriate struct depending on its Type
	switch event.Type {
	case "charge.succeeded":
		// Then define and call a function to handle the event charge.succeeded
		db := database.DB

		tx, err := db.Begin(c.UserContext())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		paymentObject := payload.Data.Object
		metadata := paymentObject.Metadata
		paymentId, err := uuid.NewV4()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
		}

		eventId, err := uuid.NewV4()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate uuid", "data": nil})
		}

		q := `INSERT INTO events 
			(id, name, title, summary, description, starts_at, ends_at, type, price, transaction_id, charge_id, payment_id, space_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`

		if _, err = tx.Exec(
			c.UserContext(),
			q,
			eventId, /*1*/
			strings.ReplaceAll(metadata.EventTitle, " ", "_"), /*2*/
			metadata.EventTitle,              /*3*/
			metadata.EventSummary,            /*4*/
			metadata.EventDescription,        /*5*/
			metadata.EventStartsAt,           /*6*/
			metadata.EventEndsAt,             /*7*/
			metadata.EventType,               /*8*/
			paymentObject.Amount,             /*9*/
			paymentObject.BalanceTransaction, /*10*/
			paymentObject.Id,                 /*11*/
			paymentId,                        /*12*/
			metadata.EventSpaceId,            /*13*/
		); err != nil {
			if err2 := tx.Rollback(c.UserContext()); err2 != nil {
				fmt.Println("Create Event Err5:", err2.Error())
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err2.Error()), "data": nil})
			}

			fmt.Println("Create Event Err6:", err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		q = `INSERT INTO payments 
			(id, user_id, entity_id, charge_id, balance_transaction_id, amount, email, currency, payment_intent_id, payment_method_id, receipt_url, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`

		if _, err = tx.Exec(
			c.Context(),
			q,
			paymentId,                          /*1*/
			metadata.UserId,                    /*2*/
			eventId,                            /*3*/
			metadata,                           /*4*/
			paymentObject.Id,                   /*5*/
			paymentObject.BalanceTransaction,   /*6*/
			paymentObject.Amount,               /*7*/
			paymentObject.BillingDetails.Email, /*8*/
			paymentObject.Currency,             /*9*/
			paymentObject.PaymentIntent,        /*10*/
			paymentObject.PaymentIntentMethod,  /*11*/
			paymentObject.ReceiptUrl,           /*12*/
			paymentObject.Status,               /*13*/
		); err != nil {
			if err2 := tx.Rollback(c.UserContext()); err2 != nil {
				fmt.Println("Create Event Err7:", err2.Error())
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err2.Error()), "data": nil})
			}

			fmt.Println("Create Event Err8:", err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}

		if err = tx.Commit(c.UserContext()); err != nil {
			if err2 := tx.Rollback(c.UserContext()); err2 != nil {
				fmt.Println("Create Event Err9:", err2.Error())
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": fmt.Sprintf("%s, %s", err.Error(), err2.Error()), "data": nil})
			}

			fmt.Println("Create Event Err10:", err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
	default:
		fmt.Fprintf(os.Stderr, "Unhandled event type: %s\n", event.Type)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

func CreateSessionForCheckout(c *fiber.Ctx) error {
	stripe.Key = model.STRIPE_API_SECRET_KEY

	var checkoutSessionData CheckoutSessionInput
	if err := c.BodyParser(&checkoutSessionData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	priceId := checkoutSessionData.PriceId
	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				// Provide the exact Price ID (for example, pr_1234) of the product you want to sell
				Price:    stripe.String(priceId),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(model.WEBAPP_ADDRESS + "/#/events/my"),
		CancelURL:  stripe.String(model.WEBAPP_ADDRESS + "/#/events/create"),
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{
				"user_id":           checkoutSessionData.UserId.String(),
				"price_id":          checkoutSessionData.PriceId,
				"event_type":        checkoutSessionData.EventType,
				"event_title":       checkoutSessionData.EventTitle,
				"event_summary":     checkoutSessionData.EventSummary,
				"event_description": checkoutSessionData.EventDescription,
				"event_space_id":    checkoutSessionData.EventSpaceId,
				"event_starts_at":   checkoutSessionData.EventStartsAt,
				"event_ends_at":     checkoutSessionData.EventEndsAt,
			},
		},
	}

	s, err := checkoutSession.New(params)

	if err != nil {
		log.Printf("session.New: %v", err)
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "ok", "checkoutURL": s.URL})
}
