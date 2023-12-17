package model

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

func SendDiscordMessage(message string) error {
	url := os.Getenv("DISCORD_HOOK_URL")
	if url == "" {
		return fmt.Errorf("failed to trigger discord hook: DISCORD_HOOK_URL env is empty")
	}

	res, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(fmt.Sprintf("{\"content\":\"%s\"}", message))))
	if err != nil {
		return fmt.Errorf("failed to trigger discord hook: %v", err)
	}

	if res.StatusCode >= 400 {
		content, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to send request, status code: %d", res.StatusCode)
		} else {
			defer func(body io.ReadCloser) {
				err := body.Close()
				if err != nil {
					logrus.Errorf("failed to close request body: %v", err)
				}
			}(res.Body)
			return fmt.Errorf("failed to send request, status code: %d, content: %s", res.StatusCode, content)
		}
	}

	return nil
}
