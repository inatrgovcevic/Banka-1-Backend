package com.banka1.banking_service.card_service.dto.card_creation.request;

import com.banka1.banking_service.card_service.domain.enums.AuthorizedPersonGender;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Past;
import lombok.Getter;
import lombok.Setter;

import java.time.LocalDate;

/**
 * Request payload for creating a new authorized-person record on a business-card request.
 */
@Getter
@Setter
public class AuthorizedPersonRequestDto {

    @NotBlank(message = "First name is required.")
    private String firstName;

    @NotBlank(message = "Last name is required.")
    private String lastName;

    @NotNull(message = "Date of birth is required.")
    @Past(message = "Date of birth must be in the past.")
    private LocalDate dateOfBirth;

    @NotNull(message = "Gender is required.")
    private AuthorizedPersonGender gender;

    @NotBlank(message = "Email is required.")
    @Email(message = "Email must be valid.")
    private String email;

    @NotBlank(message = "Phone is required.")
    private String phone;

    @NotBlank(message = "Address is required.")
    private String address;
}
