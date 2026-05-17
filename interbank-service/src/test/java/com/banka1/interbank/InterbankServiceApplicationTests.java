package com.banka1.interbank;

import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.client.UserInternalClient;
import com.banka1.interbank.service.InterbankClient;
import com.banka1.interbank.service.InterbankCoordinatorService;
import com.banka1.interbank.service.TransactionExecutorService;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;

/**
 * PR_32 Phase 1 sanity test — verifikuje da Spring kontekst ucita sa
 * application-test.properties (H2, Liquibase off).
 *
 * <p>{@code TransactionExecutorService} ima {@code @Profile("!test")} (svesno,
 * jer wire-uje banking-core/trading RestClient bean-ove koji nisu dostupni u
 * test profilu). Posle Phase 7, {@link com.banka1.interbank.controller.InboundDispatcher}
 * zavisi od njega — mocka-ujemo ga ovde da context-load prodje.
 */
@SpringBootTest
@ActiveProfiles("test")
class InterbankServiceApplicationTests {

    @MockitoBean
    private TransactionExecutorService transactionExecutorService;

    @MockitoBean
    private InterbankCoordinatorService interbankCoordinatorService;

    @MockitoBean
    private TradingInternalClient tradingInternalClient;

    @MockitoBean
    private UserInternalClient userInternalClient;

    // PR_33 Phase A: InterbankOtcOutboundService (component) zavisi od
    // InterbankClient (@Profile("!test")) — mock-ujemo ga da context-load prodje.
    @MockitoBean
    private InterbankClient interbankClient;

    @Test
    void contextLoads() {
        // Intentionally empty — sve sto treba je da Spring uspe da kreira kontekst.
    }
}
