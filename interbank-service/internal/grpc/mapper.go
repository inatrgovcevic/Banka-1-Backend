package grpc

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	commonv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/common/v1"
	interbankv1 "github.com/raf-si-2025/banka-1-go/proto/banka1/interbank/v1"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/protocol"
	"github.com/raf-si-2025/banka-1-go/interbank-service/internal/service"
)

// ---------------------------------------------------------------------------
// proto → domain
// ---------------------------------------------------------------------------

// mapProtoForeignBankId converts a proto *commonv1.ForeignBankId to the domain
// ForeignBankId. Returns zero-value when p is nil.
func mapProtoForeignBankId(p *commonv1.ForeignBankId) protocol.ForeignBankId {
	if p == nil {
		return protocol.ForeignBankId{}
	}
	return protocol.ForeignBankId{
		RoutingNumber: int(p.GetRoutingNumber()),
		Id:            p.GetId(),
	}
}

// mapProtoMonetaryValue converts a proto *commonv1.MonetaryValue. Returns zero
// MonetaryValue (empty currency, zero amount) when p is nil.
func mapProtoMonetaryValue(p *commonv1.MonetaryValue) (protocol.MonetaryValue, error) {
	if p == nil {
		return protocol.MonetaryValue{}, nil
	}
	amt, err := decimal.NewFromString(p.GetAmount())
	if err != nil {
		return protocol.MonetaryValue{}, fmt.Errorf("invalid monetary amount %q: %w", p.GetAmount(), err)
	}
	return protocol.MonetaryValue{Currency: p.GetCurrency(), Amount: amt}, nil
}

// mapProtoTxAccount dispatches the oneof body to the domain TxAccount interface.
func mapProtoTxAccount(p *interbankv1.TxAccount) (protocol.TxAccount, error) {
	if p == nil {
		return nil, errors.New("nil TxAccount")
	}
	switch body := p.GetBody().(type) {
	case *interbankv1.TxAccount_Person:
		return &protocol.PersonAccount{Id: mapProtoForeignBankId(body.Person)}, nil
	case *interbankv1.TxAccount_AccountNum:
		return &protocol.RealAccount{Num: body.AccountNum}, nil
	case *interbankv1.TxAccount_Option:
		return &protocol.OptionPseudoAccount{Id: mapProtoForeignBankId(body.Option)}, nil
	default:
		return nil, fmt.Errorf("unknown TxAccount oneof %T", body)
	}
}

// mapProtoAsset dispatches the oneof body to the domain Asset interface.
func mapProtoAsset(p *interbankv1.Asset) (protocol.Asset, error) {
	if p == nil {
		return nil, errors.New("nil Asset")
	}
	switch body := p.GetBody().(type) {
	case *interbankv1.Asset_Monas:
		mv := body.Monas
		if mv == nil {
			return nil, errors.New("nil MonetaryValue in Monas asset")
		}
		return &protocol.MonasAsset{Currency: mv.GetCurrency()}, nil

	case *interbankv1.Asset_Stock:
		sd := body.Stock
		if sd == nil {
			return nil, errors.New("nil StockDescription in Stock asset")
		}
		return &protocol.StockAsset{Ticker: sd.GetTicker()}, nil

	case *interbankv1.Asset_Option:
		od := body.Option
		if od == nil {
			return nil, errors.New("nil OptionDescription in Option asset")
		}
		pricePerUnit, err := mapProtoMonetaryValue(od.GetPricePerUnit())
		if err != nil {
			return nil, fmt.Errorf("option asset pricePerUnit: %w", err)
		}
		var stockTicker string
		if od.GetStock() != nil {
			stockTicker = od.GetStock().GetTicker()
		}
		return &protocol.OptionAsset{OptionDescription: protocol.OptionDescription{
			NegotiationId:  mapProtoForeignBankId(od.GetNegotiationId()),
			Stock:          protocol.StockDescription{Ticker: stockTicker},
			PricePerUnit:   pricePerUnit,
			SettlementDate: od.GetSettlementDate(),
			Amount:         int(od.GetAmount()),
		}}, nil

	default:
		return nil, fmt.Errorf("unknown Asset oneof %T", body)
	}
}

