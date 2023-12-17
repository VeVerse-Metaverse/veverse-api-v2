package tests

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http/httptest"
	"testing"
)

func TestIndexWorlds(t *testing.T) {
	tests := []struct {
		name         string
		route        string
		expectedCode int
		admin        bool
	}{
		{
			"get HTTP status 200",
			"/v2/worlds",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10&platform=Win64&deployment=Client",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10&platform=Linux&deployment=Server",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/worlds",
			200,
			true,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10",
			200,
			true,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10&platform=Win64&deployment=Client",
			200,
			true,
		},
		{
			"get HTTP status 200",
			"/v2/worlds?offset=0&limit=10&platform=Linux&deployment=Server",
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

			if !assert.Equal(t, tt.expectedCode, resp.StatusCode, tt.name) {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}

				jsonStr := string(body)

				fmt.Printf("%s\n", jsonStr)
			}
		})
	}
}
