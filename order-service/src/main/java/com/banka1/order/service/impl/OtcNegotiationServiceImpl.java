package com.banka1.order.service.impl;

import com.banka1.order.client.ClientClient;
import com.banka1.order.client.EmployeeClient;
import com.banka1.order.dto.AuthenticatedUser;
import com.banka1.order.dto.CreateOtcNegotiationRequest;
import com.banka1.order.dto.CustomerDto;
import com.banka1.order.dto.EmployeeDto;
import com.banka1.order.dto.OtcNegotiationHistoryResponse;
import com.banka1.order.dto.OtcNegotiationResponse;
import com.banka1.order.dto.UpdateOtcNegotiationRequest;
import com.banka1.order.entity.OtcNegotiation;
import com.banka1.order.entity.OtcNegotiationHistory;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.entity.enums.ListingType;
import com.banka1.order.entity.enums.OtcNegotiationEventType;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import com.banka1.order.exception.BadRequestException;
import com.banka1.order.exception.BusinessConflictException;
import com.banka1.order.exception.ForbiddenOperationException;
import com.banka1.order.exception.ResourceNotFoundException;
import com.banka1.order.rabbitmq.OrderNotificationProducer;
import com.banka1.order.repository.OtcNegotiationHistoryRepository;
import com.banka1.order.repository.OtcNegotiationRepository;
import com.banka1.order.repository.PortfolioRepository;
import com.banka1.order.service.OtcNegotiationService;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Clock;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.LocalDate;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Default OTC negotiation workflow implementation.
 */
@Service
@RequiredArgsConstructor
@Slf4j
public class OtcNegotiationServiceImpl implements OtcNegotiationService {

    private final OtcNegotiationRepository otcNegotiationRepository;
    private final OtcNegotiationHistoryRepository otcNegotiationHistoryRepository;
    private final PortfolioRepository portfolioRepository;
    private final ClientClient clientClient;
    private final EmployeeClient employeeClient;
    private final OrderNotificationProducer notificationProducer;
    private final Clock clock;

    @Value("${otc.contract.expiration-notification-days:3}")
    private int expirationNotificationDays;

    @Override
    @Transactional
    public OtcNegotiationResponse createNegotiation(AuthenticatedUser user, CreateOtcNegotiationRequest request) {
        validateOfferTerms(request.getQuantity(), request.getPricePerUnit(), request.getContractExpiryDate());

        Portfolio sellerPortfolio = portfolioRepository.findById(request.getSellerPortfolioId())
                .orElseThrow(() -> new ResourceNotFoundException("Seller portfolio not found"));
        validateSellerPortfolioForNegotiation(sellerPortfolio, request.getQuantity());
        if (sellerPortfolio.getUserId().equals(user.userId())) {
            throw new BusinessConflictException("Cannot create OTC negotiation against your own portfolio");
        }

        OtcNegotiation negotiation = new OtcNegotiation();
        negotiation.setBuyerId(user.userId());
        negotiation.setSellerId(sellerPortfolio.getUserId());
        negotiation.setSellerPortfolioId(sellerPortfolio.getId());
        negotiation.setListingId(sellerPortfolio.getListingId());
        negotiation.setQuantity(request.getQuantity());
        negotiation.setPricePerUnit(scalePrice(request.getPricePerUnit()));
        negotiation.setContractExpiryDate(request.getContractExpiryDate());
        negotiation.setStatus(OtcNegotiationStatus.OPEN);
        negotiation.setCreatedByUserId(user.userId());
        negotiation.setLastUpdatedByUserId(user.userId());

        OtcNegotiation saved = otcNegotiationRepository.save(negotiation);
        appendHistory(saved, user.userId(), OtcNegotiationEventType.CREATED, null);
        return toResponse(saved, user.userId());
    }

