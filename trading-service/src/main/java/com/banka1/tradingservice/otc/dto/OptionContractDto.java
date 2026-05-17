package com.banka1.tradingservice.otc.dto;

import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Response DTO za sklopljen ugovor (PR_13 C13.3).
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class OptionContractDto {
    private Long id;
    private Long offerId;
    private String stockTicker;
    private Long buyerId;
    private Long sellerId;
    private Integer amount;
    private BigDecimal pricePerStock;
    private LocalDate settlementDate;
    private OptionContractStatus status;
    private LocalDateTime createdAt;
    private LocalDateTime exercisedAt;
}
