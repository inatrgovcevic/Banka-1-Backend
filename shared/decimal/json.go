// Package decimalx asserts that github.com/shopspring/decimal serializes JSON in
// a format compatible with Java's BigDecimal under WRITE_BIGDECIMAL_AS_PLAIN:
// plain decimal string, no scientific notation. This file is intentionally
// near-empty — the test suite (json_test.go) is the contract that the dependency
// continues to honor this behavior across upgrades.
//
// The package is named decimalx (not decimal) to avoid shadowing the upstream
// library at import sites.
package decimalx
