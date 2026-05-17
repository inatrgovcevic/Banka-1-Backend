package com.banka1.account_service.controller;

import com.banka1.account_service.dto.request.*;
import com.banka1.account_service.dto.response.*;
import com.banka1.account_service.service.ClientService;
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

@RestController
@RequestMapping("/accounts/client")
@AllArgsConstructor
@PreAuthorize("hasAnyRole('CLIENT_BASIC', 'AGENT')")
/**
 * REST kontroler za operacije klijenata u Banka1 sistemu.
 * <p>
 * Omogucava klijentima (CLIENT_BASIC ili AGENT role) da upravljaju
 * svojim racunima, kartama, limitima i informatama.
 * <p>
 * Svi endpointi zahtevaju JWT Bearer token sa CLIENT_BASIC ili AGENT ulogom.
 * Operacije su ogranicene na vlasnikov racune.
 */
public class ClientController {
    private ClientService clientService;

//    @Operation(summary = "Create a new payment")
//    @ApiResponses({
//        @ApiResponse(responseCode = "400", description = "Invalid request body",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
//        @ApiResponse(responseCode = "401", description = "Unauthorized",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
//        @ApiResponse(responseCode = "403", description = "Forbidden",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
//    })
//    @PostMapping("/payments")
//    public ResponseEntity<String> newPayment(@AuthenticationPrincipal Jwt jwt,@RequestBody @Valid NewPaymentDto newPaymentDto) {
//        return new ResponseEntity<>(clientService.newPayment(jwt,newPaymentDto), HttpStatus.OK);
//    }
//
//    @Operation(summary = "Approve a transaction")
//    @ApiResponses({
//        @ApiResponse(responseCode = "400", description = "Invalid request body",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
//        @ApiResponse(responseCode = "401", description = "Unauthorized",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
//        @ApiResponse(responseCode = "403", description = "Forbidden",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
//        @ApiResponse(responseCode = "404", description = "Transaction not found",
//            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
//    })
//    @PostMapping("/transactions/{id}/approve")
//    public ResponseEntity<String> approveTransaction(@AuthenticationPrincipal Jwt jwt,@PathVariable Long id,@RequestBody @Valid ApproveDto approveDto) {
//        return new ResponseEntity<>(clientService.approveTransaction(jwt,id,approveDto), HttpStatus.OK);
//    }

