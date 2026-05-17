package com.banka1.tradingservice.interbank.service;

import com.banka1.order.client.StockClient;
import com.banka1.order.dto.StockListingDto;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.repository.PortfolioRepository;
import com.banka1.tradingservice.interbank.model.InterbankStockReservation;
import com.banka1.tradingservice.interbank.repository.InterbankStockReservationRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Unit testovi za PR_32 Phase 12: InterbankStockReservationService (reserve/commit/release).
 */
@ExtendWith(MockitoExtension.class)
class InterbankStockReservationServiceTest {

    @Mock private InterbankStockReservationRepository reservationRepository;
    @Mock private PortfolioRepository portfolioRepository;
    @Mock private StockClient stockClient;

    @InjectMocks private InterbankStockReservationService service;

    private Portfolio portfolio;

    @BeforeEach
    void setUp() {
        portfolio = new Portfolio();
        portfolio.setId(42L);
        portfolio.setUserId(100L);
        portfolio.setListingId(500L);
        portfolio.setQuantity(50);
        portfolio.setReservedQuantity(0);

        // Lenient: ne sve test metode koriste save() (npr. one koje throws pre save).
        lenient().when(reservationRepository.save(any(InterbankStockReservation.class)))
                .thenAnswer(inv -> inv.getArgument(0));
        lenient().when(portfolioRepository.save(any(Portfolio.class)))
                .thenAnswer(inv -> inv.getArgument(0));
    }

    @Test
    void reserveStock_inkrementujeReservedQuantity_iVracaReservationId() {
        when(portfolioRepository.findByUserId(100L)).thenReturn(List.of(portfolio));
        StockListingDto listing = new StockListingDto();
        listing.setId(500L);
        listing.setTicker("AAPL");
        when(stockClient.getListing(500L)).thenReturn(listing);
        when(portfolioRepository.findByUserIdAndListingIdForUpdate(100L, 500L))
                .thenReturn(Optional.of(portfolio));

        UUID reservationId = service.reserveStock(100L, "AAPL", 10, 111, "tx-local-1");

        assertThat(reservationId).isNotNull();
        assertThat(portfolio.getReservedQuantity()).isEqualTo(10);
        assertThat(portfolio.getQuantity()).isEqualTo(50);

        ArgumentCaptor<InterbankStockReservation> cap = ArgumentCaptor.forClass(InterbankStockReservation.class);
        verify(reservationRepository).save(cap.capture());
        assertThat(cap.getValue().getStatus()).isEqualTo("HELD");
        assertThat(cap.getValue().getQuantity()).isEqualTo(10);
        assertThat(cap.getValue().getTicker()).isEqualTo("AAPL");
    }

    @Test
    void reserveStock_throws_kadNemaDovoljnoAvailableQuantity() {
        portfolio.setReservedQuantity(45);
        when(portfolioRepository.findByUserId(100L)).thenReturn(List.of(portfolio));
        StockListingDto listing = new StockListingDto();
        listing.setId(500L);
        listing.setTicker("AAPL");
        when(stockClient.getListing(500L)).thenReturn(listing);
        when(portfolioRepository.findByUserIdAndListingIdForUpdate(100L, 500L))
                .thenReturn(Optional.of(portfolio));

        assertThatThrownBy(() -> service.reserveStock(100L, "AAPL", 10, 111, "tx-local-1"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Insufficient stock");
    }

    @Test
    void reserveStock_throws_kadPozicijaNijeNadjena() {
        when(portfolioRepository.findByUserId(100L)).thenReturn(List.of());

        assertThatThrownBy(() -> service.reserveStock(100L, "AAPL", 10, 111, "tx-local-1"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("No portfolio position");
    }

    @Test
    void commitStock_skidaQuantityIReservedQuantity() {
        UUID resId = UUID.randomUUID();
        portfolio.setReservedQuantity(10);
        InterbankStockReservation reservation = InterbankStockReservation.builder()
                .reservationId(resId)
                .portfolioId(42L)
                .ticker("AAPL")
                .quantity(10)
                .status("HELD")
                .build();
        when(reservationRepository.findByReservationId(resId)).thenReturn(Optional.of(reservation));
        when(portfolioRepository.findById(42L)).thenReturn(Optional.of(portfolio));

        service.commitStock(resId);

        assertThat(portfolio.getQuantity()).isEqualTo(40);
        assertThat(portfolio.getReservedQuantity()).isEqualTo(0);
        assertThat(reservation.getStatus()).isEqualTo("COMMITTED");
        assertThat(reservation.getFinalizedAt()).isNotNull();
    }

    @Test
    void commitStock_idempotent_kadJeVecCommitted() {
        UUID resId = UUID.randomUUID();
        InterbankStockReservation reservation = InterbankStockReservation.builder()
                .reservationId(resId)
                .portfolioId(42L)
                .quantity(10)
                .status("COMMITTED")
                .build();
        when(reservationRepository.findByReservationId(resId)).thenReturn(Optional.of(reservation));

        service.commitStock(resId);

        // Portfolio se ne menja
        assertThat(portfolio.getQuantity()).isEqualTo(50);
        assertThat(portfolio.getReservedQuantity()).isEqualTo(0);
    }

    @Test
    void releaseStock_vracaSamoReservedQuantity_neDiraQuantity() {
        UUID resId = UUID.randomUUID();
        portfolio.setReservedQuantity(10);
        InterbankStockReservation reservation = InterbankStockReservation.builder()
                .reservationId(resId)
                .portfolioId(42L)
                .ticker("AAPL")
                .quantity(10)
                .status("HELD")
                .build();
        when(reservationRepository.findByReservationId(resId)).thenReturn(Optional.of(reservation));
        when(portfolioRepository.findById(42L)).thenReturn(Optional.of(portfolio));

        service.releaseStock(resId);

        assertThat(portfolio.getQuantity()).isEqualTo(50);
        assertThat(portfolio.getReservedQuantity()).isEqualTo(0);
        assertThat(reservation.getStatus()).isEqualTo("RELEASED");
    }

    @Test
    void releaseStock_throws_kadJeVecCommitted() {
        UUID resId = UUID.randomUUID();
        InterbankStockReservation reservation = InterbankStockReservation.builder()
                .reservationId(resId)
                .portfolioId(42L)
                .quantity(10)
                .status("COMMITTED")
                .build();
        when(reservationRepository.findByReservationId(resId)).thenReturn(Optional.of(reservation));

        assertThatThrownBy(() -> service.releaseStock(resId))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("already COMMITTED");
    }
}
