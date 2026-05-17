package com.banka1.account_service.controller;

import com.banka1.account_service.domain.enums.CurrencyCode;
import com.banka1.account_service.dto.request.*;
import com.banka1.account_service.dto.response.*;
import com.banka1.account_service.service.ClientService;
import com.banka1.account_service.service.EmployeeService;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.media.Content;
import io.swagger.v3.oas.annotations.media.Schema;
import io.swagger.v3.oas.annotations.responses.ApiResponse;
import io.swagger.v3.oas.annotations.responses.ApiResponses;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import lombok.AllArgsConstructor;
import org.springframework.data.domain.Page;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/**
 * REST kontroler za operacije zaposlenih u Banka1 sistemu.
 * <p>
 * Omogucava zaposlenim (BASIC role) da kreiraju i upravljaju
 * racunima, firmama, i kartama klijentima.
 * <p>
 * Svi endpointi zahtevaju JWT Bearer token sa BASIC ili SERVICE ulogom.
 */
@RestController
@AllArgsConstructor
@RequestMapping("/accounts/employee")
//todo autorizacija

public class EmployeeController {

    private EmployeeService employeeService;
    private ClientService clientService;

    /**
     * Kreira novi tekuci (checking) racun sa zadatim parametrima.
     * <p>
     * Racun je denominiran u RSD valuti i moze biti licni (PERSONAL)
     * ili poslovni (BUSINESS).
     *
     * @param jwt JWT token zaposlenog
     * @param checkingDto podaci za kreiranje tekuceg racuna
     * @return {@link AccountDetailsResponseDto} sa detaljima novog racuna
     */
    @Operation(summary = "Create checking account")
    @ApiResponses({
        @ApiResponse(responseCode = "400", description = "Invalid request body",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @PreAuthorize("hasRole('BASIC')")
    @PostMapping("/accounts/checking")
    public ResponseEntity<AccountDetailsResponseDto> createCheckingAccount(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid CheckingDto checkingDto) {
        return new ResponseEntity<>(employeeService.createCheckingAccount(jwt, checkingDto), HttpStatus.OK);
    }

    /**
     * Kreira novi devizni (FX) racun sa zadatom valurom.
     * <p>
     * Racun se ne moze kreirati u RSD valuti (jer se koristi tekuci racun).
     * Moze biti licni (PERSONAL) ili poslovni (BUSINESS).
     *
     * @param jwt JWT token zaposlenog
     * @param fxDto podaci za kreiranje deviznog racuna
     * @return {@link AccountDetailsResponseDto} sa detaljima novog racuna
     */
    @Operation(summary = "Create FX account")
    @ApiResponses({
        @ApiResponse(responseCode = "400", description = "Invalid request body",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @PreAuthorize("hasRole('BASIC')")
    @PostMapping("/accounts/fx")
    public ResponseEntity<AccountDetailsResponseDto> createFxAccount(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid FxDto fxDto) {
        return new ResponseEntity<>(employeeService.createFxAccount(jwt, fxDto), HttpStatus.OK);
    }

    /**
     * Pretrazuje sve racune sa mogucnoscu filtriranja po imenu, prezimenu,
     * ili broju racuna vlasnika.
     * <p>
     * Rezultati su pagirani i sortirani po prezimenu i imenu vlasnika.
     *
     * @param jwt JWT token zaposlenog
     * @param imeVlasnikaRacuna (opciono) ime vlasnika za pretragu
     * @param prezimeVlasnikaRacuna (opciono) prezime vlasnika za pretragu
     * @param accountNumber (opciono) broj racuna za pretragu
     * @param page broj stranice (podrazumevana: 0)
     * @param size velicina stranice, max 100 (podrazumevana: 10)
     * @return {@link Page} sa {@link AccountSearchResponseDto} rezultatima
     */
    @Operation(summary = "Search all accounts")
    @ApiResponses({
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @PreAuthorize("hasAnyRole('BASIC','SERVICE')")
    @GetMapping("/accounts")
    public ResponseEntity<Page<AccountSearchResponseDto>> searchAllAccounts(@AuthenticationPrincipal Jwt jwt,
                                                                            @RequestParam(required = false) String imeVlasnikaRacuna,
                                                                            @RequestParam(required = false) String prezimeVlasnikaRacuna,
                                                                            @RequestParam(required = false) String accountNumber,
                                                                            @RequestParam(defaultValue = "0") @Min(value = 0) int page,
                                                                            @RequestParam(defaultValue = "10") @Min(value = 1) @Max(value = 100) int size
    ) {
        return new ResponseEntity<>(employeeService.searchAllAccounts(jwt,imeVlasnikaRacuna,prezimeVlasnikaRacuna,accountNumber,page,size), HttpStatus.OK);
    }

    /**
     * Menja status racuna (ACTIVE -> INACTIVE ili obrnuto).
     * <p>
     * Kada se racun deaktiviruje, sve kartice na njemu se deaktiviruju
     * i vlasnik dobija notifikaciju putem email-a.
     *
     * @param jwt JWT token zaposlenog
     * @param accountNumber broj racuna koji se menja
     * @param editStatus novi status racuna
     * @return poruka o uspehu
     */
    @Operation(summary = "Edit account status")
    @ApiResponses({
        @ApiResponse(responseCode = "400", description = "Invalid request body",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "404", description = "Account not found",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @PreAuthorize("hasRole('BASIC')")
    @PutMapping("/accounts/{accountNumber}/status")
    public ResponseEntity<String> editStatus(@AuthenticationPrincipal Jwt jwt, @PathVariable String accountNumber, @RequestBody @Valid EditStatus editStatus) {
        return new ResponseEntity<>(clientService.editStatus(jwt, accountNumber, editStatus), HttpStatus.OK);
    }

    /**
     * Preuzima detaljne informacije o racunu preko broja racuna.
     *
     * @param jwt JWT token zaposlenog
     * @param accountNumber broj racuna
     * @return {@link AccountDetailsResponseDto} sa svim detaljima racuna
     */
    @Operation(summary = "Get account details by account number (employee access)")
    @PreAuthorize("hasRole('BASIC')")
    @GetMapping("/accounts/{accountNumber}")
    public ResponseEntity<AccountDetailsResponseDto> getAccountDetails(@AuthenticationPrincipal Jwt jwt,
                                                                       @PathVariable String accountNumber) {
        return new ResponseEntity<>(employeeService.getAccountDetails(accountNumber), HttpStatus.OK);
    }

    /**
     * Preuzima sve racune specifican klijenta sa mogucnoscu paginacije.
     *
     * @param jwt JWT token zaposlenog
     * @param clientId ID klijenta
     * @param page broj stranice (podrazumevana: 0)
     * @param size velicina stranice, max 100 (podrazumevana: 10)
     * @return {@link Page} sa {@link AccountDetailsResponseDto} racunima
     */
    @Operation(summary = "Get all accounts for a specific client")
    @PreAuthorize("hasRole('BASIC')")
    @GetMapping("/accounts/client/{clientId}")
    public ResponseEntity<Page<AccountDetailsResponseDto>> getClientAccounts(@AuthenticationPrincipal Jwt jwt,
                                                                             @PathVariable Long clientId,
                                                                             @RequestParam(defaultValue = "0") @Min(0) int page,
                                                                             @RequestParam(defaultValue = "10") @Min(1) @Max(100) int size) {
        return new ResponseEntity<>(employeeService.getClientAccounts(clientId, page, size), HttpStatus.OK);
    }

    /**
     * Preuzima sve kartice vezane za specifican racun.
     *
     * @param jwt JWT token zaposlenog
     * @param accountNumber broj racuna
     * @param page broj stranice (podrazumevana: 0)
     * @param size velicina stranice, max 100 (podrazumevana: 10)
     * @return {@link Page} sa {@link CardResponseDto} karticama
     */
    // Cards are managed by the Card Service — disabled here
//    @Operation(summary = "Get cards for a specific account (employee access)")
//    @PreAuthorize("hasRole('BASIC')")
//    @GetMapping("/accounts/{accountNumber}/cards")
//    public ResponseEntity<Page<CardResponseDto>> getAccountCards(@AuthenticationPrincipal Jwt jwt,
//                                                                 @PathVariable String accountNumber,
//                                                                 @RequestParam(defaultValue = "0") @Min(0) int page,
//                                                                 @RequestParam(defaultValue = "10") @Min(1) @Max(100) int size) {
//        return new ResponseEntity<>(employeeService.getAccountCards(accountNumber, page, size), HttpStatus.OK);
//    }

    /**
     * Preuzima sve interne bankovne racune (sa vlasnikID = -1).
     *
     * @param jwt JWT token zaposlenog
     * @return lista svih internih bankovnih racuna
     */
    @Operation(summary = "Get all bank internal accounts")
    @PreAuthorize("hasRole('BASIC')")
    @GetMapping("/accounts/bank")
    public ResponseEntity<List<AccountDetailsResponseDto>> getBankAccounts(@AuthenticationPrincipal Jwt jwt) {
        return new ResponseEntity<>(employeeService.getBankAccounts(), HttpStatus.OK);
    }

    /**
     * Preuzima interni bankovski racun za specificnu valutu.
     *
     * @param jwt JWT token zaposlenog
     * @param currency kod valute (npr. USD, EUR)
     * @return {@link AccountDetailsResponseDto} bankovskog racuna
     */
    @Operation(summary = "Get bank internal account for a specific currency")
    @PreAuthorize("hasRole('BASIC')")
    @GetMapping("/accounts/bank/{currency}")
    public ResponseEntity<AccountDetailsResponseDto> getBankAccountByCurrency(@AuthenticationPrincipal Jwt jwt,
                                                                               @PathVariable CurrencyCode currency) {
        return new ResponseEntity<>(employeeService.getBankAccountByCurrency(currency), HttpStatus.OK);
    }

    /**
     * Preuzima detaljne informacije o firmi.
     *
     * @param jwt JWT token zaposlenog
     * @param id ID firme
     * @return {@link CompanyResponseDto} sa detaljima firme
     */
    @Operation(summary = "Get company details")
    @PreAuthorize("hasRole('BASIC')")
    @GetMapping("/companies/{id}")
    public ResponseEntity<CompanyResponseDto> getCompany(@AuthenticationPrincipal Jwt jwt, @PathVariable Long id) {
        return new ResponseEntity<>(employeeService.getCompany(id), HttpStatus.OK);
    }

    /**
     * Azurira detaljne informacije o firmi.
     *
     * @param jwt JWT token zaposlenog
     * @param id ID firme koja se azurira
     * @param dto novi podaci za firmu
     * @return {@link CompanyResponseDto} sa azuriranim detaljima
     */
    @Operation(summary = "Update company details")
    @PreAuthorize("hasRole('BASIC')")
    @PutMapping("/companies/{id}")
    public ResponseEntity<CompanyResponseDto> updateCompany(@AuthenticationPrincipal Jwt jwt,
                                                            @PathVariable Long id,
                                                            @RequestBody @Valid UpdateCompanyDto dto) {
        return new ResponseEntity<>(employeeService.updateCompany(id, dto), HttpStatus.OK);
    }
}
