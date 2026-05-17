package com.banka1.account_service.advice;


import com.banka1.account_service.dto.response.ErrorResponseDto;
import com.banka1.account_service.exception.BusinessException;
import com.banka1.account_service.exception.ErrorCode;
import org.springframework.amqp.AmqpException;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.security.core.AuthenticationException;
import org.springframework.stereotype.Component;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException;

import java.util.HashMap;
import java.util.Map;
import java.util.NoSuchElementException;

/**
 * Centralizovani hendler grešaka za sve REST kontrolere account-service-a.
 * <p>
 * Mapira očekivane i neočekivane izuzetke na standardizovane HTTP odgovore sa
 * {@link ErrorResponseDto} telom. Sve greške vraćaju konzistentnu strukturu
 * sa kodom, naslovom i detaljima greške.
 * <p>
 * Podržani izuzeci:
 * <ul>
 *   <li>{@link BusinessException} - poslovne greške sa {@link ErrorCode}</li>
 *   <li>{@link MethodArgumentNotValidException} - greške validacije DTO-a sa mapom polja</li>
 *   <li>{@link DataIntegrityViolationException} - kršenja integrity ograničenja baze (duplikati)</li>
 *   <li>{@link NoSuchElementException} - traženjem resurs nije pronađen (404)</li>
 *   <li>{@link IllegalArgumentException} - neispravni argumenti</li>
 *   <li>{@link AmqpException} - greške RabbitMQ komunikacije</li>
 *   <li>{@link AccessDeniedException} - pristup odbijen (403)</li>
 *   <li>{@link AuthenticationException} - autentifikacija nije uspela (401)</li>
 *   <li>{@link Exception} - svi ostali neočekivani izuzeci (500)</li>
 * </ul>
 * <p>
 * Svi izuzeci se obrađuju sa odgovarajućim HTTP status kodovima kako je navedeno
 * u svakom `@ExceptionHandler` metodu.
 */
@RestControllerAdvice
@Component("accountServiceGlobalExceptionHandler")
public class GlobalExceptionHandler {