    @Override
    @Transactional
    public OtcNegotiationResponse counterOffer(AuthenticatedUser user, Long negotiationId, UpdateOtcNegotiationRequest request) {
        validateOfferTerms(request.getQuantity(), request.getPricePerUnit(), request.getContractExpiryDate());

        OtcNegotiation negotiation = loadParticipantNegotiationForUpdate(user.userId(), negotiationId);
        ensureNegotiationMutable(negotiation);
        ensureOtherSideResponds(user.userId(), negotiation);
        validateSellerPortfolioForNegotiation(loadSellerPortfolio(negotiation), request.getQuantity());

        Snapshot previous = Snapshot.from(negotiation);
        negotiation.setQuantity(request.getQuantity());
        negotiation.setPricePerUnit(scalePrice(request.getPricePerUnit()));
        negotiation.setContractExpiryDate(request.getContractExpiryDate());
        negotiation.setStatus(OtcNegotiationStatus.COUNTERED);
        negotiation.setLastUpdatedByUserId(user.userId());

        OtcNegotiation saved = otcNegotiationRepository.save(negotiation);
        appendHistory(saved, user.userId(), OtcNegotiationEventType.COUNTEROFFERED, previous);
        sendNotificationToOtherParty(saved, user.userId(), "counteroffer.created", notificationProducer::sendOtcCounterofferCreated);
        return toResponse(saved, user.userId());
    }

    @Override
    @Transactional
    public OtcNegotiationResponse acceptNegotiation(AuthenticatedUser user, Long negotiationId) {
        OtcNegotiation negotiation = loadParticipantNegotiationForUpdate(user.userId(), negotiationId);
        ensureNegotiationMutable(negotiation);
        ensureOtherSideResponds(user.userId(), negotiation);
        if (!negotiation.getContractExpiryDate().isAfter(today())) {
            throw new BusinessConflictException("Accepted OTC contract must expire in the future");
        }

        Portfolio sellerPortfolio = loadSellerPortfolioForUpdate(negotiation);
        validateSellerPortfolioForAcceptance(sellerPortfolio, negotiation.getQuantity());

        Snapshot previous = Snapshot.from(negotiation);
        negotiation.setStatus(OtcNegotiationStatus.ACCEPTED);
        negotiation.setLastUpdatedByUserId(user.userId());
        sellerPortfolio.setReservedQuantity(sellerPortfolio.getReservedQuantity() + negotiation.getQuantity());
        sellerPortfolio.setPublicQuantity(Math.max(0, sellerPortfolio.getPublicQuantity() - negotiation.getQuantity()));
        sellerPortfolio.setIsPublic(sellerPortfolio.getPublicQuantity() > 0);

        portfolioRepository.save(sellerPortfolio);
        OtcNegotiation saved = otcNegotiationRepository.save(negotiation);
        appendHistory(saved, user.userId(), OtcNegotiationEventType.ACCEPTED, previous);
        sendNotificationToOtherParty(saved, user.userId(), "offer.accepted", notificationProducer::sendOtcOfferAccepted);
        return toResponse(saved, user.userId());
    }

    @Override
    @Transactional
    public OtcNegotiationResponse declineNegotiation(AuthenticatedUser user, Long negotiationId) {
        OtcNegotiation negotiation = loadParticipantNegotiationForUpdate(user.userId(), negotiationId);
        ensureNegotiationMutable(negotiation);
        ensureOtherSideResponds(user.userId(), negotiation);

        Snapshot previous = Snapshot.from(negotiation);
        negotiation.setStatus(OtcNegotiationStatus.DECLINED);
        negotiation.setLastUpdatedByUserId(user.userId());

        OtcNegotiation saved = otcNegotiationRepository.save(negotiation);
        appendHistory(saved, user.userId(), OtcNegotiationEventType.DECLINED, previous);
        sendNotificationToOtherParty(saved, user.userId(), "offer.declined", notificationProducer::sendOtcOfferDeclined);
        return toResponse(saved, user.userId());
    }