// mapProtoPosting converts a proto *interbankv1.Posting to the domain Posting.
func mapProtoPosting(p *interbankv1.Posting) (protocol.Posting, error) {
	if p == nil {
		return protocol.Posting{}, errors.New("nil Posting")
	}
	amt, err := decimal.NewFromString(p.GetAmount())
	if err != nil {
		return protocol.Posting{}, fmt.Errorf("invalid posting amount %q: %w", p.GetAmount(), err)
	}
	acc, err := mapProtoTxAccount(p.GetAccount())
	if err != nil {
		return protocol.Posting{}, fmt.Errorf("posting account: %w", err)
	}
	asset, err := mapProtoAsset(p.GetAsset())
	if err != nil {
		return protocol.Posting{}, fmt.Errorf("posting asset: %w", err)
	}
	return protocol.Posting{Account: acc, Amount: amt, Asset: asset}, nil
}

// mapProtoTx converts a proto *interbankv1.InterbankTransactionPayload to the
// domain InterbankTransactionPayload.
func mapProtoTx(p *interbankv1.InterbankTransactionPayload) (protocol.InterbankTransactionPayload, error) {
	if p == nil {
		return protocol.InterbankTransactionPayload{}, errors.New("nil InterbankTransactionPayload")
	}
	out := protocol.InterbankTransactionPayload{
		TransactionId:  mapProtoForeignBankId(p.GetTransactionId()),
		Message:        p.GetMessage(),
		CallNumber:     p.GetCallNumber(),
		PaymentCode:    p.GetPaymentCode(),
		PaymentPurpose: p.GetPaymentPurpose(),
	}
	for i, pp := range p.GetPostings() {
		posting, err := mapProtoPosting(pp)
		if err != nil {
			return out, fmt.Errorf("posting[%d]: %w", i, err)
		}
		out.Postings = append(out.Postings, posting)
	}
	return out, nil
}

// mapProtoOtcOffer converts a proto CreateNegotiationRequest or PutCounterRequest
// to an OtcOfferDto. Since both protos share the same fields, they are mapped
// via explicit parameter extraction.
func mapProtoOtcOfferFromCreate(req *interbankv1.CreateNegotiationRequest) (service.OtcOfferDto, error) {
	settleDate, err := time.Parse(time.RFC3339, req.GetSettlementDate())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("invalid settlementDate %q: %w", req.GetSettlementDate(), err)
	}
	pricePerUnit, err := mapProtoMonetaryValue(req.GetPricePerUnit())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("pricePerUnit: %w", err)
	}
	premium, err := mapProtoMonetaryValue(req.GetPremium())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("premium: %w", err)
	}
	var stockTicker string
	if req.GetStockDescription() != nil {
		stockTicker = req.GetStockDescription().GetTicker()
	}
	return service.OtcOfferDto{
		Stock:          protocol.StockDescription{Ticker: stockTicker},
		SettlementDate: settleDate,
		PricePerUnit:   pricePerUnit,
		Premium:        premium,
		BuyerID:        mapProtoForeignBankId(req.GetBuyerId()),
		SellerID:       mapProtoForeignBankId(req.GetSellerId()),
		Amount:         int(req.GetAmount()),
		LastModifiedBy: mapProtoForeignBankId(req.GetLastModifiedBy()),
	}, nil
}

