package com.banka1.interbank.model;

import com.banka1.interbank.model.enums.TxStatus;
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
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

/**
 * PR_32 Phase 3: lokalno 2PC stanje interbank transakcije.
 *
 * <p>JSONB polja se cuvaju kao raw {@code String} JSON umesto kao
 * {@code JsonNode}, da bi service layer eksplicitno kontrolisao
 * serijalizaciju i da bi testovi (H2) ostali funkcionalni. Hibernate 6.x
 * native {@link JdbcTypeCode}{@code (SqlTypes.JSON)} obavlja mapiranje na
 * {@code jsonb} u PostgreSQL-u i na {@code clob}/{@code varchar} u H2 bez
 * potrebe za {@code hypersistence-utils} dependency-jem.
 *
 * <p>Spec §4.2: status zivotni ciklus PREPARED → COMMITTED / ROLLED_BACK /
 * FAILED. Unique constraint na {@code (transaction_id_routing,
 * transaction_id_local)} zato sto je to "kanonski" 2PC kljuc po profesorovoj
 * specifikaciji.
 */
@Entity
@Table(
    name = "interbank_transactions",
    uniqueConstraints = @UniqueConstraint(
        name = "uq_interbank_transactions",
        columnNames = {"transaction_id_routing", "transaction_id_local"}
    )
)
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class InterbankTransactionEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "transaction_id_routing", nullable = false)
    private int transactionIdRouting;

    @Column(name = "transaction_id_local", nullable = false, length = 64)
    private String transactionIdLocal;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private TxStatus status;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "postings_json", nullable = false)
    private String postingsJson;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "reservation_refs")
    private String reservationRefs;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "message_meta")
    private String messageMeta;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @Column(name = "finalized_at")
    private Instant finalizedAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
