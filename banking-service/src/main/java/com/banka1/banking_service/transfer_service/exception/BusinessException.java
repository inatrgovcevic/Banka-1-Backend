package com.banka1.banking_service.transfer_service.exception;

import com.banka1.banking_service.transfer_service.exception.ErrorCode;
import lombok.Getter;

/**
 * Specifični izuzetak koji se baca kada dođe do narušavanja pravila poslovanja Transfer servisa.
 * Sadrži {@link ErrorCode} koji nosi informacije o HTTP statusu i mašinski čitljivom kodu greške.
 */
@Getter
public class BusinessException extends RuntimeException {

  private final ErrorCode errorCode;

  /**
   * @param errorCode Enum konstanta koja definiše tip greške
   * @param detailedMessage Detaljna poruka o grešci namenjena dijagnostici
   */
  public BusinessException(ErrorCode errorCode, String detailedMessage) {
    super(detailedMessage);
    this.errorCode = errorCode;
  }
}