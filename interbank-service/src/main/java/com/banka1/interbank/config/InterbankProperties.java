package com.banka1.interbank.config;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * PR_32 Phase 4: konfiguracija interbank protokola.
 *
 * <p>Cita iz {@code application.properties} sledece kljuceve:
 * <pre>
 * interbank.my-routing-number=111
 * interbank.my-bank-display-name=Banka 1
 * interbank.partners[0].routing-number=222
 * interbank.partners[0].display-name=Banka 2
 * interbank.partners[0].base-url=http://...
 * interbank.partners[0].inbound-token=...
 * interbank.partners[0].outbound-token=...
 * </pre>
 *
 * <p>{@link #findByInboundToken(String)} koristi {@link InterbankAuthFilter} da
 * mapira {@code X-Api-Key} header u partner banku.
 * {@link #partnerOrThrow(int)} koriste OUTBOUND klijenti kad spreme HTTP poziv.
 *
 * <p>NE koristimo Java {@code record} jer Spring Boot 4 binding za nested
 * {@code List<>} property-ja sa record-ima zahteva precizan match konstruktora
 * sto je krhko — koristimo mutable POJO sa Lombok-om.
 */
@ConfigurationProperties(prefix = "interbank")
public class InterbankProperties {

    private int myRoutingNumber;
    private String myBankDisplayName;
    private List<Partner> partners = new ArrayList<>();

    public int getMyRoutingNumber() {
        return myRoutingNumber;
    }

    public void setMyRoutingNumber(int myRoutingNumber) {
        this.myRoutingNumber = myRoutingNumber;
    }

    public String getMyBankDisplayName() {
        return myBankDisplayName;
    }

    public void setMyBankDisplayName(String myBankDisplayName) {
        this.myBankDisplayName = myBankDisplayName;
    }

    public List<Partner> getPartners() {
        return partners;
    }

    public void setPartners(List<Partner> partners) {
        this.partners = partners == null ? new ArrayList<>() : partners;
    }

    /**
     * Lookup partnera po inbound tokenu (X-Api-Key koji partner salje nama).
     *
     * @param inboundToken sirovi token iz {@code X-Api-Key} header-a; moze biti
     *                     {@code null} ili prazan
     * @return Optional sa partner-om ako token tacno matchuje neki od
     *         konfigurisanih, prazan Optional inace
     */
    public Optional<Partner> findByInboundToken(String inboundToken) {
        if (inboundToken == null || inboundToken.isEmpty()) {
            return Optional.empty();
        }
        return partners.stream()
                .filter(p -> Objects.equals(p.getInboundToken(), inboundToken))
                .findFirst();
    }

    /**
     * Lookup partnera po njegovom routing broju.
     *
     * @param routingNumber routing number trazenog partnera
     * @return Optional sa partner-om ako postoji, prazan inace
     */
    public Optional<Partner> findByRoutingNumber(int routingNumber) {
        return partners.stream()
                .filter(p -> p.getRoutingNumber() == routingNumber)
                .findFirst();
    }

    /**
     * Strict varijanta {@link #findByRoutingNumber} koja baca exception ako
     * partner nije konfigurisan. Koristi se u OUTBOUND klijentima gde missing
     * partner indikira programmer error / misconfig (nije runtime user error).
     *
     * @param routingNumber routing number trazenog partnera
     * @return Partner objekat
     * @throws IllegalArgumentException ako partner nije konfigurisan
     */
    public Partner partnerOrThrow(int routingNumber) {
        return findByRoutingNumber(routingNumber)
                .orElseThrow(() -> new IllegalArgumentException(
                        "Nepoznat partner routingNumber=" + routingNumber
                                + "; provera interbank.partners[*].routing-number"));
    }

    /**
     * Konfiguracija jednog partnera. Polja moraju biti mutable za Spring Boot
     * @ConfigurationProperties bindovanje (setter-i).
     */
    public static class Partner {

        private int routingNumber;
        private String displayName;
        private String baseUrl;
        private String inboundToken;
        private String outboundToken;

        public int getRoutingNumber() {
            return routingNumber;
        }

        public void setRoutingNumber(int routingNumber) {
            this.routingNumber = routingNumber;
        }

        public String getDisplayName() {
            return displayName;
        }

        public void setDisplayName(String displayName) {
            this.displayName = displayName;
        }

        public String getBaseUrl() {
            return baseUrl;
        }

        public void setBaseUrl(String baseUrl) {
            this.baseUrl = baseUrl;
        }

        public String getInboundToken() {
            return inboundToken;
        }

        public void setInboundToken(String inboundToken) {
            this.inboundToken = inboundToken;
        }

        public String getOutboundToken() {
            return outboundToken;
        }

        public void setOutboundToken(String outboundToken) {
            this.outboundToken = outboundToken;
        }
    }
}
