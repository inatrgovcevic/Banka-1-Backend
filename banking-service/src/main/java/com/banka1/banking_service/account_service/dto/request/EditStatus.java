package com.banka1.banking_service.account_service.dto.request;

import com.banka1.banking_service.account_service.domain.enums.Status;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class EditStatus {
    @NotNull
    private Status status;
}
