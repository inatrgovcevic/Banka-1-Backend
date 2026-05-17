package app.config;

import com.google.auth.oauth2.GoogleCredentials;
import com.google.firebase.FirebaseApp;
import com.google.firebase.FirebaseOptions;
import jakarta.annotation.PostConstruct;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Configuration;

import java.io.FileInputStream;
import java.io.IOException;

/**
 * Initializes the Firebase Admin SDK on application startup.
 *
 * <p>This configuration reads the path to a Google service account JSON file from
 * the {@code firebase.credentials-path} property (which is backed by the
 * {@code FIREBASE_CREDENTIALS_PATH} environment variable) and bootstraps a
 * {@link FirebaseApp} instance that downstream code uses via
 * {@link com.google.firebase.messaging.FirebaseMessaging}.
 *
 * <p>Initialization is deliberately best-effort: if the credentials path is not
 * configured or the file cannot be read, the service logs a warning and continues
 * booting. In that degraded mode {@link app.service.FcmPushService} detects the
 * missing {@link FirebaseApp} and silently skips push sends while the rest of
 * the notification pipeline (email delivery, RabbitMQ consumption, audit
 * persistence) keeps working. This is intentional because email is the
 * authoritative delivery channel and FCM is only an additive convenience
 * channel used by the mobile app.
 */
@Configuration
@Slf4j
public class FirebaseConfig {

    /**
     * Filesystem path to the Firebase service account JSON.
     * Empty string when the service is running in email-only mode.
     */
    @Value("${firebase.credentials-path:}")
    private String credentialsPath;

    /**
     * Initializes the Firebase Admin SDK if credentials are configured.
     *
     * <p>Executed once after bean construction. Handles three cases:
     * <ul>
     *   <li>property not set → logs a warning and skips initialization so FCM
     *       pushes become no-ops</li>
     *   <li>{@link FirebaseApp} already initialized (e.g. by a prior call in the
     *       same JVM during tests) → leaves the existing instance in place</li>
     *   <li>credentials file missing or unreadable → logs an error and leaves
     *       Firebase uninitialized; the service continues running without FCM</li>
     * </ul>
     */
    @PostConstruct
    public void initFirebase() {
        if (credentialsPath == null || credentialsPath.isBlank()) {
            log.warn("FIREBASE_CREDENTIALS_PATH not set — FCM push notifications disabled");
            return;
        }

        if (!FirebaseApp.getApps().isEmpty()) {
            log.info("FirebaseApp already initialized");
            return;
        }

        try (FileInputStream serviceAccount = new FileInputStream(credentialsPath)) {
            FirebaseOptions options = FirebaseOptions.builder()
                    .setCredentials(GoogleCredentials.fromStream(serviceAccount))
                    .build();
            FirebaseApp.initializeApp(options);
            log.info("Firebase initialized from {}", credentialsPath);
        } catch (IOException | RuntimeException e) {
            // RuntimeException catches IllegalArgumentException("no JSON input found") that
            // GoogleCredentials.fromStream throws for empty/invalid content (e.g. /dev/null mount
            // in docker-compose when secrets aren't provided locally — HOTFIX_01 from CLAUDE.md).
            log.warn("Firebase init skipped — credentials at {} unreadable or empty: {}",
                    credentialsPath, e.getMessage());
        }
    }
}
