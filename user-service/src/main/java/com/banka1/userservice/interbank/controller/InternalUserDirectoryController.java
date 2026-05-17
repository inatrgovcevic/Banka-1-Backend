package com.banka1.userservice.interbank.controller;

import com.banka1.clientService.dto.responses.ClientInfoResponseDto;
import com.banka1.clientService.service.ClientService;
import com.banka1.employeeService.dto.responses.EmployeeResponseDto;
import com.banka1.employeeService.service.CrudService;
import com.banka1.userservice.interbank.dto.InterbankUserDisplayDto;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 13: interni endpoint koji resolve-uje klijenta ili zaposlenog
 * u friendly display DTO ({@link InterbankUserDisplayDto}) za interbank flow.
 *
 * <p>Konzumira ga {@code interbank-service.UserInternalClient} koji koristi
 * rezultat za rendering Trade ticket-a, OTC ugovora i SAGA event-ova.
 *
 * <p>Endpoint je restriktovan na SERVICE token (ServiceJwtAuthInterceptor),
 * isti pattern kao {@code /clients/jmbg/{jmbg}}.
 *
 * <p>Type parametar je case-insensitive ({@code CLIENT} ili {@code EMPLOYEE}).
 * Nepoznat type vraca 400; not-found u service-u vraca 404.
 */
@RestController
@RequestMapping("/internal/interbank")
@PreAuthorize("hasRole('SERVICE')")
@RequiredArgsConstructor
@Slf4j
public class InternalUserDirectoryController {

    private final ClientService clientService;
    private final CrudService employeeService;

    @GetMapping("/user/{type}/{id}")
    public ResponseEntity<InterbankUserDisplayDto> resolve(
            @PathVariable String type,
            @PathVariable Long id) {
        return switch (type.toUpperCase()) {
            case "CLIENT" -> resolveClient(id);
            case "EMPLOYEE" -> resolveEmployee(id);
            default -> {
                log.warn("Unknown interbank user type: {}", type);
                yield ResponseEntity.badRequest().build();
            }
        };
    }

    private ResponseEntity<InterbankUserDisplayDto> resolveClient(Long id) {
        try {
            ClientInfoResponseDto c = clientService.getInfoById(id);
            String first = c.getName() == null ? "" : c.getName();
            String last = c.getLastName() == null ? "" : c.getLastName();
            return ResponseEntity.ok(new InterbankUserDisplayDto(
                    first, last, (first + " " + last).trim()));
        } catch (Exception e) {
            log.warn("Client {} not found: {}", id, e.getMessage());
            return ResponseEntity.notFound().build();
        }
    }

    private ResponseEntity<InterbankUserDisplayDto> resolveEmployee(Long id) {
        try {
            EmployeeResponseDto e = employeeService.getEmployee(id);
            String first = e.getIme() == null ? "" : e.getIme();
            String last = e.getPrezime() == null ? "" : e.getPrezime();
            return ResponseEntity.ok(new InterbankUserDisplayDto(
                    first, last, (first + " " + last).trim()));
        } catch (Exception ex) {
            log.warn("Employee {} not found: {}", id, ex.getMessage());
            return ResponseEntity.notFound().build();
        }
    }
}
