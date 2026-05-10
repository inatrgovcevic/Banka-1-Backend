package com.banka1.banking_service.transfer_service.advice;

import com.banka1.banking_service.transfer_service.advice.GlobalExceptionHandler;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.NoSuchElementException;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

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
}