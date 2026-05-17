package com.banka1.interbank.otc.wrapper.dto;

import com.banka1.interbank.otc.dto.OtcNegotiationDto;
import com.banka1.interbank.protocol.dto.ForeignBankId;

/**
 * PR_33 Phase A: FE response za POST/PUT/GET /api/interbank/otc/negotiations/*.
 *
 * <p>
 * <ul>
 *   <li>{@code localId} — nas internal mirror id (npr. {@code neg-ab12cd34}).</li>
 *   <li>{@code remoteForeignBankId} — partner-ov authoritative id (routing
 *       broj partner-a + njegov negotiation id).</li>
 *   <li>{@code state} — full snapshot (kolone iz local mirror-a; ako nismo
 *       authoritative, ovo moze da kasni za partner-ovim state-om dok ne
 *       stigne sledeci counter-offer/accept callback).</li>
 * </ul>
 */
public record OutboundNegotiationResponse(
        String localId,
        ForeignBankId remoteForeignBankId,
        OtcNegotiationDto state
) {}
