package com.banka1.tradingservice.otc.dto;

import lombok.AllArgsConstructor;
import lombok.Data;

import java.util.List;

@Data
@AllArgsConstructor
public class PublicStockDto {
    private String ticker;
    private List<PublicStockSellerDto> sellers;
}