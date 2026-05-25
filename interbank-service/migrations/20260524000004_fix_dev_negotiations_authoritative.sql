-- +goose Up
-- +goose StatementBegin
-- PR_33 follow-up bugfix: dev seed 003 postavio je is_authoritative=true za sve
-- cross-bank pregovore, ali Banka 2 (buyer_routing_number=222) ih je inicirala
-- → buyer bank je authoritative per protokol §3.2. Pravilan flag: false.

UPDATE interbank_negotiations
SET is_authoritative = false
WHERE buyer_routing_number = 222 AND seller_routing_number = 111;

UPDATE interbank_negotiations
SET is_authoritative = false
WHERE id IN ('neg-handshake-s9', 'neg-handshake-s13');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE interbank_negotiations
SET is_authoritative = true
WHERE buyer_routing_number = 222 AND seller_routing_number = 111;
-- +goose StatementEnd
