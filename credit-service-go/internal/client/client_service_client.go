package client

import (
	"errors"
	"fmt"
	"net/http"
	"os"
)

type ClientServiceClient struct {
	baseURL string
	http    *http.Client
}

func NewClientServiceClient() *ClientServiceClient {
	baseURL := os.Getenv("SERVICES_USER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8083"
	}

	return &ClientServiceClient{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

func (c *ClientServiceClient) AddMarginPermission(clientID int64) error {
	url := fmt.Sprintf("%s/customers/margin/%d", c.baseURL, clientID)

	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return err
	}

	if token := os.Getenv("INTERNAL_AUTH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("client-service add margin permission failed")
	}

	return nil
}
