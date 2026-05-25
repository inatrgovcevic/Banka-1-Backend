package protocol

// PublicStockEntry represents one row from GET /public-stock (§3.1).
// Mirrors Java PublicStockEntryDto.
type PublicStockEntry struct {
	Stock   StockDescription        `json:"stock"`
	Sellers []PublicStockSellerRef  `json:"sellers"`
}

// PublicStockSellerRef identifies one seller of a public stock.
// Mirrors Java PublicStockSellerDto.
type PublicStockSellerRef struct {
	SellerID ForeignBankId `json:"sellerId"`
	Quantity int           `json:"quantity"`
}
