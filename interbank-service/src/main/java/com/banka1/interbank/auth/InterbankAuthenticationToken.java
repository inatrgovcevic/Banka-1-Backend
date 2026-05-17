package com.banka1.interbank.auth;

import com.banka1.interbank.config.InterbankProperties;
import java.util.Collection;
import org.springframework.security.authentication.AbstractAuthenticationToken;
import org.springframework.security.core.GrantedAuthority;

/**
 * PR_32 Phase 4: Authentication token koji predstavlja autorizovanu partner
 * banku posle X-Api-Key header match-a u {@link InterbankAuthFilter}.
 *
 * <p>Principal je {@link InterbankProperties.Partner} objekat — kontroleri
 * mogu da pristupe partneru kroz
 * {@code SecurityContextHolder.getContext().getAuthentication().getPrincipal()}.
 *
 * <p>Credentials su uvek {@code null} — X-Api-Key se odmah validuje u filteru i
 * ne treba da curi dalje u kontekst.
 */
public class InterbankAuthenticationToken extends AbstractAuthenticationToken {

    private final transient InterbankProperties.Partner partner;

    public InterbankAuthenticationToken(InterbankProperties.Partner partner,
                                        Collection<? extends GrantedAuthority> authorities) {
        super(authorities);
        this.partner = partner;
        super.setAuthenticated(true);
    }

    @Override
    public Object getPrincipal() {
        return partner;
    }

    @Override
    public Object getCredentials() {
        return null;
    }
}
