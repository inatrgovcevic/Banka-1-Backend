package com.banka1.banking_service.account_service.service;

import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.dto.request.CheckingDto;
import com.banka1.banking_service.account_service.dto.request.FxDto;
import com.banka1.banking_service.account_service.dto.request.UpdateCardDto;
import com.banka1.banking_service.account_service.dto.request.UpdateCompanyDto;
import com.banka1.banking_service.account_service.dto.response.AccountDetailsResponseDto;
import com.banka1.banking_service.account_service.dto.response.AccountSearchResponseDto;
import com.banka1.banking_service.account_service.dto.response.CardResponseDto;
import com.banka1.banking_service.account_service.dto.response.CompanyResponseDto;
import org.springframework.data.domain.Page;
import org.springframework.security.oauth2.jwt.Jwt;

import java.util.List;

/**
 * Servis koji implementira poslovnu logiku dostupnu zaposlenima banke.
 * <p>
 * Omogucava:
 * <ul>
 *   <li>Kreiranje tekucih i deviznih racuna za klijente</li>
 *   <li>Pretragu racuna sa filtiranjem i paginacijom</li>
 *   <li>Upravljanje statusom racuna i limitima</li>
 *   <li>Pristup internim bankovnim racunima</li>
 *   <li>Upravljanje podatcima o firmama</li>
 * </ul>
 * <p>
 * Obicno je dostupan korisnicima sa BASIC ili SERVICE ulogom.
 */
public interface EmployeeService {

    /**
     * Kreira novi devizni (FX) racun sa zadatom valurom.
     *
     * @param jwt JWT token zaposlenog
     * @param fxDto podaci za kreiranje FX racuna
     * @return {@link AccountDetailsResponseDto} sa detaljima novog racuna
     */
    AccountDetailsResponseDto createFxAccount(Jwt jwt, FxDto fxDto);

    /**
     * Kreira novi tekuci (checking) racun denominovan u RSD.
     *
     * @param jwt JWT token zaposlenog
     * @param checkingDto podaci za kreiranje tekuceg racuna
     * @return {@link AccountDetailsResponseDto} sa detaljima novog racuna
     */
    AccountDetailsResponseDto createCheckingAccount(Jwt jwt, CheckingDto checkingDto);

    /**
     * Pretrazuje sve racune sa mogucnoscu filtriranja po imenu, prezimenu i broju racuna.
     *
     * @param jwt JWT token zaposlenog
     * @param imeVlasnikaRacuna (opciono) ime vlasnika za filtriranje
     * @param prezimeVlasnikaRacuna (opciono) prezime vlasnika za filtriranje
     * @param accountNumber (opciono) broj racuna za filtriranje
     * @param page broj stranice
     * @param size velicina stranice
     * @return {@link Page} sa {@link AccountSearchResponseDto}
     */
    Page<AccountSearchResponseDto> searchAllAccounts(Jwt jwt, String imeVlasnikaRacuna, String prezimeVlasnikaRacuna, String accountNumber, int page, int size);

    /**
     * Azurira status kartice na racunu (zastarelo - handleovati kroz Card servis).
     *
     * @param jwt JWT token zaposlenog
     * @param id ID kartice
     * @param updateCardDto novi status kartice
     * @return poruka o uspehu
     */
    String updateCard(Jwt jwt, Long id, UpdateCardDto updateCardDto);

    /**
     * Preuzima sve interne bankovske racune (sa vlasnikID = -1).
     *
     * @return lista svih internih bankovskih racuna
     */
    List<AccountDetailsResponseDto> getBankAccounts();

    /**
     * Preuzima interni bankovski racun za specificnu valutu.
     *
     * @param currencyCode kod valute
     * @return {@link AccountDetailsResponseDto} za tu valutu
     */
    AccountDetailsResponseDto getBankAccountByCurrency(CurrencyCode currencyCode);

    /**
     * Preuzima detaljne informacije o racunu preko broja racuna.
     *
     * @param accountNumber 18-cifreni broj racuna
     * @return {@link AccountDetailsResponseDto} sa svim detaljima
     */
    AccountDetailsResponseDto getAccountDetails(String accountNumber);

    /**
     * Preuzima sve racune specifican klijenta sa paginacijom.
     *
     * @param clientId ID klijenta
     * @param page broj stranice
     * @param size velicina stranice
     * @return {@link Page} sa {@link AccountDetailsResponseDto}
     */
    Page<AccountDetailsResponseDto> getClientAccounts(Long clientId, int page, int size);

    /**
     * Preuzima kartice za specifican racun.
     * <p>
     * Napomena: Kartice se obicno upravljaju kroz Card servis.
     *
     * @param accountNumber broj racuna
     * @param page broj stranice
     * @param size velicina stranice
     * @return {@link Page} sa {@link CardResponseDto}
     */
    Page<CardResponseDto> getAccountCards(String accountNumber, int page, int size);

    /**
     * Preuzima detaljne informacije o firmi.
     *
     * @param id ID firme
     * @return {@link CompanyResponseDto} sa detaljima firme
     */
    CompanyResponseDto getCompany(Long id);

    /**
     * Azurira detaljne informacije o firmi.
     *
     * @param id ID firme koja se azurira
     * @param dto novi podaci za firmu
     * @return {@link CompanyResponseDto} sa azuriranim detaljima
     */
    CompanyResponseDto updateCompany(Long id, UpdateCompanyDto dto);
}
