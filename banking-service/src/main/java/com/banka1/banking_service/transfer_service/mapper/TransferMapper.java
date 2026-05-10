package com.banka1.banking_service.transfer_service.mapper;

import com.banka1.banking_service.transfer_service.domain.Transfer;
import com.banka1.banking_service.transfer_service.dto.requests.TransferRequestDto;
import com.banka1.banking_service.transfer_service.dto.responses.TransferResponseDto;
import org.springframework.stereotype.Component;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Komponenta zadužena za transformaciju podataka između DTO objekata i baze podataka (Entiteta).
 * Koristi se za razdvajanje sloja API-ja od perzistentnog sloja.
 */
@Component
public class TransferMapper {

    /**
     * Mapira ulazni zahtev i izračunate vrednosti (iznose, kurseve, provizije) u entitet spreman za bazu.
     * @return {@link Transfer} entitet sa popunjenim poljima i trenutnim vremenom
     */
    public Transfer toEntity(
            TransferRequestDto request,
            String orderNumber,
            Long clientId,
            BigDecimal finalAmount,
            BigDecimal exchangeRate,
            BigDecimal commission
    ) {
        return Transfer.builder()
                .orderNumber(orderNumber)
                .clientId(clientId)
                .fromAccountNumber(request.getFromAccountNumber())
                .toAccountNumber(request.getToAccountNumber())
                .initialAmount(request.getAmount())
                .finalAmount(finalAmount)
                .exchangeRate(exchangeRate)
                .commission(commission)
                .verificationSessionId(request.getVerificationSessionId().toString())
                .timestamp(Instant.now())
                .build();
    }

    /**
     * Mapira entitet iz baze podataka u format pogodan za povratni odgovor klijentu.
     * @param transfer perzistentni podaci o transferu
     * @return {@link TransferResponseDto} za prikaz na klijentskoj strani
     */
    public TransferResponseDto toDto(Transfer transfer) {
        TransferResponseDto dto = new TransferResponseDto();
        dto.setOrderNumber(transfer.getOrderNumber());
        dto.setFromAccountNumber(transfer.getFromAccountNumber());
        dto.setToAccountNumber(transfer.getToAccountNumber());
        dto.setInitialAmount(transfer.getInitialAmount());
        dto.setFinalAmount(transfer.getFinalAmount());
        dto.setExchangeRate(transfer.getExchangeRate());
        dto.setCommission(transfer.getCommission());
        dto.setTimestamp(transfer.getTimestamp());
        return dto;
    }
}