package com.banka1.banking_service.account_service.service;

/**
 * Servis koji obradjuje mesecno odrzavanje racuna — oduzima naknadu za odrzavanje
 * od svih aktivnih tekucih racuna koji imaju definisanu naknadu.
 */
public interface MaintenanceFeeService {

    /**
     * Procesuira naknadu za odrzavanje za sve aktivne tekuce racune.
     * Za svaki racun koji ima definisanu naknadu i dovoljno sredstava,
     * oduzima naknadu i kreditira banka-racun u RSD.
     * Racuni sa nedovoljnim stanjem se preskacaju.
     */
    void process();
}
