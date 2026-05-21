package com.banka1.order.service.impl;

import com.banka1.order.client.ClientClient;
import com.banka1.order.client.EmployeeClient;
import com.banka1.order.dto.AuthenticatedUser;
import com.banka1.order.dto.CreateOtcNegotiationRequest;
import com.banka1.order.dto.CustomerDto;
import com.banka1.order.dto.EmployeeDto;
import com.banka1.order.dto.UpdateOtcNegotiationRequest;
import com.banka1.order.entity.OtcNegotiation;
import com.banka1.order.entity.OtcNegotiationHistory;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.entity.enums.ListingType;
import com.banka1.order.entity.enums.OtcNegotiationEventType;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import com.banka1.order.rabbitmq.OrderNotificationProducer;
import com.banka1.order.repository.OtcNegotiationHistoryRepository;
import com.banka1.order.repository.OtcNegotiationRepository;
import com.banka1.order.repository.PortfolioRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;

import java.math.BigDecimal;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class OtcNegotiationServiceImplTest {

    @Mock
    private OtcNegotiationRepository otcNegotiationRepository;
    @Mock
    private OtcNegotiationHistoryRepository otcNegotiationHistoryRepository;
    @Mock
    private PortfolioRepository portfolioRepository;
    @Mock
    private ClientClient clientClient;
    @Mock
    private EmployeeClient employeeClient;
    @Mock
    private OrderNotificationProducer notificationProducer;

    private OtcNegotiationServiceImpl service;
    private Clock fixedClock;
    private LocalDate today;

    @BeforeEach
    void setUp() {
        fixedClock = Clock.fixed(Instant.parse("2026-05-19T10:15:30Z"), ZoneOffset.UTC);
        today = LocalDate.now(fixedClock);
        service = new OtcNegotiationServiceImpl(
                otcNegotiationRepository,
                otcNegotiationHistoryRepository,
                portfolioRepository,
                clientClient,
                employeeClient,
                notificationProducer,
                fixedClock
        );
        ReflectionTestUtils.setField(service, "expirationNotificationDays", 3);
    }

    @Test
    void createNegotiationPersistsInitialStateAndHistory() {
        Portfolio sellerPortfolio = publicSellerPortfolio();
        when(portfolioRepository.findById(5L)).thenReturn(Optional.of(sellerPortfolio));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> {
            OtcNegotiation negotiation = invocation.getArgument(0);
            negotiation.setId(10L);
            return negotiation;
        });
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class)))
                .thenAnswer(invocation -> invocation.getArgument(0));

        CreateOtcNegotiationRequest request = new CreateOtcNegotiationRequest();
        request.setSellerPortfolioId(5L);
        request.setQuantity(4);
        request.setPricePerUnit(new BigDecimal("123.4567"));
        request.setContractExpiryDate(today.plusDays(10));

        var response = service.createNegotiation(user(11L), request);

        assertThat(response.getId()).isEqualTo(10L);
        assertThat(response.getBuyerId()).isEqualTo(11L);
        assertThat(response.getSellerId()).isEqualTo(22L);
        assertThat(response.getStatus()).isEqualTo(OtcNegotiationStatus.OPEN);

        ArgumentCaptor<OtcNegotiationHistory> historyCaptor = ArgumentCaptor.forClass(OtcNegotiationHistory.class);
        verify(otcNegotiationHistoryRepository).save(historyCaptor.capture());
        assertThat(historyCaptor.getValue().getEventType()).isEqualTo(OtcNegotiationEventType.CREATED);
        assertThat(historyCaptor.getValue().getResultingStatus()).isEqualTo(OtcNegotiationStatus.OPEN);
        assertThat(historyCaptor.getValue().getPreviousQuantity()).isNull();
    }

    @Test
    void counterOfferUpdatesStateStoresHistoryAndSendsNotification() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setId(7L);
        negotiation.setLastUpdatedByUserId(11L);
        negotiation.setStatus(OtcNegotiationStatus.OPEN);
        when(otcNegotiationRepository.findByIdForUpdate(7L)).thenReturn(Optional.of(negotiation));
        when(portfolioRepository.findById(5L)).thenReturn(Optional.of(publicSellerPortfolio()));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(clientClient.getCustomer(11L)).thenReturn(customer(11L, "Kupac", "kupac@example.com"));

        UpdateOtcNegotiationRequest request = new UpdateOtcNegotiationRequest();
        request.setQuantity(3);
        request.setPricePerUnit(new BigDecimal("140"));
        request.setContractExpiryDate(today.plusDays(12));

        var response = service.counterOffer(user(22L), 7L, request);

        assertThat(response.getStatus()).isEqualTo(OtcNegotiationStatus.COUNTERED);
        assertThat(negotiation.getQuantity()).isEqualTo(3);
        assertThat(negotiation.getPricePerUnit()).isEqualByComparingTo("140.0000");

        ArgumentCaptor<OtcNegotiationHistory> historyCaptor = ArgumentCaptor.forClass(OtcNegotiationHistory.class);
        verify(otcNegotiationHistoryRepository).save(historyCaptor.capture());
        assertThat(historyCaptor.getValue().getEventType()).isEqualTo(OtcNegotiationEventType.COUNTEROFFERED);
        assertThat(historyCaptor.getValue().getPreviousQuantity()).isEqualTo(2);
        assertThat(historyCaptor.getValue().getResultingStatus()).isEqualTo(OtcNegotiationStatus.COUNTERED);
        verify(notificationProducer).sendOtcCounterofferCreated(any());
    }

    @Test
    void acceptNegotiationReservesPortfolioStoresHistoryAndSendsNotification() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setId(9L);
        negotiation.setLastUpdatedByUserId(22L);
        Portfolio portfolio = publicSellerPortfolio();
        when(otcNegotiationRepository.findByIdForUpdate(9L)).thenReturn(Optional.of(negotiation));
        when(portfolioRepository.findByIdForUpdate(5L)).thenReturn(Optional.of(portfolio));
        when(portfolioRepository.save(any(Portfolio.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(employeeClient.getEmployee(22L)).thenReturn(employee(22L, "Prodavac", "prodavac@example.com"));

        var response = service.acceptNegotiation(user(11L), 9L);

        assertThat(response.getStatus()).isEqualTo(OtcNegotiationStatus.ACCEPTED);
        assertThat(portfolio.getReservedQuantity()).isEqualTo(2);
        assertThat(portfolio.getPublicQuantity()).isEqualTo(8);

        ArgumentCaptor<OtcNegotiationHistory> historyCaptor = ArgumentCaptor.forClass(OtcNegotiationHistory.class);
        verify(otcNegotiationHistoryRepository).save(historyCaptor.capture());
        assertThat(historyCaptor.getValue().getEventType()).isEqualTo(OtcNegotiationEventType.ACCEPTED);
        verify(notificationProducer).sendOtcOfferAccepted(any());
    }

    @Test
    void declineNegotiationStoresHistoryAndSendsNotification() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setId(12L);
        negotiation.setLastUpdatedByUserId(22L);
        when(otcNegotiationRepository.findByIdForUpdate(12L)).thenReturn(Optional.of(negotiation));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(employeeClient.getEmployee(22L)).thenReturn(employee(22L, "Prodavac", "prodavac@example.com"));

        var response = service.declineNegotiation(user(11L), 12L);

        assertThat(response.getStatus()).isEqualTo(OtcNegotiationStatus.DECLINED);
        verify(notificationProducer).sendOtcOfferDeclined(any());
    }

    @Test
    void cancelNegotiationStoresHistoryAndSendsNotification() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setId(13L);
        when(otcNegotiationRepository.findByIdForUpdate(13L)).thenReturn(Optional.of(negotiation));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(employeeClient.getEmployee(22L)).thenReturn(employee(22L, "Prodavac", "prodavac@example.com"));

        var response = service.cancelNegotiation(user(11L), 13L);

        assertThat(response.getStatus()).isEqualTo(OtcNegotiationStatus.CANCELLED);
        verify(notificationProducer).sendOtcOfferCancelled(any());
    }

    @Test
    void notifyContractsExpiringSoonSendsNotificationsToBothPartiesAndMarksRow() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setId(15L);
        negotiation.setStatus(OtcNegotiationStatus.ACCEPTED);
        negotiation.setContractExpiryDate(today.plusDays(3));
        when(otcNegotiationRepository.findByStatusAndContractExpiryDate(
                OtcNegotiationStatus.ACCEPTED, today.plusDays(3)
        )).thenReturn(List.of(negotiation));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(clientClient.getCustomer(11L)).thenReturn(customer(11L, "Kupac", "kupac@example.com"));
        when(employeeClient.getEmployee(22L)).thenReturn(employee(22L, "Prodavac", "prodavac@example.com"));

        service.notifyContractsExpiringSoon();

        assertThat(negotiation.getExpirationNotifiedAt()).isEqualTo(today.plusDays(3));
        verify(notificationProducer, times(2)).sendOtcContractExpiring(any());
    }

    @Test
    void expireOverdueContractsReleasesReservationsAndWritesHistory() {
        OtcNegotiation negotiation = existingNegotiation();
        negotiation.setStatus(OtcNegotiationStatus.ACCEPTED);
        negotiation.setContractExpiryDate(today.minusDays(1));
        Portfolio portfolio = publicSellerPortfolio();
        portfolio.setReservedQuantity(5);
        when(otcNegotiationRepository.findByStatusAndContractExpiryDateBefore(
                OtcNegotiationStatus.ACCEPTED, today
        )).thenReturn(List.of(negotiation));
        when(portfolioRepository.findByIdForUpdate(5L)).thenReturn(Optional.of(portfolio));
        when(portfolioRepository.save(any(Portfolio.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationRepository.save(any(OtcNegotiation.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(otcNegotiationHistoryRepository.save(any(OtcNegotiationHistory.class))).thenAnswer(invocation -> invocation.getArgument(0));

        service.expireOverdueContracts();

        assertThat(negotiation.getStatus()).isEqualTo(OtcNegotiationStatus.EXPIRED);
        assertThat(portfolio.getReservedQuantity()).isEqualTo(3);

        ArgumentCaptor<OtcNegotiationHistory> historyCaptor = ArgumentCaptor.forClass(OtcNegotiationHistory.class);
        verify(otcNegotiationHistoryRepository).save(historyCaptor.capture());
        assertThat(historyCaptor.getValue().getEventType()).isEqualTo(OtcNegotiationEventType.EXPIRED);
    }

    private AuthenticatedUser user(Long userId) {
        return new AuthenticatedUser(userId, java.util.Set.of("CLIENT_TRADING"), java.util.Set.of("trading"));
    }

    private Portfolio publicSellerPortfolio() {
        Portfolio portfolio = new Portfolio();
        portfolio.setId(5L);
        portfolio.setUserId(22L);
        portfolio.setListingId(99L);
        portfolio.setListingType(ListingType.STOCK);
        portfolio.setQuantity(10);
        portfolio.setReservedQuantity(0);
        portfolio.setPublicQuantity(10);
        portfolio.setIsPublic(true);
        portfolio.setAveragePurchasePrice(new BigDecimal("100"));
        return portfolio;
    }

    private OtcNegotiation existingNegotiation() {
        OtcNegotiation negotiation = new OtcNegotiation();
        negotiation.setId(1L);
        negotiation.setBuyerId(11L);
        negotiation.setSellerId(22L);
        negotiation.setSellerPortfolioId(5L);
        negotiation.setListingId(99L);
        negotiation.setQuantity(2);
        negotiation.setPricePerUnit(new BigDecimal("120.0000"));
        negotiation.setContractExpiryDate(today.plusDays(8));
        negotiation.setCreatedByUserId(11L);
        negotiation.setLastUpdatedByUserId(11L);
        negotiation.setStatus(OtcNegotiationStatus.OPEN);
        negotiation.setCreatedAt(java.time.LocalDateTime.now().minusDays(1));
        negotiation.setUpdatedAt(java.time.LocalDateTime.now().minusHours(2));
        return negotiation;
    }

    private CustomerDto customer(Long id, String firstName, String email) {
        CustomerDto customer = new CustomerDto();
        customer.setId(id);
        customer.setFirstName(firstName);
        customer.setLastName("Kupic");
        customer.setEmail(email);
        return customer;
    }

    private EmployeeDto employee(Long id, String firstName, String email) {
        EmployeeDto employee = new EmployeeDto();
        employee.setId(id);
        employee.setIme(firstName);
        employee.setPrezime("Prodic");
        employee.setEmail(email);
        return employee;
    }
}
