package com.banka1.banking_service.card_service.repository;

import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.domain.enums.CardStatus;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;
import java.util.Optional;

/**
 * Repository for persisted cards.
 */
public interface CardRepository extends JpaRepository<Card, Long> {

    /**
     * Checks whether a card number is already in use.
     *
     * @param cardNumber card number to check
     * @return {@code true} when a card with the number already exists
     */
    boolean existsByCardNumber(String cardNumber);

    /**
     * Finds a card by its card number.
     *
     * @param cardNumber card number to look up
     * @return the matching card, or empty if not found
     */
    Optional<Card> findByCardNumber(String cardNumber);

    /**
     * Returns all cards linked to a given bank account number.
     *
     * @param accountNumber account number to filter by
     * @return list of cards for the account (may be empty)
     */
    List<Card> findByAccountNumber(String accountNumber);

    /**
     * Returns all cards owned by a given client.
     *
     * @param clientId client ID to filter by
     * @return list of cards owned by the client (may be empty)
     */
    List<Card> findByClientId(Long clientId);

    /**
     * Counts non-terminal owner cards for one account.
     *
     * @param accountNumber linked account number
     * @param clientId owner client ID
     * @param status terminal status that should be excluded from the count
     * @return number of matching cards
     */
    long countByAccountNumberAndClientIdAndAuthorizedPersonIdIsNullAndStatusNot(
            String accountNumber,
            Long clientId,
            CardStatus status
    );

    /**
     * Counts non-terminal authorized-person cards for one account/person pair.
     *
     * @param accountNumber linked account number
     * @param authorizedPersonId authorized-person ID
     * @param status terminal status that should be excluded from the count
     * @return number of matching cards
     */
    long countByAccountNumberAndAuthorizedPersonIdAndStatusNot(
            String accountNumber,
            Long authorizedPersonId,
            CardStatus status
    );
}
