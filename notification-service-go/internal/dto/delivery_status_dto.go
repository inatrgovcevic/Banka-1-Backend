package dto

// DeliveryStatusResponse is the outbound DTO for delivery lifecycle status queries.
type DeliveryStatusResponse struct {
	DeliveryID   string `json:"deliveryId"`
	Status       string `json:"status"`
	AttemptCount int    `json:"attemptCount"`
	LastError    string `json:"lastError,omitempty"`
}
