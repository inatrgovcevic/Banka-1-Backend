package com.banka1.userservice.client.migration;

import com.banka1.clientService.migration.JmbgPlaintextToCiphertextMigrator;
import com.banka1.security.crypto.JmbgEncryptor;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.util.ReflectionTestUtils;

import java.util.List;
import java.util.Map;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class JmbgPlaintextToCiphertextMigratorTest {

    @Mock private JdbcTemplate jdbcTemplate;
    @Mock private JmbgEncryptor encryptor;

    @InjectMocks private JmbgPlaintextToCiphertextMigrator migrator;

    @BeforeEach
    void setUp() {
        ReflectionTestUtils.setField(migrator, "enabled", true);
    }

    @Test
    void migrate_skip_kada_disabled() {
        ReflectionTestUtils.setField(migrator, "enabled", false);
        migrator.migrate();
        verifyNoInteractions(jdbcTemplate, encryptor);
    }

    @Test
    void migrate_skip_kada_jmbg_kolona_ne_postoji() {
        when(jdbcTemplate.queryForObject(contains("information_schema"), eq(Integer.class)))
                .thenReturn(0);
        migrator.migrate();
        verify(jdbcTemplate, never()).queryForList(contains("SELECT id, jmbg"));
    }

    @Test
    void migrate_enkriptuje_sve_plaintext_jmbg_redove() {
        when(jdbcTemplate.queryForObject(contains("information_schema"), eq(Integer.class)))
                .thenReturn(1);
        when(jdbcTemplate.queryForList(contains("SELECT id, jmbg")))
                .thenReturn(List.of(
                        Map.of("id", 1L, "jmbg", "1234567890123"),
                        Map.of("id", 2L, "jmbg", "9876543210987")
                ));
        when(encryptor.encrypt("1234567890123")).thenReturn("ciphertext-1");
        when(encryptor.encrypt("9876543210987")).thenReturn("ciphertext-2");

        migrator.migrate();

        verify(jdbcTemplate).update(eq("UPDATE clients SET jmbg_encrypted = ? WHERE id = ?"),
                eq("ciphertext-1"), eq(1L));
        verify(jdbcTemplate).update(eq("UPDATE clients SET jmbg_encrypted = ? WHERE id = ?"),
                eq("ciphertext-2"), eq(2L));
    }

    @Test
    void migrate_nastavlja_kada_jedan_red_fail() {
        when(jdbcTemplate.queryForObject(contains("information_schema"), eq(Integer.class)))
                .thenReturn(1);
        when(jdbcTemplate.queryForList(contains("SELECT id, jmbg")))
                .thenReturn(List.of(
                        Map.of("id", 1L, "jmbg", "1234567890123"),
                        Map.of("id", 2L, "jmbg", "9876543210987")
                ));
        when(encryptor.encrypt("1234567890123")).thenThrow(new RuntimeException("Bad input"));
        when(encryptor.encrypt("9876543210987")).thenReturn("ciphertext-2");

        migrator.migrate();

        // Drugi red mora i dalje biti procesiran
        verify(jdbcTemplate).update(eq("UPDATE clients SET jmbg_encrypted = ? WHERE id = ?"),
                eq("ciphertext-2"), eq(2L));
    }
}
