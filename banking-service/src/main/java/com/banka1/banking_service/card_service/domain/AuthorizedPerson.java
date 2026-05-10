package com.banka1.banking_service.card_service.domain;

import com.banka1.banking_service.card_service.domain.BaseEntity;
import com.banka1.banking_service.card_service.domain.enums.AuthorizedPersonGender;
import jakarta.persistence.*;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.LocalDate;
import java.util.ArrayList;
import java.util.List;

/**
 * Informational business-account person for whom a card may be issued.
 * This entity never represents an authenticated system user.
 */
@Entity
@Table(name = "authorized_persons")
@Getter
@Setter
@NoArgsConstructor
public class AuthorizedPerson extends BaseEntity {

    @Column(name = "first_name", nullable = false, length = 100)
    private String firstName;

    @Column(name = "last_name", nullable = false, length = 100)
    private String lastName;

    @Column(name = "date_of_birth", nullable = false)
    private LocalDate dateOfBirth;

    @Enumerated(EnumType.STRING)
    @Column(name = "gender", nullable = false, length = 20)
    private AuthorizedPersonGender gender;

    @Column(name = "email", nullable = false, length = 255)
    private String email;

    @Column(name = "phone", nullable = false, length = 50)
    private String phone;

    @Column(name = "address", nullable = false, length = 255)
    private String address;

    @ElementCollection(fetch = FetchType.EAGER)
    @CollectionTable(
            name = "authorized_person_card_ids",
            joinColumns = @JoinColumn(name = "authorized_person_id")
    )
    @Column(name = "card_id", nullable = false)
    private List<Long> cardIds = new ArrayList<>();
}