func mapProtoOtcOfferFromPut(req *interbankv1.PutCounterRequest) (service.OtcOfferDto, error) {
	settleDate, err := time.Parse(time.RFC3339, req.GetSettlementDate())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("invalid settlementDate %q: %w", req.GetSettlementDate(), err)
	}
	pricePerUnit, err := mapProtoMonetaryValue(req.GetPricePerUnit())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("pricePerUnit: %w", err)
	}
	premium, err := mapProtoMonetaryValue(req.GetPremium())
	if err != nil {
		return service.OtcOfferDto{}, fmt.Errorf("premium: %w", err)
	}
	var stockTicker string
	if req.GetStockDescription() != nil {
		stockTicker = req.GetStockDescription().GetTicker()
	}
	return service.OtcOfferDto{
		Stock:          protocol.StockDescription{Ticker: stockTicker},
		SettlementDate: settleDate,
		PricePerUnit:   pricePerUnit,
		Premium:        premium,
		BuyerID:        mapProtoForeignBankId(req.GetBuyerId()),
		SellerID:       mapProtoForeignBankId(req.GetSellerId()),
		Amount:         int(req.GetAmount()),
		LastModifiedBy: mapProtoForeignBankId(req.GetLastModifiedBy()),
	}, nil
}

// ---------------------------------------------------------------------------
// domain → proto
// ---------------------------------------------------------------------------

// mapForeignBankIdToProto converts a domain ForeignBankId to the proto type.
func mapForeignBankIdToProto(f protocol.ForeignBankId) *commonv1.ForeignBankId {
	return &commonv1.ForeignBankId{
		RoutingNumber: int32(f.RoutingNumber),
		Id:            f.Id,
	}
}

// mapMonetaryValueToProto converts a domain MonetaryValue to the proto type.
func mapMonetaryValueToProto(mv protocol.MonetaryValue) *commonv1.MonetaryValue {
	return &commonv1.MonetaryValue{
		Currency: mv.Currency,
		Amount:   mv.Amount.String(),
	}
}

// mapVoteToProto converts a domain TransactionVote to the proto type.
func mapVoteToProto(v protocol.TransactionVote) *interbankv1.TransactionVote {
	out := &interbankv1.TransactionVote{}
	switch v.Vote {
	case protocol.VoteYes:
		out.Vote = interbankv1.TransactionVote_VOTE_YES
	case protocol.VoteNo:
		out.Vote = interbankv1.TransactionVote_VOTE_NO
	default:
		out.Vote = interbankv1.TransactionVote_VOTE_UNSPECIFIED
	}
	for _, r := range v.Reasons {
		out.Reasons = append(out.Reasons, mapNoVoteReasonToProto(r))
	}
	return out
}

// mapNoVoteReasonToProto converts a domain NoVoteReason to the proto type.
func mapNoVoteReasonToProto(r protocol.NoVoteReason) *interbankv1.NoVoteReason {
	out := &interbankv1.NoVoteReason{
		Reason: mapReasonStringToProto(r.Reason),
	}
	if r.Posting != nil {
		out.Posting = mapPostingToProto(*r.Posting)
	}
	return out
}

// mapReasonStringToProto maps a reason string constant to the proto enum.
func mapReasonStringToProto(r string) interbankv1.NoVoteReason_Reason {
	switch r {
	case protocol.ReasonUnbalancedTx:
		return interbankv1.NoVoteReason_REASON_UNBALANCED_TX
	case protocol.ReasonNoSuchAccount:
		return interbankv1.NoVoteReason_REASON_NO_SUCH_ACCOUNT
	case protocol.ReasonNoSuchAsset:
		return interbankv1.NoVoteReason_REASON_NO_SUCH_ASSET
	case protocol.ReasonUnacceptableAsset:
		return interbankv1.NoVoteReason_REASON_UNACCEPTABLE_ASSET
	case protocol.ReasonInsufficientAsset:
		return interbankv1.NoVoteReason_REASON_INSUFFICIENT_ASSET
	case protocol.ReasonOptionAmountIncorrect:
		return interbankv1.NoVoteReason_REASON_OPTION_AMOUNT_INCORRECT
	case protocol.ReasonOptionUsedOrExpired:
		return interbankv1.NoVoteReason_REASON_OPTION_USED_OR_EXPIRED
	case protocol.ReasonOptionNegotiationNotFound:
		return interbankv1.NoVoteReason_REASON_OPTION_NEGOTIATION_NOT_FOUND
	default:
		return interbankv1.NoVoteReason_REASON_UNSPECIFIED
	}
}

