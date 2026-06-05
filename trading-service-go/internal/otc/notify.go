package otc

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"banka1/trading-service-go/internal/clients"

	"banka1/go-platform/rabbitmq"
)

// OTC notification routing keys — mirror OtcNotificationService constants. They
// land on the NOTIFICATION_EXCHANGE (employee.events) the publisher is bound to;
// notification-service binds to them.
const (
	routingOtcCountered = "otc.countered"
	routingOtcAccepted  = "otc.accepted"
	routingOtcCanceled  = "otc.canceled"
	routingOtcExpiry    = "otc.expiry_reminder"
	routingOtcExpired   = "otc.expired"
)

// OtcNotifier publishes the OTC user-facing notifications. The service calls it
// AFTER the transition transaction commits (best-effort; Java sends inline within
// the @Transactional method — a rolled-back transition therefore notifies in Java
// but not here, which is strictly safer and notifications are not parity-gated).
type OtcNotifier interface {
	CounterOffered(ctx context.Context, offer *OtcOffer, actorID int64)
	Accepted(ctx context.Context, offer *OtcOffer, actorID int64)
	Canceled(ctx context.Context, offer *OtcOffer, actorID int64, eventType string)
	ExpiryReminder(ctx context.Context, contract *OptionContract, reminderDays int)
	ContractExpired(ctx context.Context, contract *OptionContract)
}

// notificationRequest mirrors TradingNotificationProducer.TradingNotificationRequest
// {username, userEmail, templateVariables}.
type notificationRequest struct {
	Username          string            `json:"username"`
	UserEmail         string            `json:"userEmail"`
	TemplateVariables map[string]string `json:"templateVariables"`
}

// RabbitNotifier publishes over the employee.events rabbitmq.Publisher and
// resolves recipient name/email via the user-service customer client (mirrors
// OtcNotificationService.notifyRecipient → ClientClient.getCustomer).
type RabbitNotifier struct {
	pub      rabbitmq.Publisher
	customer *clients.CustomerClient
	logger   *slog.Logger
}

// NewRabbitNotifier wraps the employee.events publisher + customer client.
func NewRabbitNotifier(pub rabbitmq.Publisher, customer *clients.CustomerClient, logger *slog.Logger) *RabbitNotifier {
	return &RabbitNotifier{pub: pub, customer: customer, logger: logger}
}

func (n *RabbitNotifier) CounterOffered(ctx context.Context, offer *OtcOffer, actorID int64) {
	n.notify(ctx, counterparty(offer, actorID), routingOtcCountered, n.offerVars("COUNTER_OFFERED", offer, actorID))
}

func (n *RabbitNotifier) Accepted(ctx context.Context, offer *OtcOffer, actorID int64) {
	n.notify(ctx, counterparty(offer, actorID), routingOtcAccepted, n.offerVars("ACCEPTED", offer, actorID))
}

func (n *RabbitNotifier) Canceled(ctx context.Context, offer *OtcOffer, actorID int64, eventType string) {
	n.notify(ctx, counterparty(offer, actorID), routingOtcCanceled, n.offerVars(eventType, offer, actorID))
}

func (n *RabbitNotifier) ExpiryReminder(ctx context.Context, contract *OptionContract, reminderDays int) {
	buyerVars := n.contractVars("EXPIRY_REMINDER", contract, contract.SellerID)
	buyerVars["reminderDays"] = strconv.Itoa(reminderDays)
	n.notify(ctx, contract.BuyerID, routingOtcExpiry, buyerVars)

	sellerVars := n.contractVars("EXPIRY_REMINDER", contract, contract.BuyerID)
	sellerVars["reminderDays"] = strconv.Itoa(reminderDays)
	n.notify(ctx, contract.SellerID, routingOtcExpiry, sellerVars)
}

func (n *RabbitNotifier) ContractExpired(ctx context.Context, contract *OptionContract) {
	n.notify(ctx, contract.BuyerID, routingOtcExpired, n.contractVars("EXPIRED", contract, contract.SellerID))
	n.notify(ctx, contract.SellerID, routingOtcExpired, n.contractVars("EXPIRED", contract, contract.BuyerID))
}

