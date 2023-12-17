package tests

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApp(t *testing.T) {
	tests := []struct {
		name         string
		route        string
		expectedCode int
		admin        bool
	}{
		{
			"get app identity images",
			"/v2/apps/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/images/identity",
			200,
			true,
		},
	}

	app := createApp()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := login(app, tt.admin)
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest("GET", tt.route, nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			jsonStr := string(body)

			fmt.Printf("%s\n", jsonStr)

			if !assert.Equal(t, tt.expectedCode, resp.StatusCode, tt.name) {
				t.Fail()
			}
		})
	}
}
