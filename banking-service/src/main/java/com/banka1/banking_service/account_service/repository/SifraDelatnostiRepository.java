package com.banka1.banking_service.account_service.repository;

import com.banka1.banking_service.account_service.domain.SifraDelatnosti;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface SifraDelatnostiRepository extends JpaRepository<SifraDelatnosti,Long> {
    Optional<SifraDelatnosti> findBySifra(String sifra);
}
