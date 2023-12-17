package tts

import (
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
)

var Polly *polly.Polly
var Session *session.Session

var awsRegion = os.Getenv("AWS_S3_REGION")

func Setup() (err error) {
	config := aws.NewConfig()
	config.Region = aws.String(awsRegion)
	Session, err = session.NewSession(config)
	if err != nil {
		log.Fatalf("failed to initialize a new AWS session: %v", err)
	}

	Polly = polly.New(Session)
	if Polly == nil {
		return fmt.Errorf("failed to create a Polly client")
	}

	return err
}

func NewMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), vContext.TTS, Polly))

		return c.Next()
	}
}
