package com.banka1.tradingservice.otc.dto;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class CreateOtcPositionRequest {

    @NotNull
    private Long listingId;

    @NotNull
    @Min(1)
    private Integer publicQuantity;
}
