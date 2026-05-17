package com.banka1.tradingservice.otc.service;

import com.banka1.order.client.StockClient;
import com.banka1.order.dto.StockListingDto;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.repository.PortfolioRepository;
import com.banka1.tradingservice.otc.domain.OptionContract;
import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import com.banka1.tradingservice.otc.domain.OtcOffer;
import com.banka1.tradingservice.otc.domain.OtcOfferStatus;
import com.banka1.tradingservice.otc.exception.InsufficientPublicStockException;
import com.banka1.tradingservice.otc.repository.OptionContractRepository;
import com.banka1.tradingservice.otc.repository.OtcOfferRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * PR_32 Phase 12 KRIT #2 + #3: verifikuje reserved-stock invariant i da se
 * OptionContract kreira sa statusom PENDING_PREMIUM (ne ACTIVE).
 */
@ExtendWith(MockitoExtension.class)
class OtcServiceSumInvariantTest {

    @Mock private OtcOfferRepository otcOfferRepository;
    @Mock private OptionContractRepository optionContractRepository;
    @Mock private PortfolioRepository portfolioRepository;
    @Mock private StockClient stockClient;
    @Mock private RabbitTemplate rabbitTemplate;

    @InjectMocks private OtcService service;

    @Test
    void accept_kreiraOptionContractSaStatusomPendingPremium() {
        OtcOffer offer = newOffer(20);
        when(otcOfferRepository.findById(1L)).thenReturn(Optional.of(offer));

        Portfolio sellerPos = newPortfolio(200L, 500L, 50);
        when(portfolioRepository.findByUserId(200L)).thenReturn(List.of(sellerPos));
        when(stockClient.getListing(500L)).thenReturn(stockListing(500L, "AAPL"));

        when(optionContractRepository.sumActiveBySellerAndTicker(200L, "AAPL")).thenReturn(0L);
        when(optionContractRepository.save(any(OptionContract.class)))
                .thenAnswer(inv -> {
                    OptionContract c = inv.getArgument(0);
                    c.setId(99L);
                    return c;
                });

        service.accept(1L, 200L);

        ArgumentCaptor<OptionContract> captor = ArgumentCaptor.forClass(OptionContract.class);
        verify(optionContractRepository).save(captor.capture());
        assertThat(captor.getValue().getStatus()).isEqualTo(OptionContractStatus.PENDING_PREMIUM);
        assertThat(offer.getStatus()).isEqualTo(OtcOfferStatus.ACCEPTED);
    }

    @Test
    void accept_throws_kadSumPlusOfferPrelaziPortfolio() {
        OtcOffer offer = newOffer(30);
        when(otcOfferRepository.findById(1L)).thenReturn(Optional.of(offer));

        Portfolio sellerPos = newPortfolio(200L, 500L, 50);
        when(portfolioRepository.findByUserId(200L)).thenReturn(List.of(sellerPos));
        when(stockClient.getListing(500L)).thenReturn(stockListing(500L, "AAPL"));

        // 25 vec rezervisanih + 30 nova = 55 > 50 owned
        when(optionContractRepository.sumActiveBySellerAndTicker(200L, "AAPL")).thenReturn(25L);

        assertThatThrownBy(() -> service.accept(1L, 200L))
                .isInstanceOf(InsufficientPublicStockException.class)
                .hasMessageContaining("Reserved-stock invariant violated");

        verify(optionContractRepository, never()).save(any(OptionContract.class));
    }

    @Test
    void accept_throws_kadProdavacNemaPoziciju() {
        OtcOffer offer = newOffer(5);
        when(otcOfferRepository.findById(1L)).thenReturn(Optional.of(offer));

        when(portfolioRepository.findByUserId(200L)).thenReturn(List.of());
        when(optionContractRepository.sumActiveBySellerAndTicker(200L, "AAPL")).thenReturn(0L);

        assertThatThrownBy(() -> service.accept(1L, 200L))
                .isInstanceOf(InsufficientPublicStockException.class)
                .hasMessageContaining("Reserved-stock invariant violated");
    }

    @Test
    void accept_dozvoliKadJeSumPlusOfferTacnoUOkviru() {
        OtcOffer offer = newOffer(25);
        when(otcOfferRepository.findById(1L)).thenReturn(Optional.of(offer));

        Portfolio sellerPos = newPortfolio(200L, 500L, 50);
        when(portfolioRepository.findByUserId(200L)).thenReturn(List.of(sellerPos));
        when(stockClient.getListing(500L)).thenReturn(stockListing(500L, "AAPL"));

        // 25 + 25 = 50 (tacno na granici, dozvoljeno)
        when(optionContractRepository.sumActiveBySellerAndTicker(200L, "AAPL")).thenReturn(25L);
        when(optionContractRepository.save(any(OptionContract.class)))
                .thenAnswer(inv -> {
                    OptionContract c = inv.getArgument(0);
                    c.setId(99L);
                    return c;
                });

        service.accept(1L, 200L);
        verify(optionContractRepository).save(any(OptionContract.class));
    }

    private static OtcOffer newOffer(int amount) {
        OtcOffer offer = new OtcOffer();
        offer.setId(1L);
        offer.setBuyerId(100L);
        offer.setSellerId(200L);
        offer.setStockTicker("AAPL");
        offer.setAmount(amount);
        offer.setPricePerStock(new BigDecimal("150"));
        offer.setPremium(new BigDecimal("400"));
        offer.setSettlementDate(LocalDate.now().plusMonths(2));
        offer.setStatus(OtcOfferStatus.PENDING_SELLER);
        return offer;
    }

    private static Portfolio newPortfolio(long userId, long listingId, int quantity) {
        Portfolio p = new Portfolio();
        p.setId(42L);
        p.setUserId(userId);
        p.setListingId(listingId);
        p.setQuantity(quantity);
        p.setReservedQuantity(0);
        return p;
    }

    private static StockListingDto stockListing(long id, String ticker) {
        StockListingDto dto = new StockListingDto();
        dto.setId(id);
        dto.setTicker(ticker);
        return dto;
    }
}
