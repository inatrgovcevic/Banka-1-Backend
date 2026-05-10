package com.banka1.banking_service.credit_service.rabbitMQ;

import com.banka1.banking_service.credit_service.rabbitMQ.EmailType;
import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO sent to the RabbitMQ email service.
 * Contains all necessary data for generating and sending email notifications
 * related to credit/loan operations.
 * Fields with {@code null} values are excluded from JSON serialization.
 */
@NoArgsConstructor
@Getter
@Setter
@JsonInclude(JsonInclude.Include.NON_NULL)
public class EmailDto {

    /**
     * Email address of the notification recipient.
     */
    private String userEmail;

    /**
     * Name or username of the recipient (used in the email text).
     */
    private String username;

    /**
     * Type of email notification that determines the content and template of the email.
     */
    private EmailType emailType;


    /** The credit/loan ID associated with this notification. */
    private Long creditId;

    /** The approved credit amount (used for CREDIT_APPROVED notification). */
    private BigDecimal approvedAmount;

    /** The installment amount due (used for CREDIT_INSTALLMENT_FAILED notification). */
    private BigDecimal installmentAmount;

    /** Hours until payment deadline (used for CREDIT_INSTALLMENT_FAILED notification). */
    private Integer hours;

    /**
     * Constructs an EmailDto for failed credit installment notification.
     *
     * @param userEmail the recipient's email address
     * @param username the recipient's username
     * @param creditId the credit ID
     * @param installmentAmount the amount of the failed installment
     * @param hours hours until payment deadline
     */
    public EmailDto(String userEmail,String username, Long creditId, BigDecimal installmentAmount,Integer hours) {
        this.userEmail = userEmail;
        this.username = username;
        this.creditId = creditId;
        this.installmentAmount = installmentAmount;
        this.emailType=EmailType.CREDIT_INSTALLMENT_FAILED;
        this.hours=hours;
    }

    /**
     * Constructs an EmailDto for denied credit notification.
     *
     * @param userEmail the recipient's email address
     * @param username the recipient's username
     * @param creditId the credit ID
     */
    public EmailDto(String userEmail, String username, Long creditId) {
        this.userEmail = userEmail;
        this.username = username;
        this.creditId = creditId;
        this.emailType = EmailType.CREDIT_DENIED;
    }

    /**
     * Constructs an EmailDto for approved credit notification.
     *
     * @param userEmail the recipient's email address
     * @param username the recipient's username
     * @param approvedAmount the amount of approved credit
     * @param creditId the credit ID
     */
    public EmailDto(String userEmail, String username, BigDecimal approvedAmount,Long creditId) {
        this.userEmail = userEmail;
        this.username = username;
        this.creditId = creditId;
        this.emailType = EmailType.CREDIT_APPROVED;
        this.approvedAmount = approvedAmount;
    }

    /**
     * Creates a payload for an email intended to notify the user about a transaction.
     *
     * @param username the username or display name for the email
     * @param userEmail the email address of the recipient
     * @param emailType the type of notification (TRANSACTION_COMPLETED or TRANSACTION_DENIED)
     */


}
