package com.banka1.banking_service.account_service.repository;

import com.banka1.banking_service.account_service.domain.Company;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface CompanyRepository extends JpaRepository<Company,Long> {
}
