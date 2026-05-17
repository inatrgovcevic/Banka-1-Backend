package com.banka1.userservice.interbank.controller;

import com.banka1.clientService.domain.enums.ClientRole;
import com.banka1.clientService.domain.enums.Pol;
import com.banka1.clientService.dto.responses.ClientInfoResponseDto;
import com.banka1.clientService.exception.BusinessException;
import com.banka1.clientService.exception.ErrorCode;
import com.banka1.clientService.service.ClientService;
import com.banka1.employeeService.dto.responses.EmployeeResponseDto;
import com.banka1.employeeService.service.CrudService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;

import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

/**
 * PR_32 Phase 13: pokriva resolve flow za interbank user lookup endpoint.
 *
 * <p>Endpoint je deliberatno re-implementacija logike koja izlozi user-service
 * legacy {@code ClientService}/{@code CrudService} direktno SERVICE tokenu, bez
 * autorizacionih ogranicenja postojecih /clients i /employees rute (na njih dva
 * controller-a SERVICE nije whitelisted).
 *
 * <p>Test koristi plain MockMvc standalone setup (bez Spring context boot-a) jer
 * controller nema custom @ExceptionHandler — Spring podrazumevano vraca 200/404/400
 * koje sam vec eksplicitno vracamo iz controller metoda.
 */
@ExtendWith(MockitoExtension.class)
class InternalUserDirectoryControllerTest {

    @Mock private ClientService clientService;
    @Mock private CrudService employeeService;

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        InternalUserDirectoryController controller =
                new InternalUserDirectoryController(clientService, employeeService);
        mockMvc = MockMvcBuilders.standaloneSetup(controller).build();
    }

    @Test
    void resolveClientHappy() throws Exception {
        ClientInfoResponseDto dto = new ClientInfoResponseDto(
                5L, "Ana", "Anic", "ana@banka.com", "1234567890123",
                "+381601234567", "Ulica 1", Pol.Z, 946684800000L,
                ClientRole.CLIENT_BASIC, true);
        when(clientService.getInfoById(5L)).thenReturn(dto);

        mockMvc.perform(get("/internal/interbank/user/CLIENT/5"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.firstName").value("Ana"))
                .andExpect(jsonPath("$.lastName").value("Anic"))
                .andExpect(jsonPath("$.fullName").value("Ana Anic"));
    }

    @Test
    void resolveEmployeeHappy() throws Exception {
        EmployeeResponseDto dto = new EmployeeResponseDto(
                42L, "Marko", "Markovic", "marko@banka.com", "marko.markovic",
                null, null, null, null, null, null, true, null);
        when(employeeService.getEmployee(42L)).thenReturn(dto);

        mockMvc.perform(get("/internal/interbank/user/EMPLOYEE/42"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.firstName").value("Marko"))
                .andExpect(jsonPath("$.lastName").value("Markovic"))
                .andExpect(jsonPath("$.fullName").value("Marko Markovic"));
    }

    @Test
    void resolveClientNotFound() throws Exception {
        doThrow(new BusinessException(ErrorCode.CLIENT_NOT_FOUND, "ID: 99"))
                .when(clientService).getInfoById(eq(99L));

        mockMvc.perform(get("/internal/interbank/user/CLIENT/99"))
                .andExpect(status().isNotFound());
    }

    @Test
    void resolveInvalidType() throws Exception {
        mockMvc.perform(get("/internal/interbank/user/OTHER/1"))
                .andExpect(status().isBadRequest());
    }
}