    @Override
    @Transactional
    public OtcNegotiationResponse cancelNegotiation(AuthenticatedUser user, Long negotiationId) {
        OtcNegotiation negotiation = loadParticipantNegotiationForUpdate(user.userId(), negotiationId);
        if (negotiation.getStatus() == OtcNegotiationStatus.ACCEPTED || isTerminal(negotiation.getStatus())) {
            throw new BusinessConflictException("Accepted or completed OTC negotiations cannot be cancelled");
        }

        Snapshot previous = Snapshot.from(negotiation);
        negotiation.setStatus(OtcNegotiationStatus.CANCELLED);
        negotiation.setLastUpdatedByUserId(user.userId());

        OtcNegotiation saved = otcNegotiationRepository.save(negotiation);
        appendHistory(saved, user.userId(), OtcNegotiationEventType.CANCELLED, previous);
        sendNotificationToOtherParty(saved, user.userId(), "offer.cancelled", notificationProducer::sendOtcOfferCancelled);
        return toResponse(saved, user.userId());
    }

    @Override
    @Transactional(readOnly = true)
    public List<OtcNegotiationResponse> listNegotiations(AuthenticatedUser user, OtcNegotiationStatus status,
                                                         LocalDate dateFrom, LocalDate dateTo, Long counterpartyId) {
        return otcNegotiationRepository.findByBuyerIdOrSellerId(user.userId(), user.userId()).stream()
                .filter(negotiation -> status == null || negotiation.getStatus() == status)
                .filter(negotiation -> matchesDateRange(negotiation, dateFrom, dateTo))
                .filter(negotiation -> counterpartyId == null || resolveCounterpartyId(negotiation, user.userId()).equals(counterpartyId))
                .map(negotiation -> toResponse(negotiation, user.userId()))
                .toList();
    }

    @Override
    @Transactional(readOnly = true)
    public List<OtcNegotiationHistoryResponse> getNegotiationHistory(AuthenticatedUser user, Long negotiationId) {
        OtcNegotiation negotiation = loadParticipantNegotiation(user.userId(), negotiationId);
        return otcNegotiationHistoryRepository.findByNegotiationIdOrderByChangedAtAscIdAsc(negotiation.getId()).stream()
                .map(this::toHistoryResponse)
                .toList();
    }

    @Override
    @Transactional
    public void notifyContractsExpiringSoon() {
        LocalDate targetDate = today().plusDays(expirationNotificationDays);
        otcNegotiationRepository.findByStatusAndContractExpiryDate(OtcNegotiationStatus.ACCEPTED, targetDate).stream()
                .filter(negotiation -> negotiation.getExpirationNotifiedAt() == null
                        || !targetDate.equals(negotiation.getExpirationNotifiedAt()))
                .forEach(negotiation -> {
                    sendExpiringNotification(negotiation, negotiation.getBuyerId());
                    sendExpiringNotification(negotiation, negotiation.getSellerId());
                    negotiation.setExpirationNotifiedAt(targetDate);
                    otcNegotiationRepository.save(negotiation);
                });
    }

    @Override
    @Transactional
    public void expireOverdueContracts() {
        LocalDate today = today();
        otcNegotiationRepository.findByStatusAndContractExpiryDateBefore(OtcNegotiationStatus.ACCEPTED, today)
                .forEach(negotiation -> {
                    Portfolio sellerPortfolio = loadSellerPortfolioForUpdate(negotiation);
                    releaseReservedQuantity(sellerPortfolio, negotiation.getQuantity());
                    portfolioRepository.save(sellerPortfolio);

                    Snapshot previous = Snapshot.from(negotiation);
                    negotiation.setStatus(OtcNegotiationStatus.EXPIRED);
                    negotiation.setLastUpdatedByUserId(0L);
                    otcNegotiationRepository.save(negotiation);
                    appendHistory(negotiation, 0L, OtcNegotiationEventType.EXPIRED, previous);
                });
    }