// counterparty mirrors the recipient resolution: notify the party that did NOT
// act (buyer == actor → seller, else buyer).
func counterparty(offer *OtcOffer, actorID int64) int64 {
	if offer.BuyerID == actorID {
		return offer.SellerID
	}
	return offer.BuyerID
}

// offerVars mirrors baseOfferVariables. Note the deliberate Java quirk:
// counterpartyId carries the ACTOR id, otherPartyId carries the computed
// counterparty.
func (n *RabbitNotifier) offerVars(eventType string, offer *OtcOffer, actorID int64) map[string]string {
	cp := counterparty(offer, actorID)
	return map[string]string{
		"eventType":      eventType,
		"offerId":        strconv.FormatInt(offer.ID, 10),
		"contractId":     "",
		"stockTicker":    offer.StockTicker,
		"amount":         strconv.Itoa(offer.Amount),
		"pricePerStock":  offer.PricePerStock.String(),
		"premium":        offer.Premium.String(),
		"status":         offer.Status,
		"timestamp":      time.Now().Format("2006-01-02T15:04:05.999999999"),
		"expiryDate":     offer.SettlementDate.Format("2006-01-02"),
		"counterpartyId": strconv.FormatInt(actorID, 10),
		"otherPartyId":   strconv.FormatInt(cp, 10),
		"buyerId":        strconv.FormatInt(offer.BuyerID, 10),
		"sellerId":       strconv.FormatInt(offer.SellerID, 10),
	}
}

// contractVars mirrors baseContractVariables.
func (n *RabbitNotifier) contractVars(eventType string, contract *OptionContract, otherPartyID int64) map[string]string {
	return map[string]string{
		"eventType":     eventType,
		"offerId":       strconv.FormatInt(contract.OfferID, 10),
		"contractId":    strconv.FormatInt(contract.ID, 10),
		"stockTicker":   contract.StockTicker,
		"amount":        strconv.Itoa(contract.Amount),
		"pricePerStock": contract.PricePerStock.String(),
		"status":        contract.Status,
		"timestamp":     time.Now().Format("2006-01-02T15:04:05.999999999"),
		"expiryDate":    contract.SettlementDate.Format("2006-01-02"),
		"otherPartyId":  strconv.FormatInt(otherPartyID, 10),
		"buyerId":       strconv.FormatInt(contract.BuyerID, 10),
		"sellerId":      strconv.FormatInt(contract.SellerID, 10),
	}
}

// notify resolves the recipient (skip when no customer or blank email, mirroring
// notifyRecipient) and publishes the notification request. Best-effort.
func (n *RabbitNotifier) notify(ctx context.Context, clientID int64, routingKey string, vars map[string]string) {
	cust, err := n.customer.GetCustomer(ctx, clientID)
	if err != nil || cust == nil || cust.Email == nil || strings.TrimSpace(*cust.Email) == "" {
		return
	}
	name := displayName(cust)
	if err := n.pub.Publish(ctx, routingKey, notificationRequest{
		Username:          name,
		UserEmail:         *cust.Email,
		TemplateVariables: vars,
	}); err != nil {
		n.logger.Warn("otc notification publish failed", "routingKey", routingKey, "clientId", clientID, "error", err)
	}
}

func displayName(c *clients.Customer) string {
	var first, last string
	if f := c.First(); f != nil {
		first = *f
	}
	if l := c.Last(); l != nil {
		last = *l
	}
	return strings.TrimSpace(first + " " + last)
}

// NoopNotifier discards notifications — used when the broker is unreachable at
// startup (the trading flow still works; notifications are best-effort).
type NoopNotifier struct{}

func (NoopNotifier) CounterOffered(context.Context, *OtcOffer, int64)     {}
func (NoopNotifier) Accepted(context.Context, *OtcOffer, int64)           {}
func (NoopNotifier) Canceled(context.Context, *OtcOffer, int64, string)   {}
func (NoopNotifier) ExpiryReminder(context.Context, *OptionContract, int) {}
func (NoopNotifier) ContractExpired(context.Context, *OptionContract)     {}
