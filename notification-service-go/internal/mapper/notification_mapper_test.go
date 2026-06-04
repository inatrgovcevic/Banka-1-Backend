package mapper_test

import (
	"testing"

	"Banka1Back/notification-service-go/internal/mapper"
	"Banka1Back/notification-service-go/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestToDeliveryStatusResponse_WithLastError(t *testing.T) {
	t.Parallel()
	errMsg := "connection refused"
	d := &model.NotificationDelivery{
		DeliveryID:   "del-1",
		Status:       model.StatusFailed,
		AttemptCount: 3,
		LastError:    &errMsg,
	}

	resp := mapper.ToDeliveryStatusResponse(d)

	assert.Equal(t, "del-1", resp.DeliveryID)
	assert.Equal(t, "FAILED", resp.Status)
	assert.Equal(t, 3, resp.AttemptCount)
	assert.Equal(t, "connection refused", resp.LastError)
}

func TestToDeliveryStatusResponse_NoLastError(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{
		DeliveryID:   "del-2",
		Status:       model.StatusSucceeded,
		AttemptCount: 1,
		LastError:    nil,
	}

	resp := mapper.ToDeliveryStatusResponse(d)

	assert.Equal(t, "del-2", resp.DeliveryID)
	assert.Equal(t, "SUCCEEDED", resp.Status)
	assert.Equal(t, 1, resp.AttemptCount)
	assert.Empty(t, resp.LastError)
}

func TestToDeliveryStatusResponse_PendingZeroAttempts(t *testing.T) {
	t.Parallel()
	d := &model.NotificationDelivery{
		DeliveryID:   "del-3",
		Status:       model.StatusPending,
		AttemptCount: 0,
	}

	resp := mapper.ToDeliveryStatusResponse(d)

	assert.Equal(t, "PENDING", resp.Status)
	assert.Equal(t, 0, resp.AttemptCount)
	assert.Empty(t, resp.LastError)
}
