package com.banka1.banking_service.card_service.rest_client;

/**
 * Internal DTO returned by client-service for notification recipient lookup.
 *
 * @param id internal client identifier
 * @param name client first name
 * @param lastName client last name
 * @param email client email address used for notifications
 */
public record ClientNotificationRecipientDto(
        Long id,
        String name,
        String lastName,
        String email
) {
    /**
     * Returns a display-friendly full name for notification templates.
     *
     * @return full name when both parts exist, otherwise the non-blank part
     */
    public String displayName() {
        String safeName = name == null ? "" : name.trim();
        String safeLastName = lastName == null ? "" : lastName.trim();
        String fullName = (safeName + " " + safeLastName).trim();
        return fullName.isBlank() ? safeName : fullName;
    }
}