    private void sendExpiringNotification(OtcNegotiation negotiation, Long recipientUserId) {
        NotificationTarget recipient = resolveNotificationTarget(recipientUserId);
        if (recipient == null) {
            log.debug("Skipping OTC expiring notification for unresolved user {}", recipientUserId);
            return;
        }
        Map<String, Object> payload = notificationPayload(recipient, negotiation, "contract.expiring");
        notificationProducer.sendOtcContractExpiring(payload);
    }

    private void sendNotificationToOtherParty(OtcNegotiation negotiation, Long actorUserId, String eventLabel,
                                              NotificationSender sender) {
        Long recipientId = resolveCounterpartyId(negotiation, actorUserId);
        NotificationTarget recipient = resolveNotificationTarget(recipientId);
        if (recipient == null) {
            log.debug("Skipping OTC {} notification because recipient {} could not be resolved", eventLabel, recipientId);
            return;
        }
        sender.send(notificationPayload(recipient, negotiation, eventLabel));
    }

    private Map<String, Object> notificationPayload(NotificationTarget recipient, OtcNegotiation negotiation, String eventLabel) {
        Map<String, Object> payload = new HashMap<>();
        Map<String, String> templateVariables = new HashMap<>();
        templateVariables.put("negotiationId", String.valueOf(negotiation.getId()));
        templateVariables.put("listingId", String.valueOf(negotiation.getListingId()));
        templateVariables.put("quantity", String.valueOf(negotiation.getQuantity()));
        templateVariables.put("pricePerUnit", negotiation.getPricePerUnit().toPlainString());
        templateVariables.put("contractExpiryDate", String.valueOf(negotiation.getContractExpiryDate()));
        templateVariables.put("status", negotiation.getStatus().name());
        templateVariables.put("eventLabel", eventLabel);
        templateVariables.put("daysUntilExpiry", String.valueOf(expirationNotificationDays));
        payload.put("username", recipient.name());
        payload.put("userEmail", recipient.email());
        payload.put("templateVariables", templateVariables);
        return payload;
    }

    private void appendHistory(OtcNegotiation negotiation, Long actorUserId, OtcNegotiationEventType eventType, Snapshot previous) {
        OtcNegotiationHistory history = new OtcNegotiationHistory();
        history.setNegotiationId(negotiation.getId());
        history.setActorUserId(actorUserId);
        history.setEventType(eventType);
        if (previous != null) {
            history.setPreviousQuantity(previous.quantity());
            history.setPreviousPricePerUnit(previous.pricePerUnit());
            history.setPreviousContractExpiryDate(previous.contractExpiryDate());
            history.setPreviousStatus(previous.status());
        }
        history.setNewQuantity(negotiation.getQuantity());
        history.setNewPricePerUnit(negotiation.getPricePerUnit());
        history.setNewContractExpiryDate(negotiation.getContractExpiryDate());
        history.setResultingStatus(negotiation.getStatus());
        otcNegotiationHistoryRepository.save(history);
    }

    private OtcNegotiationResponse toResponse(OtcNegotiation negotiation, Long viewerId) {
        OtcNegotiationResponse response = new OtcNegotiationResponse();
        response.setId(negotiation.getId());
        response.setBuyerId(negotiation.getBuyerId());
        response.setSellerId(negotiation.getSellerId());
        response.setSellerPortfolioId(negotiation.getSellerPortfolioId());
        response.setListingId(negotiation.getListingId());
        response.setQuantity(negotiation.getQuantity());
        response.setPricePerUnit(negotiation.getPricePerUnit());
        response.setContractExpiryDate(negotiation.getContractExpiryDate());
        response.setStatus(negotiation.getStatus());
        response.setCreatedByUserId(negotiation.getCreatedByUserId());
        response.setLastUpdatedByUserId(negotiation.getLastUpdatedByUserId());
        response.setCounterpartyId(resolveCounterpartyId(negotiation, viewerId));
        response.setCreatedAt(negotiation.getCreatedAt());
        response.setUpdatedAt(negotiation.getUpdatedAt());
        return response;
    }

