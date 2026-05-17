package com.banka1.account_service.dto.request;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/**
 * Request payload za jednostranu trade-leg operaciju (debit/credit) nad jednim
 * racunom (GHI #199, primarni bug 2).
 *
 * <p>Po PM direktivi <em>,,NE DAJE BANCI PARE, samo se skidaju sa racuna''</em>:
 * trade leg klijentskog BUY/SELL ne sme da prolazi kroz bankin racun. Ranije je
 * orchestrator zvao {@code accountClient.transaction(...)} sa korisnikom kao
 * {@code from} i bankom kao {@code to}, sto je celokupan iznos kupovine
 * kreditiralo banci. Novi endpointi {@code /internal/accounts/exchange/buy} i
 * {@code .../exchange/sell} izvrsavaju jednostrani debit odnosno credit nad
 * korisnikovim racunom; bankin racun se ne dira (osim provizije, koja i dalje
 * ide kroz {@code OrderCreationServiceImpl.transferFee}).
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
@JsonInclude(JsonInclude.Include.NON_NULL)
public class OneSidedTransactionDto {

    /** Broj racuna (18 cifara) ili PK racuna; servis prihvata oba. */
    private String accountNumber;

    /** ID racuna (PK iz baze). Alternativa za {@code accountNumber}. */
    private Long accountId;

    /** Iznos za debit/credit. Mora biti pozitivan. */
    private BigDecimal amount;

    /** Klijent koji inicira (audit/log). */
    private Long clientId;

    /** Opis transakcije za audit. */
    private String description;
}
