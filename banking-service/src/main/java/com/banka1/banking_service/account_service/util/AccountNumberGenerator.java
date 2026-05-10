package com.banka1.banking_service.account_service.util;

import com.banka1.banking_service.account_service.repository.AccountRepository;

import java.util.Random;

/**
 * Utility klasa za generisanje i validaciju 19-cifrenih bankovskih brojeva računa.
 * <p>
 * Omogućava kreiranje novih, jedinstvenih brojeva računa i validaciju postojećih.
 * Koristi se pri kreiranju novih računa da bi se osiguralo da je svaki broj
 * validan, jedinstven i formalno ispravan prema bankovnom standardu.
 * <p>
 * <strong>Struktura 19-cifrenog broja računa:</strong>
 * <pre>
 *   Pozicije 1–3    : Kod banke (fiksno: 111)
 *   Pozicije 4–7    : Kod filijale (fiksno: 0001)
 *   Pozicije 8–16   : 9 nasumičnih cifara (jedinstveni deo)
 *   Pozicije 17–18  : Kod tipa računa (2 cifre, npr. 11 za lični tekući, 21 za poslovni)
 *   Pozicija 19     : Kontrolna cifra (modulo 11 algoritam)
 * </pre>
 * <p>
 * Primeri:
 * <ul>
 *   <li>1110001 000000001 11 1 = lični tekući račun sa kontrolnom cifrom 1</li>
 *   <li>1110001 999999999 21 5 = poslovni račun sa kontrolnom cifrom 5</li>
 * </ul>
 * <p>
 * Ova klasa je immutable i ne sadrži stanje; svi metodi su statički.
 */
public final class AccountNumberGenerator {

    private AccountNumberGenerator() {}

    /**
     * Izracunava kontrolnu cifru za prefix broja racuna koristeci modulo 11 algoritam.
     * <p>
     * Ako rezultat bude 10, broj je nevalidan i treba ponovo generisati.
     *
     * @param prefix prvih 18 cifara broja racuna
     * @return kontrolna cifra 0-10
     */
    public static int calculateCheckDigit(String prefix) {
        int sum = 0;
        for (char c : prefix.toCharArray()) {
            sum += c - '0';
        }
        return (11 - sum % 11) % 11;
    }

    /**
     * Validira 19-cifreni broj racuna proveravanjem formata i kontrolne cifre.
     * <p>
     * Broj mora imati:
     * <ul>
     *   <li>Tacno 19 cifara</li>
     *   <li>Fiksni prefix banke i filijale: 1110001</li>
     *   <li>Podrzan type kod (11-17 ili 21-22)</li>
     *   <li>Ispravnu kontrolnu cifru</li>
     * </ul>
     *
     * @param number broj racuna za validaciju
     * @return {@code true} ako je broj validan, {@code false} inače
     */
    public static boolean validateAccountNumber(String number) {
        if (number == null || number.length() != 19) return false;
        for (char c : number.toCharArray()) {
            if (!Character.isDigit(c)) return false;
        }
        if (!number.startsWith("1110001")) return false;

        String typeVal = number.substring(16, 18);
        if (!typeVal.matches("1[1-7]|2[1-2]")) return false;

        String prefix = number.substring(0, 18);
        int expected = calculateCheckDigit(prefix);
        if (expected == 10) return false;
        return (number.charAt(18) - '0') == expected;
    }

    /**
     * Generise jedinstveni 19-cifreni broj racuna.
     * <p>
     * Proces:
     * <ol>
     *   <li>Genera se random deo od 9 cifara</li>
     *   <li>Gradi se broj sa fiksnim kodom banke (111), fijale (0001), random delom i tipom racuna</li>
     *   <li>Dodaje se 2-cifreni kod tipa racuna</li>
     *   <li>Racuna se kontrolna cifra i dodaje kao 19. cifra</li>
     *   <li>Ako broj vec postoji u bazi, proces se ponavlja</li>
     *   <li>Kada je broj validan i jedinstven, vraća se</li>
     * </ol>
     *
     * @param typeVal kod tipa racuna kao string (npr. "11" za licni tekuci, "21" za poslovni)
     * @param random instanca {@link Random} klase za generisanje nasumičnih cifara
     * @param accountRepository repository za proveru jedinstvnosti broja u bazi
     * @return jedinstveni, validan 19-cifreni broj racuna
     */
    public static String generate(String typeVal, Random random, AccountRepository accountRepository) {
        StringBuilder sb = new StringBuilder();
        String val = "";
        boolean exists = true;
        while (exists) {
            sb.setLength(0);
            for (int i = 0; i < 9; i++) {
                sb.append(random.nextInt(10));
            }
            val = "111" + "0001" + sb + typeVal;
            int checkDigit = calculateCheckDigit(val);
            if (checkDigit == 10) continue;
            val += checkDigit;
            exists = accountRepository.existsByBrojRacuna(val);
        }
        return val;
    }
}
