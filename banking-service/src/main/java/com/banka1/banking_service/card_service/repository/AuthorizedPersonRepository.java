package com.banka1.banking_service.card_service.repository;

import com.banka1.banking_service.card_service.domain.AuthorizedPerson;
import org.springframework.data.jpa.repository.JpaRepository;

import java.time.LocalDate;
import java.util.Optional;

/**
 * Repository for persisted authorized-person records.
 */
public interface AuthorizedPersonRepository extends JpaRepository<AuthorizedPerson, Long> {

    /**
     * Tries to match an existing authorized person by stable identity fields.
     *
     * @param email email address
     * @param firstName first name
     * @param lastName last name
     * @param dateOfBirth date of birth
     * @return matched person when found
     */
    Optional<AuthorizedPerson> findByEmailIgnoreCaseAndFirstNameIgnoreCaseAndLastNameIgnoreCaseAndDateOfBirth(
            String email,
            String firstName,
            String lastName,
            LocalDate dateOfBirth
    );
}
