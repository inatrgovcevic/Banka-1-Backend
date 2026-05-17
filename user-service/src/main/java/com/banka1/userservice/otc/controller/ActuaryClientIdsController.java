package com.banka1.userservice.otc.controller;

import com.banka1.clientService.repository.KlijentRepository;
import com.banka1.employeeService.domain.enums.Role;
import com.banka1.employeeService.repository.ZaposlenRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Interni endpoint koji vraca klijentske ID-eve svih aktuara (zaposleni sa role=AGENT).
 * Koristi ga trading-service za filtriranje OTC public stocks po supervisor view-u.
 *
 * Mapira employee emails na client IDs jer su actuary i klijent ista osoba
 * ali sa razlicitim ID-evima u razlicitim tabelama.
 */
@Slf4j
@RestController
@RequestMapping("/internal/otc")
@RequiredArgsConstructor
public class ActuaryClientIdsController {

    private final ZaposlenRepository zaposlenRepository;
    private final KlijentRepository klijentRepository;

    @GetMapping("/actuary-client-ids")
    public ResponseEntity<List<Long>> actuaryClientIds() {
        Set<String> actuaryEmails = zaposlenRepository.findByRole(Role.AGENT).stream()
                .map(z -> z.getEmail().toLowerCase())
                .collect(Collectors.toSet());

        List<Long> clientIds = actuaryEmails.stream()
                .flatMap(email -> klijentRepository.findByEmail(email).stream())
                .map(k -> k.getId())
                .collect(Collectors.toList());

        log.debug("actuaryClientIds: found {} AGENT employees → {} matching clients",
                actuaryEmails.size(), clientIds.size());
        return ResponseEntity.ok(clientIds);
    }
}