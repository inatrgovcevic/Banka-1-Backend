package com.banka1.clientService.advice;

import com.banka1.clientService.exception.BusinessException;
import com.banka1.clientService.exception.ErrorCode;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.AmqpException;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDecision;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import org.hibernate.exception.ConstraintViolationException;

import java.sql.SQLException;
import java.util.NoSuchElementException;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class GlobalExceptionHandlerTest {

    private MockMvc mockMvc;

    static class ValidatedRequest {
        @NotBlank(message = "Ime je obavezno")
        private String ime;

        @Email(message = "Nevalidan format email-a")
        @NotBlank(message = "Email je obavezan")
        private String email;

        public String getIme() { return ime; }
        public void setIme(String ime) { this.ime = ime; }
        public String getEmail() { return email; }
        public void setEmail(String email) { this.email = email; }
    }

    @RestController
    static class TestController {

        @GetMapping("/test-dupe")
        public void throwDataIntegrity() {
            throw new DataIntegrityViolationException("dup key");
        }

        @GetMapping("/test-dupe-email")
        public void throwDataIntegrityEmail() {
            throw new DataIntegrityViolationException("ERROR: duplicate key value violates unique constraint \"clients_email_key\"");
        }

        @GetMapping("/test-dupe-email-alt")
        public void throwDataIntegrityEmailAlt() {
            throw new DataIntegrityViolationException("ERROR: duplicate key value violates unique constraint on (email)");
        }

        @GetMapping("/test-dupe-jmbg")
        public void throwDataIntegrityJmbg() {
            throw new DataIntegrityViolationException("ERROR: duplicate key value violates unique constraint \"clients_jmbg_key\"");
        }

        @GetMapping("/test-dupe-jmbg-alt")
        public void throwDataIntegrityJmbgAlt() {
            throw new DataIntegrityViolationException("ERROR: duplicate key value violates unique constraint on (jmbg)");
        }

        @GetMapping("/test-dupe-null-msg")
        public void throwDataIntegrityNullMsg() {
            throw new DataIntegrityViolationException(null);
        }

        @GetMapping("/test-missing")
        public void throwNoSuchElement() {
            throw new NoSuchElementException("Klijent nije pronadjen");
        }

        @GetMapping("/test-unexpected")
        public void throwUnexpected() {
            throw new RuntimeException("unexpected error");
        }

        @GetMapping("/test-business-not-found")
        public void throwBusinessNotFound() {
            throw new BusinessException(ErrorCode.CLIENT_NOT_FOUND, "ID: 99");
        }

        @GetMapping("/test-business-conflict")
        public void throwBusinessConflict() {
            throw new BusinessException(ErrorCode.EMAIL_ALREADY_EXISTS, "email@test.com");
        }

        @GetMapping("/test-illegal-arg")
        public void throwIllegalArgument() {
            throw new IllegalArgumentException("Nevalidan argument");
        }

        @GetMapping("/test-amqp")
        public void throwAmqp() {
            throw new AmqpException("RabbitMQ je nedostupan");
        }

        @GetMapping("/test-access-denied")
        public void throwAccessDenied() {
            throw new AccessDeniedException("Zabranjen pristup");
        }

        @GetMapping("/test-authorization-denied")
        public void throwAuthorizationDenied() {
            // Spring Security 6 wrap of @PreAuthorize denial
            throw new AuthorizationDeniedException("Zabranjen pristup", new AuthorizationDecision(false));
        }

        @GetMapping("/test-constraint-email")
        public void throwConstraintViolationEmail() {
            ConstraintViolationException cve = new ConstraintViolationException(
                    "duplicate key", new SQLException(), "clients_email_key");
            throw new DataIntegrityViolationException("duplicate key", cve);
        }

        @GetMapping("/test-constraint-jmbg")
        public void throwConstraintViolationJmbg() {
            ConstraintViolationException cve = new ConstraintViolationException(
                    "duplicate key", new SQLException(), "clients_jmbg_key");
            throw new DataIntegrityViolationException("duplicate key", cve);
        }

        @GetMapping("/test-constraint-null-name")
        public void throwConstraintViolationNullName() {
            ConstraintViolationException cve = new ConstraintViolationException(
                    "duplicate key", new SQLException(), null);
            throw new DataIntegrityViolationException("duplicate key", cve);
        }

        @PostMapping("/test-validation")
        public String validateBody(@RequestBody @Valid ValidatedRequest req) {
            return req.getIme();
        }
    }

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders.standaloneSetup(new TestController())
                .setControllerAdvice(new GlobalExceptionHandler())
                .build();
    }

    @Test
    void dataIntegrityViolationGenericReturns409() throws Exception {
        mockMvc.perform(get("/test-dupe"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CONSTRAINT_VIOLATION"))
                .andExpect(jsonPath("$.errorTitle").value("Podatak već postoji"));
    }

    @Test
    void dataIntegrityViolationEmailReturns409WithEmailErrorCode() throws Exception {
        mockMvc.perform(get("/test-dupe-email"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_002"))
                .andExpect(jsonPath("$.errorTitle").value("Email adresa je već u upotrebi"));
    }

    @Test
    void dataIntegrityViolationJmbgReturns409WithJmbgErrorCode() throws Exception {
        mockMvc.perform(get("/test-dupe-jmbg"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_003"))
                .andExpect(jsonPath("$.errorTitle").value("JMBG je već u upotrebi"));
    }

    @Test
    void noSuchElementReturns404() throws Exception {
        mockMvc.perform(get("/test-missing"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.errorCode").value("ERR_NOT_FOUND"))
                .andExpect(jsonPath("$.errorDesc").value("Klijent nije pronadjen"));
    }

    @Test
    void unexpectedExceptionReturns500() throws Exception {
        mockMvc.perform(get("/test-unexpected"))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.errorCode").value("ERR_INTERNAL_SERVER"));
    }

    @Test
    void businessExceptionNotFoundReturns404() throws Exception {
        mockMvc.perform(get("/test-business-not-found"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_001"))
                .andExpect(jsonPath("$.errorTitle").value("Klijent nije pronađen"))
                .andExpect(jsonPath("$.errorDesc").value("ID: 99"));
    }

    @Test
    void businessExceptionConflictReturns409() throws Exception {
        mockMvc.perform(get("/test-business-conflict"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_002"))
                .andExpect(jsonPath("$.errorTitle").value("Email adresa je već u upotrebi"));
    }

    @Test
    void dataIntegrityViolationEmailAltFormatReturns409WithEmailErrorCode() throws Exception {
        mockMvc.perform(get("/test-dupe-email-alt"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_002"))
                .andExpect(jsonPath("$.errorTitle").value("Email adresa je već u upotrebi"));
    }

    @Test
    void dataIntegrityViolationJmbgAltFormatReturns409WithJmbgErrorCode() throws Exception {
        mockMvc.perform(get("/test-dupe-jmbg-alt"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_003"))
                .andExpect(jsonPath("$.errorTitle").value("JMBG je već u upotrebi"));
    }

    @Test
    void dataIntegrityViolationNullMessageReturns409Generic() throws Exception {
        mockMvc.perform(get("/test-dupe-null-msg"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CONSTRAINT_VIOLATION"));
    }

    @Test
    void illegalArgumentReturns400() throws Exception {
        mockMvc.perform(get("/test-illegal-arg"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.errorDesc").value("Nevalidan argument"));
    }

    @Test
    void amqpExceptionReturns500() throws Exception {
        mockMvc.perform(get("/test-amqp"))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.errorCode").value("ERR_INTERNAL_SERVER"));
    }

    @Test
    void accessDeniedReturns403() throws Exception {
        mockMvc.perform(get("/test-access-denied"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_FORBIDDEN"));
    }

    @Test
    void authorizationDeniedReturns403() throws Exception {
        // Spring Security 6 raises AuthorizationDeniedException for @PreAuthorize
        // denials; the same handler must catch it and return 403, not fall through to 500.
        mockMvc.perform(get("/test-authorization-denied"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_FORBIDDEN"));
    }

    @Test
    void constraintViolationEmailCauseReturns409WithEmailErrorCode() throws Exception {
        mockMvc.perform(get("/test-constraint-email"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_002"))
                .andExpect(jsonPath("$.errorTitle").value("Email adresa je već u upotrebi"));
    }

    @Test
    void constraintViolationJmbgCauseReturns409WithJmbgErrorCode() throws Exception {
        mockMvc.perform(get("/test-constraint-jmbg"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CLIENT_003"))
                .andExpect(jsonPath("$.errorTitle").value("JMBG je već u upotrebi"));
    }

    @Test
    void constraintViolationNullNameFallsBackToMessageParsing() throws Exception {
        mockMvc.perform(get("/test-constraint-null-name"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CONSTRAINT_VIOLATION"));
    }

    @Test
    void methodArgumentNotValidReturns400WithErrors() throws Exception {
        mockMvc.perform(post("/test-validation")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"ime\":\"\",\"email\":\"nije-email\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.validationErrors.ime").exists())
                .andExpect(jsonPath("$.validationErrors.email").exists());
    }
}
