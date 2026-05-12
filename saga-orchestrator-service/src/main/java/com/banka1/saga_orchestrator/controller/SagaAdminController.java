package com.banka1.saga_orchestrator.controller;

import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.tags.Tag;
import lombok.RequiredArgsConstructor;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.UUID;

/**
 * Admin/internal API za pregled SAGA instance-a. Po DoD #213 ovo je samo
 * read-only debug endpoint. JWT/role enforcement (admin) ide kroz security-lib
 * filter chain — biće dodat u sledećem PR-u kad #213 bude review-ovan.
 */
@RestController
@RequestMapping("/saga/instances")
@RequiredArgsConstructor
@Tag(name = "Saga", description = "Admin pregled SAGA instance-a")
public class SagaAdminController {

    private final SagaInstanceRepository repository;

    @Operation(summary = "Paged listing SAGA instance-a sa opcionim filterima")
    @GetMapping
    public ResponseEntity<Page<SagaInstance>> list(
            @RequestParam(required = false) SagaState state,
            @RequestParam(required = false) SagaType sagaType,
            @RequestParam(defaultValue = "0") int page,
            @RequestParam(defaultValue = "20") int size) {
        PageRequest pageable = PageRequest.of(page, size);
        if (state != null && sagaType != null) {
            return ResponseEntity.ok(repository.findByStateAndSagaType(state, sagaType, pageable));
        }
        if (state != null) {
            return ResponseEntity.ok(repository.findByState(state, pageable));
        }
        if (sagaType != null) {
            return ResponseEntity.ok(repository.findBySagaType(sagaType, pageable));
        }
        return ResponseEntity.ok(repository.findAll(pageable));
    }

    @Operation(summary = "Detalj jedne SAGA instance po UUID-u")
    @GetMapping("/{id}")
    public ResponseEntity<SagaInstance> getOne(@PathVariable UUID id) {
        return repository.findById(id)
                .map(ResponseEntity::ok)
                .orElseGet(() -> ResponseEntity.notFound().build());
    }
}
