package com.banka1.banking_service.account_service.service;


import com.banka1.banking_service.account_service.dto.request.EditAccountLimitDto;
import com.banka1.banking_service.account_service.dto.request.EditAccountNameDto;
import com.banka1.banking_service.account_service.dto.request.EditStatus;
import com.banka1.banking_service.account_service.dto.response.AccountDetailsResponseDto;
import com.banka1.banking_service.account_service.dto.response.AccountResponseDto;
import com.banka1.banking_service.account_service.dto.response.CardResponseDto;
import org.springframework.data.domain.Page;
import org.springframework.security.oauth2.jwt.Jwt;

/**
 * Servis koji implementira poslovnu logiku dostupnu klijentima banke.
 * Pokriva pregled racuna, upravljanje imenima i limitima, uvid u kartice i promenu statusa racuna.
 */
public interface ClientService {

    /**
     * Vraca paginiranu listu aktivnih racuna prijavljenog klijenta.
     *
     * @param jwt  JWT token klijenta (koristiti za ekstrakciju ID-a vlasnika)
     * @param page broj stranice (0-indeksiran)
     * @param size velicina stranice
     * @return stranica {@link AccountResponseDto} koja pripada klijentu
     */
    Page<AccountResponseDto> findMyAccounts(Jwt jwt, int page, int size);

    /**
     * Menja naziv racuna identifikovanog internim ID-em.
     *
     * @param jwt                 JWT token vlasnika
     * @param id                  interni ID racuna
     * @param editAccountNameDto  novi naziv racuna
     * @return poruka o uspesnosti promene
     */
    String editAccountName(Jwt jwt, Long id, EditAccountNameDto editAccountNameDto);

    /**
     * Menja naziv racuna identifikovanog brojem racuna.
     *
     * @param jwt                 JWT token vlasnika
     * @param accountNumber       18-cifreni broj racuna
     * @param editAccountNameDto  novi naziv racuna
     * @return poruka o uspesnosti promene
     */
    String editAccountName(Jwt jwt, String accountNumber, EditAccountNameDto editAccountNameDto);

    /**
     * Menja dnevni i mesecni limit racuna identifikovanog internim ID-em.
     * Zahteva verifikaciju putem koda pre primene promene.
     *
     * @param jwt                  JWT token vlasnika
     * @param id                   interni ID racuna
     * @param editAccountLimitDto  novi dnevni i mesecni limit sa verifikacionim kodom
     * @return poruka o uspesnosti promene
     */
    String editAccountLimit(Jwt jwt, Long id, EditAccountLimitDto editAccountLimitDto);

    /**
     * Menja dnevni i mesecni limit racuna identifikovanog brojem racuna.
     * Zahteva verifikaciju putem koda pre primene promene.
     *
     * @param jwt                  JWT token vlasnika
     * @param accountNumber        18-cifreni broj racuna
     * @param editAccountLimitDto  novi dnevni i mesecni limit sa verifikacionim kodom
     * @return poruka o uspesnosti promene
     */
    String editAccountLimit(Jwt jwt, String accountNumber, EditAccountLimitDto editAccountLimitDto);

    /**
     * Vraca detalje racuna identifikovanog internim ID-em.
     * Dostupno samo vlasniku racuna.
     *
     * @param jwt JWT token vlasnika
     * @param id  interni ID racuna
     * @return detaljan prikaz racuna
     */
    AccountDetailsResponseDto getDetails(Jwt jwt, Long id);

    /**
     * Vraca detalje racuna identifikovanog brojem racuna.
     * Dostupno samo vlasniku racuna.
     *
     * @param jwt           JWT token vlasnika
     * @param accountNumber 19-cifreni broj racuna
     * @return detaljan prikaz racuna
     */
    AccountDetailsResponseDto getDetails(Jwt jwt, String accountNumber);

    /**
     * Vraca paginiranu listu kartica vezanih za racun.
     * Poziva Card Service putem REST poziva da pribavi kartice.
     *
     * @param jwt  JWT token vlasnika
     * @param id   interni ID racuna
     * @param page broj stranice (0-indeksiran)
     * @param size velicina stranice
     * @return stranica {@link CardResponseDto} za dati racun
     */
    Page<CardResponseDto> findAllCards(Jwt jwt, Long id, int page, int size);

    /**
     * Menja status racuna (aktiviranje ili deaktiviranje).
     * Samo zaposleni mogu pozivati ovaj metod. Prilikom deaktivacije salje se
     * email notifikacija i event ka Card Service-u za deaktiviranje svih kartica.
     *
     * @param jwt           JWT token zaposlenog
     * @param accountNumber 18-cifreni broj racuna
     * @param editStatus    zeljeni status racuna
     * @return poruka o uspesnosti promene
     */
    String editStatus(Jwt jwt, String accountNumber, EditStatus editStatus);
}
