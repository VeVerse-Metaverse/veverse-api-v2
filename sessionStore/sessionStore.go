package sessionStore

import (
	"github.com/gofiber/fiber/v2/middleware/session"
	"time"
)

var Session *session.Store

func init() {
	config := session.Config{Expiration: time.Minute}
	Session = session.New(config)
}
