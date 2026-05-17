package com.banka1.interbank.model;

import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.MessageType;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * PR_32 Phase 3: idempotency cache za INBOUND i OUTBOUND interbank pozive.
 *
 * <p>Unique constraint na {@code (direction, sender_routing_number,
 * locally_generated_key)} obezbedjuje da retry istog poziva (po Tim 2 §2.2)
 * uvek vrati cached response umesto da ga obradi ponovo. Tabela se NE brise —
 * idempotency persistuje zauvek.
 *
 * @see com.banka1.interbank.protocol.dto.IdempotenceKey
 */
@Entity
@Table(
    name = "interbank_messages",
    uniqueConstraints = @UniqueConstraint(
        name = "uq_interbank_messages",
        columnNames = {"direction", "sender_routing_number", "locally_generated_key"}
    )
)
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class InterbankMessageEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 10)
    private Direction direction;

    @Column(name = "sender_routing_number", nullable = false)
    private int senderRoutingNumber;

    @Column(name = "locally_generated_key", nullable = false, length = 64)
    private String locallyGeneratedKey;

    @Enumerated(EnumType.STRING)
    @Column(name = "message_type", nullable = false, length = 32)
    private MessageType messageType;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private MessageStatus status;

    @Column(name = "request_body", nullable = false, columnDefinition = "TEXT")
    private String requestBody;

    @Column(name = "response_body", columnDefinition = "TEXT")
    private String responseBody;

    @Column(name = "http_status")
    private Integer httpStatus;

    @Column(name = "retry_count", nullable = false)
    private int retryCount;

    @Column(name = "transaction_id_routing")
    private Integer transactionIdRouting;

    @Column(name = "transaction_id_local", length = 64)
    private String transactionIdLocal;

    @Column(name = "last_attempt_at")
    private Instant lastAttemptAt;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
