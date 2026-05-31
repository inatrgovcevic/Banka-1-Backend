package otc

import (
	"context"
	"time"

	"banka1/trading-service-go/internal/api"
)

// HistoryForUser mirrors OtcNegotiationHistoryService.historyForUser: resolves the
// date filters the way Java does (dateFrom → start of that day; dateTo → start of
// the following day, so the upper bound is exclusive of the day after) and maps the
// rows to the response DTO. Sorted changed_at DESC by the repository.
func (s *Service) HistoryForUser(ctx context.Context, userID int64, status *string, otherPartyID *int64, dateFrom, dateTo *time.Time) ([]OtcNegotiationHistoryResponse, error) {
	var from, to *time.Time
	if dateFrom != nil {
		f := truncateToDate(*dateFrom)
		from = &f
	}
	if dateTo != nil {
		t := truncateToDate(*dateTo).AddDate(0, 0, 1)
		to = &t
	}
	rows, err := s.repo.HistoryForUser(ctx, userID, status, otherPartyID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]OtcNegotiationHistoryResponse, 0, len(rows))
	for i := range rows {
		out = append(out, toHistoryResponse(&rows[i]))
	}
	return out, nil
}

func toHistoryResponse(h *NegotiationHistory) OtcNegotiationHistoryResponse {
	return OtcNegotiationHistoryResponse{
		ID:                h.ID,
		OfferID:           h.OfferID,
		BuyerID:           h.BuyerID,
		SellerID:          h.SellerID,
		ActorID:           h.ActorID,
		ActorName:         h.ActorName,
		EventType:         h.EventType,
		StockTicker:       h.StockTicker,
		OldAmount:         h.OldAmount,
		NewAmount:         h.NewAmount,
		OldPricePerStock:  h.OldPricePerStock,
		NewPricePerStock:  h.NewPricePerStock,
		OldPremium:        h.OldPremium,
		NewPremium:        h.NewPremium,
		OldSettlementDate: api.LocalDateFromPtr(h.OldSettlementDate),
		NewSettlementDate: api.LocalDateFromPtr(h.NewSettlementDate),
		OldStatus:         h.OldStatus,
		NewStatus:         h.NewStatus,
		ChangedAt:         api.NewLocalDateTime(h.ChangedAt),
	}
}
