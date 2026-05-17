package com.banka1.interbank.otc.dto;

import com.banka1.interbank.protocol.dto.StockDescription;
import java.util.List;

/**
 * PR_32 Phase 5 (stub za Phase 10): jedan red u public-stock listi.
 *
 * <p>Predstavlja jedan ticker zajedno sa svim prodavcima koji nude akcije
 * tog ticker-a. Tim 2 §11 publicira ovu strukturu kroz
 * {@code GET /public-stock} endpoint.
 *
 * @param stock   opis hartije (ticker)
 * @param sellers lista prodavaca i njihovih kolicina
 */
public record PublicStockEntryDto(StockDescription stock, List<PublicStockSellerDto> sellers) {}
