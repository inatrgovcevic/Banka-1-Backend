package com.banka1.banking_service.card_service;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;

class LiquibaseChangelogFormatTest {

    private static final Path CHANGELOG_PATH =
            Path.of("src", "main", "resources", "changelog", "card-001-beginning.sql");

    @Test
    void formattedSqlChangesetsMustIncludeAuthorAndId() throws IOException {
        List<String> lines = Files.readAllLines(CHANGELOG_PATH);

        for (String line : lines) {
            String trimmed = line.trim();
            if (trimmed.startsWith("-- changeset ")) {
                String changeset = trimmed.substring("-- changeset ".length());
                assertTrue(
                        changeset.matches("[A-Za-z0-9_-]+:[A-Za-z0-9_-]+"),
                        () -> "Invalid Liquibase changeset header: " + trimmed);
            }
        }
    }
}