// mapTxAccountToProto converts a domain TxAccount to the proto type.
func mapTxAccountToProto(acc protocol.TxAccount) *interbankv1.TxAccount {
	switch a := acc.(type) {
	case *protocol.PersonAccount:
		return &interbankv1.TxAccount{
			Body: &interbankv1.TxAccount_Person{
				Person: mapForeignBankIdToProto(a.Id),
			},
		}
	case *protocol.RealAccount:
		return &interbankv1.TxAccount{
			Body: &interbankv1.TxAccount_AccountNum{
				AccountNum: a.Num,
			},
		}
	case *protocol.OptionPseudoAccount:
		return &interbankv1.TxAccount{
			Body: &interbankv1.TxAccount_Option{
				Option: mapForeignBankIdToProto(a.Id),
			},
		}
	default:
		return &interbankv1.TxAccount{}
	}
}

// mapAssetToProto converts a domain Asset to the proto type.
func mapAssetToProto(asset protocol.Asset) *interbankv1.Asset {
	switch a := asset.(type) {
	case *protocol.MonasAsset:
		return &interbankv1.Asset{
			Body: &interbankv1.Asset_Monas{
				Monas: &commonv1.MonetaryValue{Currency: a.Currency, Amount: "0"},
			},
		}
	case *protocol.StockAsset:
		return &interbankv1.Asset{
			Body: &interbankv1.Asset_Stock{
				Stock: &commonv1.StockDescription{Ticker: a.Ticker},
			},
		}
	case *protocol.OptionAsset:
		od := a.OptionDescription
		return &interbankv1.Asset{
			Body: &interbankv1.Asset_Option{
				Option: &commonv1.OptionDescription{
					NegotiationId:  mapForeignBankIdToProto(od.NegotiationId),
					Stock:          &commonv1.StockDescription{Ticker: od.Stock.Ticker},
					PricePerUnit:   mapMonetaryValueToProto(od.PricePerUnit),
					SettlementDate: od.SettlementDate,
					Amount:         int32(od.Amount),
				},
			},
		}
	default:
		return &interbankv1.Asset{}
	}
}

// mapPostingToProto converts a domain Posting to the proto type.
func mapPostingToProto(p protocol.Posting) *interbankv1.Posting {
	return &interbankv1.Posting{
		Account: mapTxAccountToProto(p.Account),
		Amount:  p.Amount.String(),
		Asset:   mapAssetToProto(p.Asset),
	}
}

// mapNegotiationDtoToProto converts a domain OtcNegotiationDto to the proto
// Negotiation message (used in GetNegotiation response).
func mapNegotiationDtoToProto(id protocol.ForeignBankId, dto service.OtcNegotiationDto) *interbankv1.Negotiation {
	return &interbankv1.Negotiation{
		Id:       mapForeignBankIdToProto(id),
		BuyerId:  mapForeignBankIdToProto(dto.BuyerID),
		SellerId: mapForeignBankIdToProto(dto.SellerID),
		LastModifiedBy: mapForeignBankIdToProto(dto.LastModifiedBy),
		StockDescription: &commonv1.StockDescription{Ticker: dto.Stock.Ticker},
		PricePerUnit:     mapMonetaryValueToProto(dto.PricePerUnit),
		Amount:           int32(dto.Amount),
		SettlementDate:   dto.SettlementDate.UTC().Format(time.RFC3339),
		Premium:          mapMonetaryValueToProto(dto.Premium),
		IsOngoing:        dto.IsOngoing,
	}
}