    /**
     * Obradjuje greske narusavanja ogranicenja baze podataka (npr. duplikat unique kolone).
     *
     * @param ex izuzetak nastao pri krsenju integrity ogranicenja
     * @return HTTP 409 Conflict odgovor sa kodom {@code ERR_CONSTRAINT_VIOLATION}
     */
    @ExceptionHandler(DataIntegrityViolationException.class)
    public ResponseEntity<ErrorResponseDto> handleDataIntegrityViolation(DataIntegrityViolationException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_CONSTRAINT_VIOLATION",
                "Podatak već postoji",
                "Jedan od podataka je već u upotrebi."
        );
        return new ResponseEntity<>(error, HttpStatus.CONFLICT);
    }

    /**
     * Obradjuje greske kada trazeni resurs ne postoji u kolekciji.
     *
     * @param ex izuzetak nastao pri pristupanju nepostojecem elementu
     * @return HTTP 404 Not Found odgovor sa kodom {@code ERR_NOT_FOUND}
     */
    @ExceptionHandler(NoSuchElementException.class)
    public ResponseEntity<ErrorResponseDto> handleNoSuchElement(NoSuchElementException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_NOT_FOUND",
                "Resurs nije pronađen",
                ex.getMessage()
        );
        return new ResponseEntity<>(error, HttpStatus.NOT_FOUND);
    }

    /**
     * Obradjuje greske neispravnih argumenata koji ne prolaze programsku validaciju.
     *
     * @param ex izuzetak nastao pri detektovanju neispravnog argumenta
     * @return HTTP 400 Bad Request odgovor sa kodom {@code ERR_VALIDATION}
     */
    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<ErrorResponseDto> handleIllegalArgument(IllegalArgumentException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_VALIDATION",
                "Neispravni argumenti",
                ex.getMessage()
        );
        return new ResponseEntity<>(error, HttpStatus.BAD_REQUEST);
    }

    /**
     * Obradjuje greske kada Spring ne moze da konvertuje vrednost iz path-a ili
     * query parametra u ocekivani tip (npr. nepostojeca konstanta enum-a kao sto je
     * {@code CurrencyCode}). Ranije su se ovakve greske vracale kao 500 jer ih je
     * pokupio generic {@link Exception} handler iako je u pitanju neispravan ulaz
     * od strane klijenta.
     *
     * @param ex izuzetak nastao pri konverziji parametra
     * @return HTTP 400 Bad Request odgovor sa kodom {@code ERR_VALIDATION}
     */
    @ExceptionHandler(MethodArgumentTypeMismatchException.class)
    public ResponseEntity<ErrorResponseDto> handleTypeMismatch(MethodArgumentTypeMismatchException ex) {
        String requiredType = ex.getRequiredType() == null ? "" : ex.getRequiredType().getSimpleName();
        String detail = "Neispravna vrednost '" + ex.getValue() + "' za parametar '" + ex.getName() + "'"
                + (requiredType.isEmpty() ? "." : ", ocekivan tip: " + requiredType + ".");
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_VALIDATION",
                "Neispravni argumenti",
                detail
        );
        return new ResponseEntity<>(error, HttpStatus.BAD_REQUEST);
    }

    /**
     * Obradjuje greske komunikacije sa RabbitMQ brokerom.
     *
     * @param ex AMQP izuzetak nastao pri slanju poruke
     * @return HTTP 500 Internal Server Error odgovor sa kodom {@code ERR_INTERNAL_SERVER}
     */
    @ExceptionHandler(AmqpException.class)
    public ResponseEntity<ErrorResponseDto> handleRabbitMqException(AmqpException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_INTERNAL_SERVER",
                "Serverska greška",
                "Mejl nije poslat. Naš tim je obavešten."
        );
        return new ResponseEntity<>(error, HttpStatus.INTERNAL_SERVER_ERROR);
    }

    /**
     * Obradjuje odbijanja pristupa iz Spring Security-ja sa statusom 403 Forbidden.
     * <p>
     * Spring Security 5: {@code @Secured} / {@code @PreAuthorize} -> {@link AccessDeniedException}.
     * Spring Security 6: {@code @PreAuthorize} -> {@link AuthorizationDeniedException}
     * (potklasa {@link AccessDeniedException}). Eksplicitno hvatamo obe da ne bi propala kroz
     * generic {@link Exception} handler i vratila 500.
     *
     * @param ex izuzetak nedozvoljenog pristupa
     * @return HTTP 403 odgovor sa standardizovanim telom greske
     */
    @ExceptionHandler({AccessDeniedException.class, AuthorizationDeniedException.class})
    public ResponseEntity<ErrorResponseDto> handleAccessDenied(AccessDeniedException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_FORBIDDEN",
                "Pristup odbijen",
                "Nemate dozvolu za ovu akciju."
        );
        return new ResponseEntity<>(error, HttpStatus.FORBIDDEN);
    }

    @ExceptionHandler(AuthenticationException.class)
    public ResponseEntity<ErrorResponseDto> handleAuthenticationException(AuthenticationException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_UNAUTHORIZED",
                "Neautorizovan pristup",
                "Niste autentifikovani."
        );
        return new ResponseEntity<>(error, HttpStatus.UNAUTHORIZED);
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ErrorResponseDto> handleUnexpectedException(Exception ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_INTERNAL_SERVER",
                "Serverska greška",
                "Došlo je do neočekivanog problema. Naš tim je obavešten."
        );
        return new ResponseEntity<>(error, HttpStatus.INTERNAL_SERVER_ERROR);
    }

    /**
     * Obradjuje poznate biznis izuzetke i mapira ih na odgovarajuci HTTP status.
     *
     * @param ex biznis izuzetak koji sadrzi domen-specifican kod greske
     * @return odgovor sa detaljima biznis greske i HTTP statusom iz {@link ErrorCode}
     */
    @ExceptionHandler(BusinessException.class)
    public ResponseEntity<ErrorResponseDto> handleBusinessException(BusinessException ex) {
        ErrorCode errorCode = ex.getErrorCode();
        ErrorResponseDto error = new ErrorResponseDto(
                errorCode.getCode(),
                errorCode.getTitle(),
                ex.getMessage()
        );
        return new ResponseEntity<>(error, errorCode.getHttpStatus());
    }

    /**
     * Obradjuje greske validacije DTO zahteva i vraca listu neispravnih polja.
     *
     * @param ex izuzetak nastao pri validaciji ulaznih podataka
     * @return HTTP 400 odgovor sa mapom validacionih gresaka po poljima
     */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponseDto> handleValidation(MethodArgumentNotValidException ex) {
        Map<String, String> validationErrors = new HashMap<>();
        for (FieldError fieldError : ex.getBindingResult().getFieldErrors()) {
            validationErrors.put(fieldError.getField(), fieldError.getDefaultMessage());
        }
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_VALIDATION",
                "Neispravni podaci",
                "Molimo Vas proverite unete podatke.",
                validationErrors
        );
        return new ResponseEntity<>(error, HttpStatus.BAD_REQUEST);
    }
}
