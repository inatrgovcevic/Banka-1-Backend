package com.banka1.tradingservice.otc.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class OtcPositionDto {
    private Long id;
    private Long listingId;
    private String stockTicker;
    private Integer totalQuantity;
    private Integer reservedQuantity;
    private Integer publicQuantity;
    private Integer availableQuantity;
}
