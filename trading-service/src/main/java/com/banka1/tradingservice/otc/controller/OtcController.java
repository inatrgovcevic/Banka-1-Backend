package com.banka1.tradingservice.otc.controller;

import com.banka1.tradingservice.otc.dto.CounterOfferRequest;
import com.banka1.tradingservice.otc.dto.CreateOtcOfferRequest;
import com.banka1.tradingservice.otc.dto.CreateOtcPositionRequest;
import com.banka1.tradingservice.otc.dto.OtcOfferDto;
import com.banka1.tradingservice.otc.dto.OtcPositionDto;
import com.banka1.tradingservice.otc.dto.PublicStockDto;
import com.banka1.tradingservice.otc.dto.UpdateOtcPositionRequest;
import com.banka1.tradingservice.otc.service.OtcService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/**
 * REST kontroler za OTC pregovore i ugovore (PR_04).
 * Spec: Celina 4.txt — Portal: OTC Ponude i Ugovori.
 */
@RestController
@RequestMapping("/otc")
@RequiredArgsConstructor
public class OtcController {

    private final OtcService otcService;

    /**
     * Inicijalna ponuda kupca prodavcu (status PENDING_SELLER).
     */
    @PostMapping("/offers")
    public ResponseEntity<OtcOfferDto> createOffer(
            @AuthenticationPrincipal Jwt jwt,
            @RequestBody @Valid CreateOtcOfferRequest req) {
        Long buyerId = jwt.getClaim("id");
        String name = jwt.getClaim("name");
        return new ResponseEntity<>(otcService.createOffer(buyerId, req, name), HttpStatus.CREATED);
    }

    /** Kontraponuda — moze posaljiti kupac ili prodavac. */
    @PostMapping("/offers/{offerId}/counter")
    public ResponseEntity<OtcOfferDto> counterOffer(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long offerId,
            @RequestBody @Valid CounterOfferRequest req) {
        Long actorId = jwt.getClaim("id");
        String name = jwt.getClaim("name");
        return ResponseEntity.ok(otcService.counterOffer(offerId, actorId, req, name));
    }

    /** Prihvatanje ponude — dozvoljeno strani koja trenutno ceka odgovor. */
    @PostMapping("/offers/{offerId}/accept")
    public ResponseEntity<OtcOfferDto> accept(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long offerId) {
        Long actorId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.accept(offerId, actorId));
    }

    /** Bilo kupac, bilo prodavac moze odustati. */
    @PostMapping("/offers/{offerId}/reject")
    public ResponseEntity<OtcOfferDto> reject(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long offerId) {
        Long actorId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.reject(offerId, actorId));
    }

    /** Povlacenje sopstvene ponude pre nego sto je druga strana odgovorila. */
    @PostMapping("/offers/{offerId}/withdraw")
    public ResponseEntity<OtcOfferDto> withdraw(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long offerId) {
        Long actorId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.withdraw(offerId, actorId));
    }

    /** Javno dostupne akcije za OTC kupovinu — vidljivo svim ulogovanim korisnicima osim sopstvenih.
     *  Supervisor vidi samo akcije koje su izlozili aktuari. */
    @GetMapping("/public-stocks")
    public ResponseEntity<List<PublicStockDto>> publicStocks(@AuthenticationPrincipal Jwt jwt) {
        Long currentUserId = jwt.getClaim("id");
        String role = jwt.getClaim("roles");
        boolean supervisorView = "SUPERVISOR".equalsIgnoreCase(role);
        return ResponseEntity.ok(otcService.getPublicStocks(currentUserId, supervisorView));
    }

    /** Stranica: Aktivne ponude. */
    @GetMapping("/offers/active")
    public ResponseEntity<List<OtcOfferDto>> activeForCurrentUser(@AuthenticationPrincipal Jwt jwt) {
        Long userId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.activeForUser(userId));
    }

    /** Iskoristi opciju (SAGA OTC_EXERCISE). */
    @PostMapping("/contracts/{contractId}/exercise")
    public ResponseEntity<Void> exercise(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long contractId) {
        Long buyerId = jwt.getClaim("id");
        otcService.exerciseContract(contractId, buyerId);
        return ResponseEntity.accepted().build();
    }

    // ---- My OTC Positions (Tab 2) ----

    /** Sve STOCK pozicije koje je korisnik izlozio za OTC trading. */
    @GetMapping("/my-positions")
    public ResponseEntity<List<OtcPositionDto>> myPositions(@AuthenticationPrincipal Jwt jwt) {
        Long userId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.getMyPositions(userId));
    }

    /** Izlozavanje STOCK pozicije na OTC trziste. */
    @PostMapping("/positions")
    public ResponseEntity<OtcPositionDto> addPosition(
            @AuthenticationPrincipal Jwt jwt,
            @RequestBody @Valid CreateOtcPositionRequest req) {
        Long userId = jwt.getClaim("id");
        return new ResponseEntity<>(otcService.addPosition(userId, req), HttpStatus.CREATED);
    }

    /** Izmena izlozene kolicine. */
    @PutMapping("/positions/{positionId}")
    public ResponseEntity<OtcPositionDto> updatePosition(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long positionId,
            @RequestBody @Valid UpdateOtcPositionRequest req) {
        Long userId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.updatePosition(userId, positionId, req));
    }

    /** Uklanjanje pozicije sa OTC trzista. */
    @DeleteMapping("/positions/{positionId}")
    public ResponseEntity<Void> removePosition(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long positionId) {
        Long userId = jwt.getClaim("id");
        otcService.removePosition(userId, positionId);
        return ResponseEntity.noContent().build();
    }

    /**
     * PR_13 C13.3: GET /otc/contracts/my — sklopljeni ugovori za current user-a.
     * Optional ?status filter (ACTIVE | EXERCISED | EXPIRED).
     */
    @GetMapping("/contracts/my")
    public ResponseEntity<java.util.List<com.banka1.tradingservice.otc.dto.OptionContractDto>> myContracts(
            @AuthenticationPrincipal Jwt jwt,
            @RequestParam(required = false) com.banka1.tradingservice.otc.domain.OptionContractStatus status) {
        Long userId = jwt.getClaim("id");
        return ResponseEntity.ok(otcService.myContracts(userId, status));
    }
}
