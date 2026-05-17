package com.banka1.employeeService.domain;

import com.banka1.employeeService.domain.enums.Permission;
import com.banka1.employeeService.domain.enums.Pol;
import com.banka1.employeeService.domain.enums.Role;
import jakarta.persistence.*;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.hibernate.annotations.SQLDelete;
import org.hibernate.annotations.SQLRestriction;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.HashSet;
import java.util.Set;

/**
 * JPA entitet koji predstavlja zaposlenog u banci.
 * Implementira soft delete – fizicko brisanje je zamenjeno postavljanjem zastavice {@code deleted = true},
 * a {@code @SQLRestriction} automatski iskljucuje obrisane redove iz svih upita.
 */
@Entity
@Table(
        name = "employees",
        indexes = {
                @Index(name = "idx_employees_ime_prezime", columnList = "ime, prezime"),
                @Index(name = "idx_employees_pozicija", columnList = "pozicija")
        }
)
@SQLDelete(sql = "UPDATE employees SET deleted = true WHERE id = ? AND version = ?")
@SQLRestriction("deleted = false")
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class Zaposlen extends BaseEntity {

    /**
     * Ime zaposlenog.
     */
    @NotBlank
    @Column(nullable = false)
    private String ime;

    /**
     * Prezime zaposlenog.
     */
    @NotBlank
    @Column(nullable = false)
    private String prezime;

    /**
     * Datum rodjenja zaposlenog.
     */
    @Column(nullable = false)
    private LocalDate datumRodjenja;

    /**
     * Pol zaposlenog (M ili Z).
     */
    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private Pol pol;

    /**
     * Email adresa zaposlenog – mora biti jedinstvena u sistemu.
     */
    @NotBlank
    @Email
    @Column(nullable = false, unique = true)
    private String email;

    /**
     * Broj telefona zaposlenog (opcioni).
     */
    private String brojTelefona;

    /**
     * Adresa stanovanja zaposlenog (opciona).
     */
    private String adresa;

    /**
     * Korisnicko ime – mora biti jedinstveno u sistemu.
     */
    @NotBlank
    @Column(nullable = false, unique = true)
    private String username;

    /**
     * BCrypt/Argon2 hash lozinke zaposlenog.
     */
    private String password;

    /**
     * Pozicija (radno mesto) zaposlenog.
     */
    @NotBlank
    @Column(nullable = false)
    private String pozicija;

    /**
     * Departman u kome zaposleni radi.
     */
    @NotBlank
    @Column(nullable = false)
    private String departman;

    /**
     * Indikator da li je nalog zaposlenog aktivan.
     * Default je {@code true} po specifikaciji (Celina 1, "Po default-u se podrazumeva da je
     * zaposleni aktivan"). Admin moze eksplicitno kreirati neaktivnog zaposlenog kroz DTO flag.
     */
    private boolean aktivan = true;

    /**
     * Brojac neuspesnih pokusaja prijave; resetuje se na 0 nakon uspesne prijave.
     * Koristi se za zakljucavanje naloga posle vise neuspeha (Celina 1, Scenario 5).
     */
    @Column(nullable = false)
    private int failedLoginAttempts = 0;

    /**
     * Trenutak do kog je nalog zakljucan zbog vise neuspesnih pokusaja prijave;
     * {@code null} kada nalog nije zakljucan.
     */
    private LocalDateTime lockedUntil;

    /**
     * RBAC uloga zaposlenog koja odredjuje nivo pristupa.
     */
    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private Role role;

    /**
     * Aktivacioni / reset token vezan za zaposlenog (opcioni).
     */
    @OneToOne(mappedBy = "zaposlen", cascade = CascadeType.ALL, orphanRemoval = true)
    private ConfirmationToken confirmationToken;

    /**
     * Skup pojedinacnih permisija dodeljenih zaposlenom na osnovu njegove uloge.
     */
    @ElementCollection(targetClass = Permission.class)
    @CollectionTable(
            name = "zaposlen_permissions",
            joinColumns = @JoinColumn(name = "zaposlen_id")
    )
    @Column(name = "permission", nullable = false)
    @Enumerated(EnumType.STRING)
    private Set<Permission> permissionSet = new HashSet<>();

    @Override
    public String toString() {
        return "Zaposlen{" +
                "ime='" + ime + '\'' +
                ", prezime='" + prezime + '\'' +
                ", datumRodjenja=" + datumRodjenja +
                ", pol=" + pol +
                ", email='" + email + '\'' +
                ", brojTelefona='" + brojTelefona + '\'' +
                ", adresa='" + adresa + '\'' +
                ", username='" + username + '\'' +
                ", password='" + password + '\'' +
                ", pozicija='" + pozicija + '\'' +
                ", departman='" + departman + '\'' +
                ", aktivan=" + aktivan +
                ", role=" + role +
                ", confirmationToken=" + confirmationToken +
                ", permissionSet=" + permissionSet +
                '}';
    }
}
