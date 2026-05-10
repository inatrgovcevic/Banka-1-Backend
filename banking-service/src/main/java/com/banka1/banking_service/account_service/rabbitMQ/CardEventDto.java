package com.banka1.banking_service.account_service.rabbitMQ;

import com.banka1.banking_service.account_service.rabbitMQ.CardEventType;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class CardEventDto {
    private Long clientId;
    private String accountNumber;
    private CardEventType eventType;
}
