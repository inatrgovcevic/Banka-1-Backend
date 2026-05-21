package com.banka1.order.service;

import com.banka1.order.dto.AuthenticatedUser;
import com.banka1.order.dto.CreateOtcNegotiationRequest;
import com.banka1.order.dto.OtcNegotiationHistoryResponse;
import com.banka1.order.dto.OtcNegotiationResponse;
import com.banka1.order.dto.UpdateOtcNegotiationRequest;
import com.banka1.order.entity.enums.OtcNegotiationStatus;

import java.time.LocalDate;
import java.util.List;

/**
 * OTC negotiation workflow service.
 */
public interface OtcNegotiationService {

    OtcNegotiationResponse createNegotiation(AuthenticatedUser user, CreateOtcNegotiationRequest request);

    OtcNegotiationResponse counterOffer(AuthenticatedUser user, Long negotiationId, UpdateOtcNegotiationRequest request);

    OtcNegotiationResponse acceptNegotiation(AuthenticatedUser user, Long negotiationId);

    OtcNegotiationResponse declineNegotiation(AuthenticatedUser user, Long negotiationId);

    OtcNegotiationResponse cancelNegotiation(AuthenticatedUser user, Long negotiationId);

    List<OtcNegotiationResponse> listNegotiations(AuthenticatedUser user, OtcNegotiationStatus status,
                                                  LocalDate dateFrom, LocalDate dateTo, Long counterpartyId);

    List<OtcNegotiationHistoryResponse> getNegotiationHistory(AuthenticatedUser user, Long negotiationId);

    void notifyContractsExpiringSoon();

    void expireOverdueContracts();
}
