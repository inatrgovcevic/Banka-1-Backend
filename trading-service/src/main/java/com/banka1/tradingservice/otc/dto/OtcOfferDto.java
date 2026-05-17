package com.banka1.tradingservice.otc.dto;

import com.banka1.tradingservice.otc.domain.OtcOfferStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * REST response DTO za OTC ponudu (PR_04).
 * Spec: Celina 4.txt, Entitet ponude (sva polja).
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class OtcOfferDto {
    private Long id;
    private String stockTicker;
    private Long buyerId;
    private Long sellerId;
    private Integer amount;
    private BigDecimal pricePerStock;
    private BigDecimal premium;
    private LocalDate settlementDate;
    private OtcOfferStatus status;
    private String modifiedBy;
    private LocalDateTime lastModified;
}
