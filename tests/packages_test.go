package tests

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexPackages(t *testing.T) {
	tests := []struct {
		name         string
		route        string
		expectedCode int
		admin        bool
	}{
		{
			"packages-index (user)",
			"/v2/packages",
			200,
			false,
		},
		{
			"packages-index (user) &offset=0&limit=10",
			"/v2/packages?offset=0&limit=10",
			200,
			false,
		},
		{
			"packages-index (user) &platform=Win64&deployment=Client&offset=0&limit=10",
			"/v2/packages?offset=0&limit=10&platform=Win64&deployment=Client",
			200,
			false,
		},
		{
			"packages-index (user) &platform=Linux&deployment=Server&offset=0&limit=10",
			"/v2/packages?offset=0&limit=10&platform=Linux&deployment=Server",
			200,
			false,
		},
		{
			"packages-index (user) &query=a",
			"/v2/packages?query=a",
			200,
			false,
		},
		{
			"packages-index (user) offset=0&limit=10&query=a",
			"/v2/packages?offset=0&limit=10&query=a",
			200,
			false,
		},
		{
			"packages-index (user) offset=0&limit=10&platform=Win64&deployment=Client&query=a",
			"/v2/packages?offset=0&limit=10&platform=Win64&deployment=Client&query=a",
			200,
			false,
		},
		{
			"packages-index (user) offset=0&limit=10&platform=Linux&deployment=Server&query=a",
			"/v2/packages?offset=0&limit=10&platform=Linux&deployment=Server&query=a",
			200,
			false,
		},
		{
			"packages-index (admin)",
			"/v2/packages",
			200,
			true,
		},
		{
			"packages-index (admin) ?offset=0&limit=10",
			"/v2/packages?offset=0&limit=10",
			200,
			true,
		},
		{
			"packages-index (admin) offset=0&limit=10&platform=Win64&deployment=Client",
			"/v2/packages?offset=0&limit=10&platform=Win64&deployment=Client",
			200,
			true,
		},
		{
			"packages-index (admin) offset=0&limit=10&platform=Linux&deployment=Server",
			"/v2/packages?offset=0&limit=10&platform=Linux&deployment=Server",
			200,
			true,
		},
		{
			"packages-index (admin) &query=a",
			"/v2/packages?query=a",
			200,
			true,
		},
		{
			"packages-index (admin) offset=0&limit=10&query=a",
			"/v2/packages?offset=0&limit=10&query=a",
			200,
			true,
		},
		{
			"packages-index (admin) offset=0&limit=10&platform=Win64&deployment=Client&query=a",
			"/v2/packages?offset=0&limit=10&platform=Win64&deployment=Client&query=a",
			200,
			true,
		},
		{
			"packages-index (admin) offset=0&limit=10&platform=Linux&deployment=Server&query=a",
			"/v2/packages?offset=0&limit=10&platform=Linux&deployment=Server&query=a",
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

func TestGetPackages(t *testing.T) {
	tests := []struct {
		name         string
		route        string
		expectedCode int
		admin        bool
	}{
		{
			"get HTTP status 200",
			"/v2/packages/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/packages/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/packages/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/?platform=Win64&deployment=Client",
			200,
			false,
		},
		{
			"get HTTP status 200",
			"/v2/packages/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/?platform=Linux&deployment=Server",
			200,
			false,
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