    /**
     * Preuzima sve racune vlasnika sa mogucnoscu paginacije.
     * <p>
     * Vlasnik moze videti samo svoje aktivne racune.
     *
     * @param jwt JWT token klijenta
     * @param page broj stranice (podrazumevana: 0)
     * @param size velicina stranice, max 100 (podrazumevana: 10)
     * @return {@link Page} sa {@link AccountResponseDto} racunima
     */
    @Operation(summary = "Get my accounts")
    @ApiResponses({
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @GetMapping("/accounts")
    public ResponseEntity<Page<AccountResponseDto>> findMyAccounts(@AuthenticationPrincipal Jwt jwt,
                                                                   @RequestParam(defaultValue = "0") @Min(value = 0) int page,
                                                                   @RequestParam(defaultValue = "10") @Min(value = 1) @Max(value = 100) int size) {
        return new ResponseEntity<>(clientService.findMyAccounts(jwt,page,size), HttpStatus.OK);
    }

    /**
     * Azurira naziv racuna.
     * <p>
     * Vlasnik moze promeniti naziv svojeg racuna, pod uslovom da
     * ne postoji drugi racun sa istim nazivom koji mu pripada.
     *
     * @param jwt JWT token klijenta
     * @param accountNumber broj racuna
     * @param editAccountNameDto novi naziv racuna
     * @return poruka o uspehu
     */
    @Operation(summary = "Edit account name")
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
    @PutMapping("/api/accounts/{accountNumber}/name")
    public ResponseEntity<String> editAccountName(@AuthenticationPrincipal Jwt jwt,@PathVariable String accountNumber,@RequestBody @Valid EditAccountNameDto editAccountNameDto)
    {
        return new ResponseEntity<>(clientService.editAccountName(jwt,accountNumber,editAccountNameDto), HttpStatus.OK);
    }

    /**
     * Azurira naziv racuna preko ID-a.
     * <p>
     * Ekvivalentna operacija kao {@link #editAccountName(Jwt, String, EditAccountNameDto)},
     * ali koristi ID racuna umesto broja racuna.
     *
     * @param jwt JWT token klijenta
     * @param id ID racuna
     * @param editAccountNameDto novi naziv racuna
     * @return poruka o uspehu
     */
    @PatchMapping("/accounts/{id}/name")
    public ResponseEntity<String> editAccountNameId(@AuthenticationPrincipal Jwt jwt,@PathVariable Long id,@RequestBody @Valid EditAccountNameDto editAccountNameDto)
    {
        return new ResponseEntity<>(clientService.editAccountName(jwt,id,editAccountNameDto), HttpStatus.OK);
    }

    /**
     * Azurira dnevni i mesecni limit trosenja na racunu.
     * <p>
     * Azuriranje limite zahteva verifikaciju preko mobilne aplikacije.
     * Dnevni limit mora biti manji ili jednak od mesecnog.
     *
     * @param jwt JWT token klijenta
     * @param id ID racuna
     * @param editAccountLimitDto novi limitni sa session ID-om verifikacije
     * @return poruka o uspehu
     */
    @Operation(summary = "Edit account transaction limit")
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
    @PatchMapping("/accounts/{id}/limits")
    public ResponseEntity<String> editAccountLimitId(@AuthenticationPrincipal Jwt jwt,@PathVariable Long id,@RequestBody @Valid EditAccountLimitDto editAccountLimitDto)
    {
        return new ResponseEntity<>(clientService.editAccountLimit(jwt,id,editAccountLimitDto), HttpStatus.OK);
    }

    /**
     * Azurira dnevni i mesecni limit trosenja na racunu preko broja racuna.
     *
     * @param jwt JWT token klijenta
     * @param accountNumber broj racuna
     * @param editAccountLimitDto novi limitni sa session ID-om verifikacije
     * @return poruka o uspehu
     */
    @PutMapping("/api/accounts/{accountNumber}/limits")
    public ResponseEntity<String> editAccountLimit(@AuthenticationPrincipal Jwt jwt,@PathVariable String accountNumber,@RequestBody @Valid EditAccountLimitDto editAccountLimitDto)
    {
        return new ResponseEntity<>(clientService.editAccountLimit(jwt,accountNumber,editAccountLimitDto), HttpStatus.OK);
    }

    /**
     * Preuzima detaljne informacije o racunu preko ID-a.
     * <p>
     * Vlasnik moze videti samo svoje racune.
     *
     * @param jwt JWT token klijenta
     * @param id ID racuna
     * @return {@link AccountDetailsResponseDto} sa svim detaljima racuna
     */
    @Operation(summary = "Get account details")
    @ApiResponses({
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "404", description = "Account not found",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @GetMapping("/accounts/{id}")
    public ResponseEntity<AccountDetailsResponseDto> getDetailsId (@AuthenticationPrincipal Jwt jwt,@PathVariable Long id)
    {
        return new ResponseEntity<>(clientService.getDetails(jwt,id), HttpStatus.OK);
    }

    /**
     * Preuzima detaljne informacije o racunu preko broja racuna.
     *
     * @param jwt JWT token klijenta
     * @param accountNumber broj racuna
     * @return {@link AccountDetailsResponseDto} sa svim detaljima racuna
     */
    @PreAuthorize("hasAnyRole('CLIENT_BASIC', 'AGENT', 'SERVICE')")
    @GetMapping("/api/accounts/{accountNumber}")
    public ResponseEntity<AccountDetailsResponseDto> getDetails (@AuthenticationPrincipal Jwt jwt,@PathVariable String accountNumber)
    {
        return new ResponseEntity<>(clientService.getDetails(jwt,accountNumber), HttpStatus.OK);
    }

    /**
     * Preuzima sve kartice vezane za racun vlasnika.
     *
     * @param jwt JWT token klijenta
     * @param id ID racuna
     * @param page broj stranice (podrazumevana: 0)
     * @param size velicina stranice, max 100 (podrazumevana: 10)
     * @return {@link Page} sa {@link CardResponseDto} karticama
     */
    @Operation(summary = "Get account cards")
    @ApiResponses({
        @ApiResponse(responseCode = "401", description = "Unauthorized",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "403", description = "Forbidden",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class))),
        @ApiResponse(responseCode = "404", description = "Account not found",
            content = @Content(schema = @Schema(implementation = ErrorResponseDto.class)))
    })
    @GetMapping("/accounts/{id}/cards")
    public ResponseEntity<Page<CardResponseDto>> findAllCards(@AuthenticationPrincipal Jwt jwt,
                                                              @PathVariable Long id,
                                                              @RequestParam(defaultValue = "0") @Min(value = 0) int page,
                                                              @RequestParam(defaultValue = "10") @Min(value = 1) @Max(value = 100) int size) {
        return new ResponseEntity<>(clientService.findAllCards(jwt,id,page,size), HttpStatus.OK);
    }
}
