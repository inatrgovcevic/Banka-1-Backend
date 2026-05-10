package com.banka1.banking_service.account_service.dto.request;

import com.banka1.banking_service.account_service.domain.enums.AccountConcrete;
import com.banka1.banking_service.account_service.dto.request.FirmaDto;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO za zahtev kreiranja tekuceg racuna.
 * <p>
 * Sadrzi sve podatke potrebne za kreiranje novog tekuceg (checking) racuna
 * koji je denominiran u RSD valuti.
 * <p>
 * Validacija:
 * <ul>
 *   <li>Naziv racuna mora biti popunjen</li>
 *   <li>Ili ID vlasnika ili JMBG mora biti postavljen (ne oba niti oba prazna)</li>
 *   <li>Tip racuna mora biti validan enum</li>
 *   <li>Pocetno stanje (inicijalni balans) mora biti postavljen</li>
 *   <li>Flegom za kreiranje kartice mora biti postavljen</li>
 * </ul>
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
//todo proveriti sta cemo za odrzavanje racuna
public class CheckingDto {

    /**
     * Naziv koji je korisnik dao za svoj tekuci racun.
     * <p>
     * Mora biti jedinstven za tog vlasnika (ne sme imati dva racuna sa
     * istim nazivom).
     */
    @NotBlank(message = "Unesi naziv racuna")
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
     * Vrsta/tip tekuceg racuna (licni, stedni, poslovni, itd.).
     */
    @NotNull(message = "Unesi podvrstu racuna")
    private AccountConcrete vrstaRacuna;

    /**
     * Podaci o firmi, popunjava se samo ako je tip racuna BUSINESS.
     */
    private FirmaDto firma;

    /**
     * Pocetno stanje racuna u RSD.
     */
    @NotNull(message = "Unesi initialBalance")
    private BigDecimal initialBalance;

    /**
     * Da li da se kreira kartice za ovaj racun odmah pri kreiranju.
     */
    @NotNull(message = "Unesi createCard")
    private Boolean createCard;
}
