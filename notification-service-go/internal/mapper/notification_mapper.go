package mapper

import (
	"Banka1Back/notification-service-go/internal/dto"
	"Banka1Back/notification-service-go/internal/model"
)

// ToDeliveryStatusResponse maps a NotificationDelivery entity to its outbound DTO.
func ToDeliveryStatusResponse(d *model.NotificationDelivery) dto.DeliveryStatusResponse {
	var lastErr string
	if d.LastError != nil {
		lastErr = *d.LastError
	}
	return dto.DeliveryStatusResponse{
		DeliveryID:   d.DeliveryID,
		Status:       string(d.Status),
		AttemptCount: d.AttemptCount,
		LastError:    lastErr,
	}
}
