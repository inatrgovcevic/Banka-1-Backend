package com.banka1.order.security;

/**
 * PR_29: Type-safe enum koji ogledava {@code com.banka1.employeeService.domain.enums.Role}
 * iz user-service modula.
 *
 * <p>order-service ne deli persistence sloj sa user-service, vec dobija role kao
 * String preko {@link com.banka1.order.dto.EmployeeDto#getRole()} (REST poziv ka
 * user-service). Pre PR_29 logika je upoređivala raw string-ove
 * ({@code "ADMIN".equals(employee.getRole())}) sto je tipo-prone i nadahnjuje tihi
 * fail ako se ime role-a ikad menja u source enum-u.
 *
 * <p>Ovaj enum sluzi kao type-safe konstante za poredjenje, dok ostaje fleksibilan
 * (case-insensitive trim) za eventualne razlike u serialization-u.
 */
public enum Role {

    /** Osnovna uloga – standardne bankarske operacije. */
    BASIC,

    /** Uloga agenta – dozvoljava trgovinu hartijama od vrednosti. */
    AGENT,

    /** Uloga supervizora – ukljucuje OTC operacije i dodelu limita agentima. */
    SUPERVISOR,

    /** Administratorska uloga – puno upravljanje svim zaposlenima. */
    ADMIN;

    /**
     * Vraca {@code true} ako se {@code roleName} (case-insensitive, trimovan)
     * poklapa sa imenom ovog enum-a.
     */
    public boolean matches(String roleName) {
        return roleName != null && this.name().equalsIgnoreCase(roleName.trim());
    }
}
