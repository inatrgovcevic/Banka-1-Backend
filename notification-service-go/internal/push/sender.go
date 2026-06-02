package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2/google"
)

type SenderConfig struct {
	CredentialsFile string
	ProjectID       string
	HTTPTimeout     time.Duration
}

func SenderConfigFromEnv() (SenderConfig, error) {
	creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if creds == "" {
		return SenderConfig{}, errors.New("FCM: GOOGLE_APPLICATION_CREDENTIALS env var is required (path to service account JSON key file)")
	}
	projectID := os.Getenv("FCM_PROJECT_ID")
	if projectID == "" {
		return SenderConfig{}, errors.New("FCM: FCM_PROJECT_ID env var is required")
	}
	return SenderConfig{
		CredentialsFile: creds,
		ProjectID:       projectID,
		HTTPTimeout:     15 * time.Second,
	}, nil
}

type FCMSender struct {
	projectID string
	client    *http.Client
	log       *slog.Logger
}

func NewFCMSender(cfg SenderConfig, log *slog.Logger) (*FCMSender, error) {
	credBytes, err := os.ReadFile(cfg.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("FCM: read credentials file %q: %w", cfg.CredentialsFile, err)
	}

	jwtCfg, err := google.JWTConfigFromJSON(credBytes, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, fmt.Errorf("FCM: parse service account JSON from %q: %w", cfg.CredentialsFile, err)
	}

	httpClient := jwtCfg.Client(context.Background())
	httpClient.Timeout = cfg.HTTPTimeout

	return &FCMSender{
		projectID: cfg.ProjectID,
		client:    httpClient,
		log:       log,
	}, nil
}

type fcmV1Message struct {
	Message fcmV1MessageBody `json:"message"`
}

type fcmV1MessageBody struct {
	Token        string            `json:"token"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *fcmAndroidConfig `json:"android,omitempty"`
	APNS         *fcmAPNSConfig    `json:"apns,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroidConfig struct {
	Priority string `json:"priority"`
}

type fcmAPNSConfig struct {
	Headers fcmAPNSHeaders `json:"headers"`
}

type fcmAPNSHeaders struct {
	APNSPriority string `json:"apns-priority"`
}

func priorityConfig() (*fcmAndroidConfig, *fcmAPNSConfig) {
	return &fcmAndroidConfig{Priority: "high"},
		&fcmAPNSConfig{Headers: fcmAPNSHeaders{APNSPriority: "10"}}
}

func (s *FCMSender) url() string {
	return fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", s.projectID)
}

func (s *FCMSender) SendNotification(ctx context.Context, deviceToken, title, body string) error {
	android, apns := priorityConfig()
	msg := fcmV1Message{
		Message: fcmV1MessageBody{
			Token: deviceToken,
			Notification: &fcmNotification{
				Title: title,
				Body:  body,
			},
			Android: android,
			APNS:    apns,
		},
	}

	return s.send(ctx, msg)
}

func (s *FCMSender) SendData(ctx context.Context, deviceToken string, data map[string]string) error {
	if len(data) == 0 {
		data = make(map[string]string)
	}

	android, apns := priorityConfig()
	msg := fcmV1Message{
		Message: fcmV1MessageBody{
			Token:   deviceToken,
			Data:    data,
			Android: android,
			APNS:    apns,
		},
	}

	return s.send(ctx, msg)
}

func (s *FCMSender) send(ctx context.Context, msg fcmV1Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("fcm marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("fcm http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.log.Debug("fcm push sent successfully", "status", resp.StatusCode)
		return nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	s.log.Error("fcm push failed",
		"status", resp.StatusCode,
		"response", string(respBody),
	)
	return fmt.Errorf("fcm push returned status %d: %s", resp.StatusCode, string(respBody))
}
