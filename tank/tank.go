package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

type Payload struct {
	Data string `json:"data"`
}

func sendRequest() {
	requestURL := fmt.Sprintf("http://localhost:%d", 3000)
	res, err := http.Get(requestURL)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d\n", res.StatusCode)

	values := map[string]string{"email": "tank@veverse.com", "password": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	json_data, err := json.Marshal(values)

	requestURL = fmt.Sprintf("http://localhost:%d/v2/auth/login", 3000)
	res, err = http.Post(requestURL, "application/json", bytes.NewBuffer(json_data))
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var payload Payload
	err = json.Unmarshal(b, &payload)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d, client body: %s\n", res.StatusCode, (string)(b))

	requestURL = fmt.Sprintf("http://localhost:%d/v2/spaces/%s/placeables", 3000, "8455697e-af76-4c1c-8a71-00a319b7addf")
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", payload.Data))

	client := &http.Client{}
	res, err = client.Do(req)
	if err != nil {
		fmt.Printf("error making http request: %s\n", err)
		os.Exit(1)
	}

	defer res.Body.Close()

	b, err = io.ReadAll(res.Body)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("client: got response!\n")
	fmt.Printf("client: status code: %d, client body: %s\n", res.StatusCode, (string)(b))
}

func main() {
	var wg sync.WaitGroup

	for i := 0; i < 1; i++ { // number of parallel goroutines
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i <= 100; i++ { // number of consecutive requests
				sendRequest()
			}
		}()
	}

	wg.Wait()
}