    private OtcNegotiationHistoryResponse toHistoryResponse(OtcNegotiationHistory history) {
        OtcNegotiationHistoryResponse response = new OtcNegotiationHistoryResponse();
        response.setId(history.getId());
        response.setNegotiationId(history.getNegotiationId());
        response.setActorUserId(history.getActorUserId());
        response.setEventType(history.getEventType());
        response.setPreviousQuantity(history.getPreviousQuantity());
        response.setNewQuantity(history.getNewQuantity());
        response.setPreviousPricePerUnit(history.getPreviousPricePerUnit());
        response.setNewPricePerUnit(history.getNewPricePerUnit());
        response.setPreviousContractExpiryDate(history.getPreviousContractExpiryDate());
        response.setNewContractExpiryDate(history.getNewContractExpiryDate());
        response.setPreviousStatus(history.getPreviousStatus());
        response.setResultingStatus(history.getResultingStatus());
        response.setChangedAt(history.getChangedAt());
        return response;
    }

    private Long resolveCounterpartyId(OtcNegotiation negotiation, Long viewerId) {
        return viewerId != null && viewerId.equals(negotiation.getBuyerId())
                ? negotiation.getSellerId()
                : negotiation.getBuyerId();
    }

    private boolean matchesDateRange(OtcNegotiation negotiation, LocalDate dateFrom, LocalDate dateTo) {
        LocalDate activityDate = negotiation.getUpdatedAt().toLocalDate();
        if (dateFrom != null && activityDate.isBefore(dateFrom)) {
            return false;
        }
        return dateTo == null || !activityDate.isAfter(dateTo);
    }

    private void validateOfferTerms(Integer quantity, BigDecimal pricePerUnit, LocalDate contractExpiryDate) {
        if (quantity == null || quantity < 1) {
            throw new BadRequestException("Quantity must be positive");
        }
        if (pricePerUnit == null || pricePerUnit.signum() <= 0) {
            throw new BadRequestException("Price per unit must be positive");
        }
        if (contractExpiryDate == null || !contractExpiryDate.isAfter(today())) {
            throw new BadRequestException("Contract expiry date must be in the future");
        }
    }

    private LocalDate today() {
        return LocalDate.now(clock);
    }

    private void validateSellerPortfolioForNegotiation(Portfolio sellerPortfolio, Integer quantity) {
        if (sellerPortfolio.getListingType() != ListingType.STOCK) {
            throw new BadRequestException("Only public stock portfolios can be negotiated via OTC");
        }
        if (!Boolean.TRUE.equals(sellerPortfolio.getIsPublic()) || sellerPortfolio.getPublicQuantity() == null || sellerPortfolio.getPublicQuantity() < quantity) {
            throw new BusinessConflictException("Requested OTC quantity exceeds seller public quantity");
        }
        int freeQuantity = sellerPortfolio.getQuantity() - sellerPortfolio.getReservedQuantity();
        if (freeQuantity < quantity) {
            throw new BusinessConflictException("Requested OTC quantity exceeds seller available quantity");
        }
    }

    private void validateSellerPortfolioForAcceptance(Portfolio sellerPortfolio, Integer quantity) {
        validateSellerPortfolioForNegotiation(sellerPortfolio, quantity);
    }

    private void ensureNegotiationMutable(OtcNegotiation negotiation) {
        if (isTerminal(negotiation.getStatus()) || negotiation.getStatus() == OtcNegotiationStatus.ACCEPTED) {
            throw new BusinessConflictException("OTC negotiation is already completed");
        }
    }

    private boolean isTerminal(OtcNegotiationStatus status) {
        return status == OtcNegotiationStatus.DECLINED
                || status == OtcNegotiationStatus.CANCELLED
                || status == OtcNegotiationStatus.EXPIRED;
    }

