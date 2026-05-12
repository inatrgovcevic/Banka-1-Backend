package com.banka1.employeeService.advice;

import com.banka1.employeeService.exception.BusinessException;
import com.banka1.employeeService.exception.ErrorCode;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.AmqpException;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.MediaType;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDecision;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import java.util.NoSuchElementException;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

class GlobalExceptionHandlerTest {

    private MockMvc mockMvc;

    static class ValidatedRequest {
        @NotBlank(message = "must not be blank")
        private String name;
        @Email(message = "must be a valid email")
        @NotBlank(message = "must not be blank")
        private String email;

        public String getName() {
            return name;
        }

        public void setName(String n) {
            this.name = n;
        }

        public String getEmail() {
            return email;
        }

        public void setEmail(String e) {
            this.email = e;
        }
    }

    @RestController
    static class TestController {
        @GetMapping("/test-dupe")
        public void throwDataIntegrity() {
            throw new DataIntegrityViolationException("dup key");
        }

        @GetMapping("/test-missing")
        public void throwNoSuchElement() {
            throw new NoSuchElementException("not found");
        }

        @GetMapping("/test-illegal")
        public void throwIllegalArgument() {
            throw new IllegalArgumentException("bad arg");
        }

        @GetMapping("/test-amqp")
        public void throwAmqp() {
            throw new AmqpException("broker down");
        }

        @GetMapping("/test-unexpected")
        public void throwUnexpected() {
            throw new RuntimeException("unexpected");
        }

        @GetMapping("/test-access-denied")
        public void throwAccessDenied() {
            throw new AccessDeniedException("denied");
        }

        @GetMapping("/test-authorization-denied")
        public void throwAuthorizationDenied() {
            // Spring Security 6 raises this for @PreAuthorize denials
            throw new AuthorizationDeniedException("denied", new AuthorizationDecision(false));
        }

        @GetMapping("/test-business-not-found")
        public void throwBusinessNotFound() {
            throw new BusinessException(ErrorCode.USER_NOT_FOUND, "ID: 99");
        }

        @GetMapping("/test-business-conflict")
        public void throwBusinessConflict() {
            throw new BusinessException(ErrorCode.EMAIL_ALREADY_EXISTS, "email@test.com");
        }

        @GetMapping("/test-business-invalid-token")
        public void throwBusinessInvalidToken() {
            throw new BusinessException(ErrorCode.INVALID_TOKEN, "bad token value");
        }

        @GetMapping("/test-business-inactive")
        public void throwBusinessInactive() {
            throw new BusinessException(ErrorCode.USER_INACTIVE, "account inactive");
        }

        @PostMapping("/test-validation")
        public String validateBody(@RequestBody @Valid ValidatedRequest req) {
            return req.getName();
        }
    }

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders.standaloneSetup(new TestController())
                .setControllerAdvice(new GlobalExceptionHandler())
                .build();
    }

    @Test
    void dataIntegrityViolationReturns409() throws Exception {
        mockMvc.perform(get("/test-dupe"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CONSTRAINT_VIOLATION"));
    }

    @Test
    void noSuchElementReturns404() throws Exception {
        mockMvc.perform(get("/test-missing"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.errorCode").value("ERR_NOT_FOUND"))
                .andExpect(jsonPath("$.errorDesc").value("not found"));
    }

    @Test
    void illegalArgumentReturns400() throws Exception {
        mockMvc.perform(get("/test-illegal"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.errorDesc").value("bad arg"));
    }

    @Test
    void amqpExceptionReturns500() throws Exception {
        mockMvc.perform(get("/test-amqp"))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.errorCode").value("ERR_INTERNAL_SERVER"));
    }

    @Test
    void unexpectedExceptionReturns500() throws Exception {
        mockMvc.perform(get("/test-unexpected"))
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
        mockMvc.perform(get("/test-authorization-denied"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_FORBIDDEN"));
    }

    @Test
    void businessExceptionNotFoundReturns404WithErrorCode() throws Exception {
        mockMvc.perform(get("/test-business-not-found"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.errorCode").value("ERR_USER_001"))
                .andExpect(jsonPath("$.errorDesc").value("ID: 99"));
    }

    @Test
    void businessExceptionConflictReturns409WithErrorCode() throws Exception {
        mockMvc.perform(get("/test-business-conflict"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_USER_002"));
    }

    @Test
    void businessExceptionInvalidTokenReturns400() throws Exception {
        mockMvc.perform(get("/test-business-invalid-token"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_AUTH_002"))
                .andExpect(jsonPath("$.errorDesc").value("bad token value"));
    }

    @Test
    void businessExceptionUserInactiveReturns403() throws Exception {
        mockMvc.perform(get("/test-business-inactive"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_AUTH_003"));
    }

    @Test
    void methodArgumentNotValidReturns400WithValidationErrors() throws Exception {
        mockMvc.perform(post("/test-validation")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"\",\"email\":\"not-an-email\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.validationErrors.name").exists())
                .andExpect(jsonPath("$.validationErrors.email").exists());
    }

    @Test
    void methodArgumentNotValidWithBothBlankFieldsReturns400() throws Exception {
        mockMvc.perform(post("/test-validation")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"name\":\"\",\"email\":\"\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"));
    }
}

