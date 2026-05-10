package com.banka1.banking_service.account_service.dto.request;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO za zahtev promene naziva bankarskog računa.
 * <p>
 * Omogućava vlasnicima računa da promene naziv svog računa na nešto što je lakše
 * za pamćenje i organizovanje. Novi naziv se primenjuje odmah nakon validacije i
 * sprema u bazu podataka.
 * <p>
 * Validacija:
 * <ul>
 *   <li>Naziv mora biti popunjen (ne sme biti prazan)</li>
 *   <li>Naziv mora imati između 3 i 50 karaktera</li>
 *   <li>Naziv mora biti jedinstven za tog vlasnika (nema duplikata)</li>
 * </ul>
 * <p>
 * Primeri validnih naziva: "Moj tekući račun", "Savings", "Holiday Fund 2024".
 */
@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
public class EditAccountNameDto {
    /**
     * Novi naziv za racun.
     * <p>
     * Mora biti drugačiji od trenutnog naziva i ne sme se
     * podudarati sa nazivom drugog racuna istog vlasnika.
     */
    @NotBlank(message = "Unesi accountName")
    @Size(min = 3, max = 50)
    private String accountName;
}
