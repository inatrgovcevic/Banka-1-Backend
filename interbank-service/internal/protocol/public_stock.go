package protocol

import "github.com/shopspring/decimal"

// PublicStockEntry represents one row from GET /public-stock (§3.1).
// Mirrors Java PublicStockEntryDto.
type PublicStockEntry struct {
	Stock   StockDescription       `json:"stock"`
	Sellers []PublicStockSellerRef `json:"sellers"`
}

// PublicStockSellerRef identifies one seller of a public stock.
//
// Wire keys are "seller" + "amount" — matching Banka 2's PublicStock.Seller
// record (ForeignBankId seller, BigDecimal amount) AND Banka 1's own live
// /public-stock handler (api.SellerRow Seller/Amount). The previous tags
// "sellerId"/"quantity" disagreed with both, so decoding a partner's real
// /public-stock response (OutboundFetchPublicStock) silently produced
// zero-value sellers.
//
// Quantity is decimal.Decimal (was int): Banka 2 serializes the available
// amount as a BigDecimal and its live response contains scaled values like
// 1.0000, which a Go int CANNOT decode ("cannot unmarshal number 1.0000 into
// Go value of type int") — that would fail the whole /public-stock decode and
// drop every seller. decimal.Decimal accepts 1.0000, 3, "5" etc. gracefully.
type PublicStockSellerRef struct {
	SellerID ForeignBankId   `json:"seller"`
	Quantity decimal.Decimal `json:"amount"`
}
