package com.banka1.tradingservice.otc.exception;

import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.ResponseStatus;

/**
 * PR_32 Phase 12 KRIT #3: baca se kad reserved-stock invariant ne prolazi —
 * prodavac pokusava da prihvati ponudu ciji bi prihvat povecao sumu
 * angazovanih akcija (PENDING_PREMIUM + ACTIVE OptionContract.amount + nova
 * ponuda) iznad kolicine akcija u portfoliu za taj ticker.
 *
 * <p>Mapira se na HTTP 400 (Bad Request) jer je problem na strani klijenta
 * (prodavac vec ima previse otvorenih obaveza).
 */
@ResponseStatus(HttpStatus.BAD_REQUEST)
public class InsufficientPublicStockException extends RuntimeException {

    public InsufficientPublicStockException(String message) {
        super(message);
    }
}
