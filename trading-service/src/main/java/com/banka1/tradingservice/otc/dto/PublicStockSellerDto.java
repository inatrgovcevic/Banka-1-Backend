package com.banka1.tradingservice.otc.dto;

import lombok.AllArgsConstructor;
import lombok.Data;

@Data
@AllArgsConstructor
public class PublicStockSellerDto {
    private Long sellerId;
    private String sellerName;
    private Integer availableQuantity;
}