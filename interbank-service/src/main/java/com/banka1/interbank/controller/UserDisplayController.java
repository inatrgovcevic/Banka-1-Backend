package com.banka1.interbank.controller;

import com.banka1.interbank.client.UserInternalClient;
import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.exception.InvalidNegotiationException;
import com.banka1.interbank.exception.NegotiationNotFoundException;
import com.banka1.interbank.otc.dto.UserInformationDto;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 10 Task 10.5: GET /interbank/user/{rn}/{id} per Tim 2 §3.7.
 *
 * <p>Resolve display imena za UI prikaz na strani partner banke. ID prefix
 * ({@code C-} ili {@code E-}) odredjuje da li resolve-ujemo klijenta ili
 * zaposlenog kroz {@link UserInternalClient#resolveUser}.
 *
 * <p>Validacije:
 * <ul>
 *   <li>{@code rn} mora biti nas routing — inace 404 (mi ne znamo o tudjim
 *       korisnicima).</li>
 *   <li>{@code id} format: {@code C-<digits>} ili {@code E-<digits>} —
 *       inace 400.</li>
 *   <li>User-service vraca null/404 → mi vracamo 404.</li>
 * </ul>
 */
@RestController
@RequiredArgsConstructor
@Slf4j
public class UserDisplayController {

    private final UserInternalClient userClient;
    private final InterbankProperties props;

    @GetMapping("/interbank/user/{rn}/{id}")
    public UserInformationDto user(@PathVariable int rn, @PathVariable String id) {
        if (rn != props.getMyRoutingNumber()) {
            throw new NegotiationNotFoundException(
                    "User " + rn + "/" + id + " not in this bank (rn=" + rn + ")");
        }
        if (id == null || id.length() < 3) {
            throw new InvalidNegotiationException(
                    "Invalid user id format: " + id);
        }
        String type;
        String numericPart;
        if (id.startsWith("C-")) {
            type = "CLIENT";
            numericPart = id.substring(2);
        } else if (id.startsWith("E-")) {
            type = "EMPLOYEE";
            numericPart = id.substring(2);
        } else {
            throw new InvalidNegotiationException(
                    "User id must start with C- or E-: " + id);
        }
        Long numericId;
        try {
            numericId = Long.valueOf(numericPart);
        } catch (NumberFormatException e) {
            throw new InvalidNegotiationException(
                    "User id numeric part not a valid Long: " + numericPart);
        }
        UserInternalClient.UserDisplayRes res;
        try {
            res = userClient.resolveUser(type, numericId);
        } catch (Exception e) {
            log.warn("user-service resolve failed za {}/{}", type, numericId, e);
            throw new NegotiationNotFoundException(
                    "User " + id + " not found");
        }
        if (res == null) {
            throw new NegotiationNotFoundException("User " + id + " not found");
        }
        return new UserInformationDto(
                props.getMyBankDisplayName(),
                res.fullName());
    }
}
