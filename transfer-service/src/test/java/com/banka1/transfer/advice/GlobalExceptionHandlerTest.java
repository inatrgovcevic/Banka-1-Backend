package com.banka1.transfer.advice;

import com.banka1.transfer.advice.GlobalExceptionHandler;
import com.banka1.transfer.dto.responses.ErrorResponseDto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDecision;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.NoSuchElementException;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

class GlobalExceptionHandlerTest {

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders.standaloneSetup(new TestController())
                .setControllerAdvice(new GlobalExceptionHandler())
                .build();
    }

    // Pomocni kontroler za simulaciju gresaka
    @RestController
    static class TestController {
        @GetMapping("/data-integrity")
        public void throwDataIntegrity() { throw new DataIntegrityViolationException("Conflict"); }

        @GetMapping("/not-found")
        public void throwNotFound() { throw new NoSuchElementException("Not found"); }

        @GetMapping("/illegal-arg")
        public void throwIllegalArg() { throw new IllegalArgumentException("Bad request"); }

        @GetMapping("/generic-error")
        public void throwGeneric() throws Exception { throw new Exception("Unexpected"); }

        @GetMapping("/access-denied")
        public void throwAccessDenied() { throw new AccessDeniedException("denied"); }

        @GetMapping("/authorization-denied")
        public void throwAuthorizationDenied() {
            throw new AuthorizationDeniedException("denied", new AuthorizationDecision(false));
        }
    }

    @Test
    void handleDataIntegrityViolation_ShouldReturnConflict() throws Exception {
        mockMvc.perform(get("/data-integrity"))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.errorCode").value("ERR_CONSTRAINT_VIOLATION"))
                .andExpect(jsonPath("$.errorTitle").value("Konflikt podataka"));
    }

    @Test
    void handleNoSuchElement_ShouldReturnNotFound() throws Exception {
        mockMvc.perform(get("/not-found"))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.errorCode").value("ERR_NOT_FOUND"))
                .andExpect(jsonPath("$.errorDesc").value("Not found"));
    }

    @Test
    void handleIllegalArgument_ShouldReturnBadRequest() throws Exception {
        mockMvc.perform(get("/illegal-arg"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"));
    }

    @Test
    void handleUnexpectedException_ShouldReturnInternalServerError() throws Exception {
        mockMvc.perform(get("/generic-error"))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.errorCode").value("ERR_INTERNAL_SERVER"))
                .andExpect(jsonPath("$.errorTitle").value("Serverska greška"));
    }

    /**
     * Spring Security 5: @PreAuthorize denial -> AccessDeniedException.
     * Handler must return 403 instead of falling through to the 500 catch-all.
     */
    @Test
    void handleAccessDenied_ShouldReturnForbidden() throws Exception {
        mockMvc.perform(get("/access-denied"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_FORBIDDEN"));
    }

    /**
     * Spring Security 6: @PreAuthorize denial -> AuthorizationDeniedException
     * (subclass of AccessDeniedException). Same handler must catch it.
     */
    @Test
    void handleAuthorizationDenied_ShouldReturnForbidden() throws Exception {
        mockMvc.perform(get("/authorization-denied"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.errorCode").value("ERR_FORBIDDEN"));
    }
}