package market

import (
	"context"
	"testing"
	"time"
)

type recordingPriceAlertPublisher struct {
	payloads []PriceAlertNotificationPayload
}

func (p *recordingPriceAlertPublisher) PublishPriceAlertTriggered(ctx context.Context, payload PriceAlertNotificationPayload) error {
	p.payloads = append(p.payloads, payload)
	return nil
}

func TestAlertSatisfiedConditions(t *testing.T) {
	listing := Listing{Price: "100.00", Change: "-12.00"}
	cases := []struct {
		name      string
		condition PriceAlertCondition
		threshold string
		want      bool
	}{
		{"above inclusive", PriceAlertAbove, "100.00", true},
		{"below false", PriceAlertBelow, "99.99", false},
		{"intraday drop", PriceAlertPctDropIntraday, "10.00", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alert := PriceAlert{Condition: tc.condition, Threshold: tc.threshold}
			if got := alertSatisfied(alert, listing); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestPriceAlertPayloadIncludesJavaAndCompatibilityVariables(t *testing.T) {
	alert := PriceAlert{ID: 1, UserID: 7, RecipientType: "CLIENT", Condition: PriceAlertAbove, Threshold: "120.0000"}
	listing := Listing{Ticker: "AAPL", Price: "121.50000000"}

	payload := priceAlertPayload(alert, listing)

	if payload.ClientID == nil || *payload.ClientID != 7 {
		t.Fatalf("expected client id 7, got %#v", payload.ClientID)
	}
	if payload.TemplateVariables["ticker"] != "AAPL" ||
		payload.TemplateVariables["price"] != "121.50000000" ||
		payload.TemplateVariables["triggeredPrice"] != "121.50000000" ||
		payload.TemplateVariables["threshold"] != "120.0000" ||
		payload.TemplateVariables["condition"] != "ABOVE" {
		t.Fatalf("unexpected template variables: %#v", payload.TemplateVariables)
	}
}

func TestNormalizeNotificationType(t *testing.T) {
	got, err := normalizeNotificationType(" email ")
	if err != nil || got != "EMAIL" {
		t.Fatalf("expected EMAIL, got %q err=%v", got, err)
	}
	if _, err := normalizeNotificationType("sms"); err != ErrBadRequest {
		t.Fatalf("expected bad request for unsupported type, got %v", err)
	}
}

func TestPriceAlertLastTriggeredDebounceShape(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	alert := PriceAlert{LastTriggeredAt: &now}
	if alert.LastTriggeredAt == nil || !alert.LastTriggeredAt.Equal(now) {
		t.Fatal("expected alert to carry debounce timestamp")
	}
}
