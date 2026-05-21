package com.banka1.order.controller;

import com.banka1.order.dto.CreateOtcNegotiationRequest;
import com.banka1.order.dto.OtcNegotiationHistoryResponse;
import com.banka1.order.dto.OtcNegotiationResponse;
import com.banka1.order.dto.UpdateOtcNegotiationRequest;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import com.banka1.order.service.OtcNegotiationService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;

import java.lang.reflect.Method;
import java.time.LocalDate;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class OtcNegotiationControllerTest {

    @Mock
    private OtcNegotiationService otcNegotiationService;

    private OtcNegotiationController controller;

    @BeforeEach
    void setUp() {
        controller = new OtcNegotiationController(otcNegotiationService);
    }

    @Test
    void createAndListEndpointsDelegateAuthenticatedUser() {
        OtcNegotiationResponse response = new OtcNegotiationResponse();
        response.setId(3L);
        when(otcNegotiationService.createNegotiation(any(), any())).thenReturn(response);
        when(otcNegotiationService.listNegotiations(any(), any(), any(), any(), any())).thenReturn(List.of(response));

        ResponseEntity<OtcNegotiationResponse> createResponse = controller.createNegotiation(jwt(), new CreateOtcNegotiationRequest());
        ResponseEntity<List<OtcNegotiationResponse>> listResponse = controller.listNegotiations(jwt(), OtcNegotiationStatus.OPEN,
                LocalDate.now().minusDays(1), LocalDate.now(), 22L);

        assertThat(createResponse.getBody().getId()).isEqualTo(3L);
        assertThat(listResponse.getBody()).hasSize(1);
        verify(otcNegotiationService).createNegotiation(any(), any());
        verify(otcNegotiationService).listNegotiations(any(), any(), any(), any(), any());
    }

    @Test
    void historyEndpointDelegates() {
        OtcNegotiationHistoryResponse history = new OtcNegotiationHistoryResponse();
        history.setNegotiationId(8L);
        when(otcNegotiationService.getNegotiationHistory(any(), any())).thenReturn(List.of(history));

        ResponseEntity<List<OtcNegotiationHistoryResponse>> response = controller.getHistory(jwt(), 8L);

        assertThat(response.getBody()).hasSize(1);
        assertThat(response.getBody().getFirst().getNegotiationId()).isEqualTo(8L);
        verify(otcNegotiationService).getNegotiationHistory(any(), any());
    }

    @Test
    void otcEndpointsHaveExpectedMappingsAndSecurity() throws Exception {
        Method create = OtcNegotiationController.class.getDeclaredMethod("createNegotiation", Jwt.class, CreateOtcNegotiationRequest.class);
        assertThat(create.getAnnotation(PostMapping.class).value()).isEmpty();
        assertThat(create.getAnnotation(PreAuthorize.class).value()).isEqualTo("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')");

        Method counter = OtcNegotiationController.class.getDeclaredMethod("counterOffer", Jwt.class, Long.class, UpdateOtcNegotiationRequest.class);
        assertThat(counter.getAnnotation(PostMapping.class).value()).containsExactly("/{id}/counteroffer");

        Method accept = OtcNegotiationController.class.getDeclaredMethod("acceptNegotiation", Jwt.class, Long.class);
        assertThat(accept.getAnnotation(PostMapping.class).value()).containsExactly("/{id}/accept");

        Method decline = OtcNegotiationController.class.getDeclaredMethod("declineNegotiation", Jwt.class, Long.class);
        assertThat(decline.getAnnotation(PostMapping.class).value()).containsExactly("/{id}/decline");

        Method cancel = OtcNegotiationController.class.getDeclaredMethod("cancelNegotiation", Jwt.class, Long.class);
        assertThat(cancel.getAnnotation(PostMapping.class).value()).containsExactly("/{id}/cancel");

        Method list = OtcNegotiationController.class.getDeclaredMethod("listNegotiations", Jwt.class, OtcNegotiationStatus.class, LocalDate.class, LocalDate.class, Long.class);
        assertThat(list.getAnnotation(GetMapping.class).value()).isEmpty();

        Method history = OtcNegotiationController.class.getDeclaredMethod("getHistory", Jwt.class, Long.class);
        assertThat(history.getAnnotation(GetMapping.class).value()).containsExactly("/{id}/history");
    }

    private Jwt jwt() {
        return Jwt.withTokenValue("token")
                .subject("15")
                .claim("roles", List.of("CLIENT_TRADING"))
                .claim("permissions", List.of("trading"))
                .header("alg", "none")
                .build();
    }
}
