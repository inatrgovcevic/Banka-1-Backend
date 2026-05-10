package com.banka1.banking_service.card_service.rabbitMQ;

import com.banka1.banking_service.card_service.dto.card_creation.request.AutoCardCreationRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.response.CardCreationResponseDto;
import com.banka1.banking_service.card_service.service.CardRequestService;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.time.LocalDate;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class CardCreationEventListenerTest {

    @Mock
    private CardRequestService cardRequestService;

    @Test
    void handleCardCreateEventDelegatesToAutomaticCardCreationFlow() {
        CardCreationEventListener listener = new CardCreationEventListener(cardRequestService);
        CardCreationEventDto event = new CardCreationEventDto(25L, "265000000000123456", "CARD_CREATE");

        when(cardRequestService.createAutomaticCard(org.mockito.ArgumentMatchers.any(AutoCardCreationRequestDto.class)))
                .thenReturn(new CardCreationResponseDto(
                        44L,
                        "4111111111111111",
                        "123",
                        LocalDate.of(2031, 4, 13),
                        "Visa Debit"
                ));

        listener.handleCardCreateEvent(event, "card.create");

        ArgumentCaptor<AutoCardCreationRequestDto> captor = ArgumentCaptor.forClass(AutoCardCreationRequestDto.class);
        verify(cardRequestService).createAutomaticCard(captor.capture());
        assertEquals(25L, captor.getValue().getClientId());
        assertEquals("265000000000123456", captor.getValue().getAccountNumber());
    }
}
