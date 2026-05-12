package com.banka1.employeeService.dto.requests;

import com.banka1.employeeService.domain.enums.Pol;
import com.banka1.employeeService.domain.enums.Role;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.Data;

import java.time.LocalDate;

/**
 * DTO za zahtev kreiranja novog zaposlenog.
 * Sva polja su obavezna osim {@code brojTelefona} i {@code adresa}.
 */
@Data
public class EmployeeCreateRequestDto {

    /** Ime zaposlenog. */
    @NotBlank(message = "Ime je obavezno")
    private String ime;

    /** Prezime zaposlenog. */
    @NotBlank(message = "Prezime je obavezno")
    private String prezime;

    /** Datum rodjenja zaposlenog. */
    @NotNull(message = "Datum rođenja je obavezan")
    private LocalDate datumRodjenja;

    /** Pol zaposlenog. */
    @NotNull(message = "Pol je obavezan")
    private Pol pol;

    /** Email adresa zaposlenog. */
    @Email(message = "Nevalidan format email-a")
    @NotBlank(message = "Email je obavezan")
    private String email;

    /** Broj telefona zaposlenog (opcioni, u internacionalnom formatu). */
    @Pattern(
            regexp = "^\\+?[0-9]{8,15}$",
            message = "Neispravan broj telefona"
    )
    private String brojTelefona;

    /** Adresa stanovanja zaposlenog (opciona). */
    private String adresa;

    /** Korisnicko ime zaposlenog (opcioni; ako izostane, izvodi se iz email-a). */
    private String username;

    /** Pozicija (radno mesto) zaposlenog. */
    @NotBlank(message = "Pozicija je obavezna")
    private String pozicija;

    /** Departman u kome ce zaposleni raditi. */
    @NotBlank(message = "Departman je obavezan")
    private String departman;

    /** RBAC uloga koja se dodeljuje zaposlenom. */
    @NotNull(message = "Uloga (Role) je obavezna")
    private Role role;

    /**
     * Indikator da li nalog odmah postaje aktivan; ako je {@code null}, podrazumeva se {@code true}
     * (Celina 1: "Po default-u se podrazumeva da je zaposleni aktivan, ali moguce je napraviti i
     * zaposlenog koji nije aktivan").
     */
    private Boolean aktivan;
}
