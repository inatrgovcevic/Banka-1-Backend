package com.banka1.employeeService.repository;

import com.banka1.employeeService.domain.Zaposlen;
import com.banka1.employeeService.domain.enums.Role;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

/**
 * Spring Data JPA repozitorijum za entitet {@link Zaposlen}.
 * Pruza standardne CRUD operacije, provere jedinstvenosti i pretrage sa paginacijom.
 * Svi upiti automatski iskljucuju soft-obrisane zapise zahvaljujuci {@code @SQLRestriction("deleted = false")}
 * na entitetu; JPQL upiti dodatno sadrze eksplicitnu proveru {@code z.deleted = false} kao odbrambenu meru.
 */
@Repository
public interface ZaposlenRepository extends JpaRepository<Zaposlen, Long> {

    /**
     * Pronalazi zaposlenog po email adresi.
     *
     * @param email email adresa zaposlenog
     * @return opcioni zaposleni ako postoji
     */
    Optional<Zaposlen> findByEmail(String email);

    List<Zaposlen> findByRole(Role role);

    /**
     * Proverava da li zaposleni sa zadatom email adresom vec postoji.
     *
     * @param email email adresa za proveru
     * @return {@code true} ako adresa vec postoji
     */
    boolean existsByEmail(String email);

    /**
     * Proverava da li zaposleni sa zadatim korisnickim imenom vec postoji.
     *
     * @param username korisnicko ime za proveru
     * @return {@code true} ako korisnicko ime vec postoji
     */
    boolean existsByUsername(String username);

    /**
     * Pronalazi zaposlenog po korisnickom imenu.
     *
     * @param username korisnicko ime zaposlenog
     * @return opcioni zaposleni ako postoji
     */
    Optional<Zaposlen> findByUsername(String username);

    /**
     * Pretrazuje zaposlene po pojedinacnim filterima uz paginaciju.
     * Svaki filter koristi case-insensitive LIKE pretragu; prazan string se ponasa kao wildcard.
     *
     * @param ime filter po imenu
     * @param prezime filter po prezimenu
     * @param email filter po email adresi
     * @param departman filter po departmanu
     * @param pozicija filter po poziciji
     * @param pageable parametri paginacije i sortiranja
     * @return stranica zaposlenih koji zadovoljavaju sve filtere
     */
    @Query("SELECT z FROM Zaposlen z WHERE " +
            "LOWER(z.ime) LIKE LOWER(CONCAT('%', :ime, '%')) AND " +
            "LOWER(z.prezime) LIKE LOWER(CONCAT('%', :prezime, '%')) AND " +
            "LOWER(z.email) LIKE LOWER(CONCAT('%', :email, '%')) AND " +
            "LOWER(z.departman) LIKE LOWER(CONCAT('%', :departman, '%')) AND " +
            "LOWER(z.pozicija) LIKE LOWER(CONCAT('%', :pozicija, '%')) AND " +
            "z.deleted = false")
    Page<Zaposlen> searchEmployees(
            @Param("ime") String ime,
            @Param("prezime") String prezime,
            @Param("email") String email,
            @Param("departman") String departman,
            @Param("pozicija") String pozicija,
            Pageable pageable
    );

    /**
     * Pretrazuje zaposlene jednim tekstualnim upitom po svim relevantnim kolonama uz paginaciju.
     * Upit se poredi sa imenom, prezimenom, emailom, departmanom i pozicijom.
     *
     * @param query tekstualni upit za pretragu
     * @param pageable parametri paginacije i sortiranja
     * @return stranica zaposlenih koji odgovaraju upitu
     */
    @Query("SELECT z FROM Zaposlen z WHERE " +
            "z.deleted = false AND (" +
            "LOWER(z.ime) LIKE LOWER(CONCAT('%', :query, '%')) OR " +
            "LOWER(z.prezime) LIKE LOWER(CONCAT('%', :query, '%')) OR " +
            "LOWER(z.email) LIKE LOWER(CONCAT('%', :query, '%')) OR " +
            "LOWER(z.departman) LIKE LOWER(CONCAT('%', :query, '%')) OR " +
            "LOWER(z.pozicija) LIKE LOWER(CONCAT('%', :query, '%')))")
    Page<Zaposlen> globalSearchEmployees(@Param("query") String query, Pageable pageable);
}