    private void ensureOtherSideResponds(Long actorUserId, OtcNegotiation negotiation) {
        if (actorUserId.equals(negotiation.getLastUpdatedByUserId())) {
            throw new BusinessConflictException("The other OTC participant must respond to the latest offer");
        }
    }

    private Portfolio loadSellerPortfolio(OtcNegotiation negotiation) {
        return portfolioRepository.findById(negotiation.getSellerPortfolioId())
                .orElseThrow(() -> new ResourceNotFoundException("Seller portfolio not found"));
    }

    private Portfolio loadSellerPortfolioForUpdate(OtcNegotiation negotiation) {
        return portfolioRepository.findByIdForUpdate(negotiation.getSellerPortfolioId())
                .orElseThrow(() -> new ResourceNotFoundException("Seller portfolio not found"));
    }

    private OtcNegotiation loadParticipantNegotiation(Long userId, Long negotiationId) {
        OtcNegotiation negotiation = otcNegotiationRepository.findById(negotiationId)
                .orElseThrow(() -> new ResourceNotFoundException("OTC negotiation not found"));
        ensureParticipant(userId, negotiation);
        return negotiation;
    }

    private OtcNegotiation loadParticipantNegotiationForUpdate(Long userId, Long negotiationId) {
        OtcNegotiation negotiation = otcNegotiationRepository.findByIdForUpdate(negotiationId)
                .orElseThrow(() -> new ResourceNotFoundException("OTC negotiation not found"));
        ensureParticipant(userId, negotiation);
        return negotiation;
    }

    private void ensureParticipant(Long userId, OtcNegotiation negotiation) {
        if (!userId.equals(negotiation.getBuyerId()) && !userId.equals(negotiation.getSellerId())) {
            throw new ForbiddenOperationException("Authenticated user is not a participant in this OTC negotiation");
        }
    }

    private void releaseReservedQuantity(Portfolio sellerPortfolio, int quantity) {
        int releasedQuantity = Math.max(0, sellerPortfolio.getReservedQuantity() - quantity);
        sellerPortfolio.setReservedQuantity(releasedQuantity);
    }

    private NotificationTarget resolveNotificationTarget(Long userId) {
        try {
            CustomerDto customer = clientClient.getCustomer(userId);
            if (customer != null && customer.getEmail() != null && !customer.getEmail().isBlank()) {
                return new NotificationTarget(buildFullName(customer.getFirstName(), customer.getLastName()), customer.getEmail());
            }
        } catch (Exception ignored) {
            log.debug("User {} not resolved via client-service for OTC notifications", userId);
        }
        try {
            EmployeeDto employee = employeeClient.getEmployee(userId);
            if (employee != null && employee.getEmail() != null && !employee.getEmail().isBlank()) {
                return new NotificationTarget(buildFullName(employee.getIme(), employee.getPrezime()), employee.getEmail());
            }
        } catch (Exception ignored) {
            log.debug("User {} not resolved via employee-service for OTC notifications", userId);
        }
        return null;
    }

    private String buildFullName(String firstName, String lastName) {
        String safeFirstName = firstName == null ? "" : firstName.trim();
        String safeLastName = lastName == null ? "" : lastName.trim();
        String fullName = (safeFirstName + " " + safeLastName).trim();
        return fullName.isBlank() ? "Korisnik" : fullName;
    }

    private BigDecimal scalePrice(BigDecimal price) {
        return price.setScale(4, RoundingMode.HALF_UP);
    }

    private record NotificationTarget(String name, String email) {
    }

    private record Snapshot(Integer quantity, BigDecimal pricePerUnit, LocalDate contractExpiryDate, OtcNegotiationStatus status) {
        private static Snapshot from(OtcNegotiation negotiation) {
            return new Snapshot(
                    negotiation.getQuantity(),
                    negotiation.getPricePerUnit(),
                    negotiation.getContractExpiryDate(),
                    negotiation.getStatus()
            );
        }
    }

    @FunctionalInterface
    private interface NotificationSender {
        void send(Object payload);
    }
}
