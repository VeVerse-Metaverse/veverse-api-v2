package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"veverse-api/ai"
	"veverse-api/aws/s3"
	"veverse-api/aws/ses"
	"veverse-api/database"
	_ "veverse-api/docs"
	"veverse-api/google/tts"
	"veverse-api/k8s"
	"veverse-api/router"
	"veverse-api/translation"
	"veverse-api/validation"

	"dev.hackerman.me/artheon/veverse-oauth-providers/eos"
	"dev.hackerman.me/artheon/veverse-oauth-providers/le7el"
	st "dev.hackerman.me/artheon/veverse-shared/telegram"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/helmet/v2"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/discord"
	"github.com/markbates/goth/providers/google"
	"github.com/sirupsen/logrus"
)

//go:generate swag init

// @title VeVerse API
// @version 1.0.0
// @description VeVerse API swagger
// @termsOfService http://swagger.io/terms/
// @contact.name Support
// @contact.email support@le7el.com
// @basePath /v2/
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func main() {
	const idleTimeout = 10 * time.Second
	const readTimeout = 30 * time.Second

	translation.InitTranslation()
	validation.RegisterValidations()

	app := fiber.New(fiber.Config{
		BodyLimit:   4 * 1024 * 1024 * 1024, // 4 GiB upload limit
		IdleTimeout: idleTimeout,
		ReadTimeout: readTimeout,
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			headers := make(map[string]string)
			ctx.Request().Header.VisitAll(func(key, value []byte) {
				// Don't log the Authorization header
				if string(key) == "Authorization" {
					return
				}
				headers[string(key)] = string(value)
			})

			msg := fmt.Sprintf("<pre>Error: %s, Request: %s %s, Headers: %v, Body: %s</pre>", err.Error(), ctx.Method(), ctx.Path(), headers, ctx.Request().Body())

			logrus.Errorf(msg)

			if err1 := st.SendTelegramMessage(msg); err1 != nil {
				logrus.Errorf("failed to send telegram message: %v", err1)
			}

			return err
		},
		EnableTrustedProxyCheck: true,
	})
	corsConfig := cors.Config{
		AllowOrigins: "*,127.0.0.1,127.0.0.1:3000,local.api.veverse.com,dev.api.veverse.com,test.api.veverse.com,api.veverse.com",
	}
	app.Use(cors.New(corsConfig))
	app.Use(helmet.New())
	//app.Use(csrf.New())

	oAuthGoogleKey := os.Getenv("OAUTH_GOOGLE_ID")
	oAuthGoogleSecret := os.Getenv("OAUTH_GOOGLE_SECRET")
	oAuthLe7elKey := os.Getenv("OAUTH_LE7EL_ID")
	oAuthLe7elSecret := os.Getenv("OAUTH_LE7EL_SECRET")
	oAuthEOSKey := os.Getenv("OAUTH_EOS_ID")
	oAuthEOSSecret := os.Getenv("OAUTH_EOS_SECRET")
	oAuthDiscordKey := os.Getenv("OAUTH_DISCORD_ID")
	oAuthDiscordSecret := os.Getenv("OAUTH_DISCORD_SECRET")

	baseAddress := os.Getenv("API_ADDRESS")
	goth.UseProviders(
		google.New(oAuthGoogleKey, oAuthGoogleSecret, baseAddress+"/oauth/google/callback"),
		le7el.New(oAuthLe7elKey, oAuthLe7elSecret, baseAddress+"/oauth/le7el/callback", "offline", "openid"),
		eos.New(oAuthEOSKey, oAuthEOSSecret, baseAddress+"/oauth/eos/callback"),
		discord.New(oAuthDiscordKey, oAuthDiscordSecret, baseAddress+"/oauth/discord/callback", "identify", "email"),
	)

	limiterConfig := limiter.Config{
		Max:        10000,
		Expiration: 1 * time.Minute,
	}

	app.Use(limiter.New(limiterConfig))
	app.Use(logger.New())

	if err := database.Setup(); err != nil {
		log.Fatal(err)
	}
	app.Use(database.NewMiddleware())

	if err := k8s.Setup(); err != nil {
		log.Fatal(err)
	}
	app.Use(k8s.NewMiddleware())

	if err := database.SetupClickhouse(); err != nil {
		log.Fatal(err)
	}
	app.Use(database.NewClickhouseMiddleware())

	if err := ai.Setup(); err != nil {
		log.Fatal(err)
	}
	app.Use(ai.NewMiddleware())

	if err := tts.Setup(); err != nil {
		log.Fatal(err)
	}
	app.Use(tts.NewMiddleware())

	if err := s3.Setup(); err != nil {
		log.Fatal(err)
	}

	if err := ses.Setup(); err != nil {
		log.Fatal(err)
	}

	if err := ai.Setup(); err != nil {
		log.Println(err)
	}
	app.Use(ai.NewMiddleware())

	router.SetupRoutes(app)

	port := os.Getenv("API_PORT")
	portNum, err := strconv.Atoi(port)
	if err != nil {
		port = "3000"
	} else if portNum <= 0 || portNum > 65535 {
		port = "3000"
	}
	log.Fatal(app.Listen(":" + port))
}
