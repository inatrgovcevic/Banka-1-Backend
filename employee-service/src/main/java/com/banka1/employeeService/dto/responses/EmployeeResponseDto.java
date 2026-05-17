package com.banka1.employeeService.dto.responses;

import com.banka1.employeeService.domain.enums.Pol;
import com.banka1.employeeService.domain.enums.Role;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;

import java.time.LocalDate;

/**
 * DTO koji predstavlja podatke o zaposlenom koji se vracaju klijentu.
 * Ne sadrzi osetljive podatke poput lozinke ili tokena.
 */
@Getter
@Setter
@AllArgsConstructor
public class EmployeeResponseDto {

    /** Identifikator zaposlenog. */
    private Long id;

    /** Ime zaposlenog. */
    private String ime;

    /** Prezime zaposlenog. */
    private String prezime;

    /** Email adresa zaposlenog. */
    private String email;

    /** Korisnicko ime zaposlenog. */
    private String username;

    /** Datum rodjenja. */
    private LocalDate datumRodjenja;

    /** Pol. */
    private Pol pol;

    /** Broj telefona. */
    private String brojTelefona;

    /** Adresa stanovanja. */
    private String adresa;

    /** Pozicija (radno mesto) zaposlenog. */
    private String pozicija;

    /** Departman u kome zaposleni radi. */
    private String departman;

    /** Indikator da li je nalog zaposlenog aktivan. */
    private boolean aktivan;

    /** RBAC uloga zaposlenog. */
    private Role role;
}
