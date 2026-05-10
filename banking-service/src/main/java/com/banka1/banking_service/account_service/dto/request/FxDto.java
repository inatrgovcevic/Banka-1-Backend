package com.banka1.banking_service.account_service.dto.request;

import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.dto.request.FirmaDto;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO za zahtev kreiranja deviznog (FX) racuna.
 * <p>
 * Sadrzi sve podatke potrebne za kreiranje novog deviznog racuna
 * u bilo kojoj valuti osim RSD.
 * <p>
 * Validacija:
 * <ul>
 *   <li>Naziv racuna mora biti popunjen i razlicit od RSD</li>
 *   <li>Ili ID vlasnika ili JMBG mora biti postavljen</li>
 *   <li>Tip racuna (PERSONAL ili BUSINESS) mora biti validan</li>
 *   <li>Valuta ne sme biti RSD</li>
 *   <li>Pocetno stanje mora biti postavljeno</li>
 *   <li>Flegom za kreiranje kartice mora biti postavljen</li>
 * </ul>
 */
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class FxDto {
    /**
     * Naziv koji je korisnik dao za svoj devizni racun.
     */
    @NotBlank(message = "Ne sme biti prazan naziv racuna")
    private String nazivRacuna;

    /**
     * ID korisnika iz korisnickog servisa. Alternativa za JMBG.
     */
    private Long idVlasnika;

    /**
     * JMBG (jedinstveni maticni broj gradjanina) korisnika. Alternativa za ID.
     */
    private String jmbg;

    /**
     * Kod valute (npr. USD, EUR, GBP). Ne sme biti RSD.
     */
    @NotNull(message = "Unesi valutu")
    private CurrencyCode currencyCode;

    /**
     * Tip vlasnistva racuna (PERSONAL za licne, BUSINESS za poslovne).
     */
    @NotNull(message = "Unesi tip racuna")
    private AccountOwnershipType tipRacuna;

    /**
     * Pocetno stanje racuna u zadatoj valuti.
     */
    @NotNull(message = "Unesi initialBalance")
    private BigDecimal initialBalance;

    /**
     * Da li da se kreira kartice za ovaj racun odmah pri kreiranju.
     */
    @NotNull(message = "Unesi createCard")
    private Boolean createCard;

    /**
     * Podaci o firmi, popunjava se samo ako je tip racuna BUSINESS.
     */
    private FirmaDto firma;

}
