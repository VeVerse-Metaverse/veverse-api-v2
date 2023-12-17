package tts

import (
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"fmt"
	"github.com/gofiber/fiber/v2"
)

var GoogleTTS *texttospeech.Client

func Setup() (err error) {
	GoogleTTS, err = texttospeech.NewClient(context.Background())
	if GoogleTTS == nil {
		return fmt.Errorf("failed to create a TTS client")
	}

	return err
}

func NewMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), vContext.TTS, GoogleTTS))

		return c.Next()
	}
}
