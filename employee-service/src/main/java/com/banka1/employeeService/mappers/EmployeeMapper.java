package com.banka1.employeeService.mappers;

import com.banka1.employeeService.domain.Zaposlen;
import com.banka1.employeeService.domain.enums.Permission;
import com.banka1.employeeService.domain.enums.Role;
import com.banka1.employeeService.domain.service.ZaposlenService;
import com.banka1.employeeService.dto.requests.EmployeeCreateRequestDto;
import com.banka1.employeeService.dto.requests.EmployeeEditRequestDto;
import com.banka1.employeeService.dto.requests.EmployeeUpdateRequestDto;
import com.banka1.employeeService.dto.responses.EmployeeResponseDto;
import com.banka1.employeeService.exception.BusinessException;
import com.banka1.employeeService.exception.ErrorCode;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.Set;

/**
 * Mapper koji konvertuje DTO objekte u JPA entitete i obrnuto za entitet {@link Zaposlen}.
 * Takodje delegira postavljanje permisija servisu {@link ZaposlenService}.
 */
@Component
@RequiredArgsConstructor
public class EmployeeMapper {

    /** Servis koji postavlja skup permisija zaposlenog na osnovu njegove uloge. */
    private final ZaposlenService zaposlenService;

    /**
     * Mapira DTO za kreiranje zaposlenog u entitet.
     * Nakon mapiranja, postavlja permisije u skladu sa dodeljenon ulogom.
     *
     * @param dto ulazni podaci za kreiranje
     * @return novi entitet zaposlenog sa popunjenim permisijama
     */
    public Zaposlen toEntity(EmployeeCreateRequestDto dto) {
        Zaposlen zaposlen = new Zaposlen();
        zaposlen.setIme(dto.getIme());
        zaposlen.setPrezime(dto.getPrezime());
        zaposlen.setDatumRodjenja(dto.getDatumRodjenja());
        zaposlen.setPol(dto.getPol());
        zaposlen.setEmail(dto.getEmail());
        zaposlen.setBrojTelefona(dto.getBrojTelefona());
        zaposlen.setAdresa(dto.getAdresa());
        zaposlen.setUsername(dto.getUsername());
        zaposlen.setPozicija(dto.getPozicija());
        zaposlen.setDepartman(dto.getDepartman());
        zaposlen.setRole(dto.getRole());
        // Celina 1: default je aktivan, admin moze eksplicitno traziti neaktivnog
        zaposlen.setAktivan(dto.getAktivan() == null || dto.getAktivan());
        zaposlenService.setovanjePermisija(zaposlen);
        return zaposlen;
    }

    /**
     * Mapira entitet zaposlenog u izlazni DTO za API odgovor.
     * Ne ukljucuje osetljive podatke poput lozinke ili tokena.
     *
     * @param zaposlen entitet zaposlenog
     * @return DTO za API odgovor
     */
    public EmployeeResponseDto toDto(Zaposlen zaposlen) {
        return new EmployeeResponseDto(
                zaposlen.getId(),
                zaposlen.getIme(),
                zaposlen.getPrezime(),
                zaposlen.getEmail(),
                zaposlen.getUsername(),
                zaposlen.getDatumRodjenja(),
                zaposlen.getPol(),
                zaposlen.getBrojTelefona(),
                zaposlen.getAdresa(),
                zaposlen.getPozicija(),
                zaposlen.getDepartman(),
                zaposlen.isAktivan(),
                zaposlen.getRole()
        );
    }

    /**
     * Azurira entitet zaposlenog administrativnim izmenama.
     * Proverava da administrator ne moze da dodeli ulogu jacu od sopstvene.
     *
     * @param zaposlen entitet koji se menja
     * @param dto DTO sa novim vrednostima
     * @param role uloga administratora koji vrsi izmenu
     * @throws BusinessException ako DTO zahteva ulogu jacu od uloge administratora
     */
    public void updateEntityFromDto(Zaposlen zaposlen, EmployeeUpdateRequestDto dto, Role role, Set<Permission> permissions) {
        if (dto.getIme() != null) zaposlen.setIme(dto.getIme());
        if (dto.getPrezime() != null) zaposlen.setPrezime(dto.getPrezime());
        if (dto.getBrojTelefona() != null) zaposlen.setBrojTelefona(dto.getBrojTelefona());
        if (dto.getAdresa() != null) zaposlen.setAdresa(dto.getAdresa());
        if (dto.getPozicija() != null) zaposlen.setPozicija(dto.getPozicija());
        if (dto.getDepartman() != null) zaposlen.setDepartman(dto.getDepartman());
        if (dto.getAktivan() != null) zaposlen.setAktivan(dto.getAktivan());
        if (dto.getRole() != null) {
            if (dto.getRole().getPower() > role.getPower())
                throw new BusinessException(ErrorCode.NOT_STRONG_ROLE, "Ne mozes da mu das jacu rolu od svoje");
            zaposlen.setRole(dto.getRole());
            zaposlenService.setovanjePermisija(zaposlen);
        }
        if(dto.getMargin()!=null)
        {
            if(dto.getMargin()) {
                if (permissions.contains(Permission.MARGIN_TRADE))
                    zaposlen.getPermissionSet().add(Permission.MARGIN_TRADE);
            }
            else
            {
                zaposlen.getPermissionSet().remove(Permission.MARGIN_TRADE);
            }
        }
    }

    /**
     * Azurira entitet zaposlenog podacima koje korisnik moze samostalno da menja.
     * Ne dozvoljava promenu uloge ni statusa aktivnosti.
     *
     * @param zaposlen entitet koji se menja
     * @param dto DTO sa novim vrednostima
     */
    public void updateEntityFromDto(Zaposlen zaposlen, EmployeeEditRequestDto dto) {
        if (dto.getIme() != null) zaposlen.setIme(dto.getIme());
        if (dto.getPrezime() != null) zaposlen.setPrezime(dto.getPrezime());
        if (dto.getBrojTelefona() != null) zaposlen.setBrojTelefona(dto.getBrojTelefona());
        if (dto.getAdresa() != null) zaposlen.setAdresa(dto.getAdresa());
        if (dto.getPozicija() != null) zaposlen.setPozicija(dto.getPozicija());
        if (dto.getDepartman() != null) zaposlen.setDepartman(dto.getDepartman());
    }
}
