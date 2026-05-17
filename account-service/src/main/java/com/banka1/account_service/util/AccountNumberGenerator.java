package com.banka1.account_service.util;

import com.banka1.account_service.repository.AccountRepository;

import java.util.Random;

/**
 * Utility klasa za generisanje i validaciju 18-cifrenih bankovskih brojeva racuna.
 * <p>
 * Format prema specifikaciji (Celina 2):
 * <pre>
 *   Pozicije 1-3   : Sifra banke (fiksno 111 za Banku1)
 *   Pozicije 4-7   : Sifra filijale (fiksno 0001)
 *   Pozicije 8-16  : 9 nasumicnih cifara (jedinstveni deo)
 *   Pozicije 17-18 : Tip racuna (npr. 11 - licni tekuci, 21 - poslovni FX)
 * </pre>
 * <p>
 * Validnost: zbir svih 18 cifara mora biti deljiv sa 11 ((zbir svih cifara) % 11 == 0).
 * Pri generisanju se prvih 8 random cifara bira slucajno, a 9. cifra se izabira tako
 * da uslov bude ispunjen; ako bi taj izbor bio 10, slucajan deo se regenerise.
 * <p>
 * Primer:
 * <ul>
 *   <li>1110001 000000005 11 -> sum 1+1+1+0+0+0+1+0+0+0+0+0+0+0+0+5+1+1 = 11 -> 11%11=0 OK</li>
 * </ul>
 */
public final class AccountNumberGenerator {

    /** Konstantni prefix banke + filijale (1-7). */
    private static final String FIXED_PREFIX = "1110001";

    /** Suma cifara fiksnog prefix-a (1+1+1+0+0+0+1). */
    private static final int FIXED_PREFIX_DIGIT_SUM = 4;

    /** Maksimalan broj pokusaja generisanja jedinstvenog broja pre nego sto se baci greska. */
    private static final int MAX_GENERATION_ATTEMPTS = 1024;

    private AccountNumberGenerator() {}

    /**
     * Sabira sve cifre datog stringa.
     *
     * @param digits string cifara
     * @return zbir cifara
     */
    public static int digitSum(String digits) {
        int sum = 0;
        for (char c : digits.toCharArray()) {
            sum += c - '0';
        }
        return sum;
    }

    /**
     * Validira 18-cifreni broj racuna proveravanjem strukture i mod-11 zbira.
     *
     * @param number broj racuna za validaciju
     * @return {@code true} ako je broj validan, {@code false} inace
     */
    public static boolean validateAccountNumber(String number) {
        if (number == null || number.length() != 18) return false;
        for (char c : number.toCharArray()) {
            if (!Character.isDigit(c)) return false;
        }
        if (!number.startsWith(FIXED_PREFIX)) return false;

        String typeVal = number.substring(16, 18);
        if (!typeVal.matches("1[1-7]|2[1-2]")) return false;

        return digitSum(number) % 11 == 0;
    }

    /**
     * Generise jedinstveni 18-cifreni broj racuna.
     * <p>
     * Postupak:
     * <ol>
     *   <li>Generise 8 nasumicnih cifara.</li>
     *   <li>Bira 9. cifru tako da zbir svih 18 cifara bude deljiv sa 11.</li>
     *   <li>Ako bi 9. cifra morala biti 10 (kombinacija nedopustiva), regenerise se.</li>
     *   <li>Ako broj vec postoji u bazi, regenerise se.</li>
     * </ol>
     *
     * @param typeVal kod tipa racuna kao 2-cifreni string (npr. "11", "21")
     * @param random instanca {@link Random} za nasumicne cifre
     * @param accountRepository repository za proveru jedinstvenosti
     * @return jedinstveni, validan 18-cifreni broj racuna
     * @throws IllegalStateException ako se posle {@value #MAX_GENERATION_ATTEMPTS} pokusaja ne nadje jedinstven broj
     */
    public static String generate(String typeVal, Random random, AccountRepository accountRepository) {
        for (int attempt = 0; attempt < MAX_GENERATION_ATTEMPTS; attempt++) {
            StringBuilder randomPart = new StringBuilder(9);
            int sum = FIXED_PREFIX_DIGIT_SUM
                    + (typeVal.charAt(0) - '0')
                    + (typeVal.charAt(1) - '0');
            for (int i = 0; i < 8; i++) {
                int d = random.nextInt(10);
                randomPart.append(d);
                sum += d;
            }
            int last = (11 - sum % 11) % 11;
            if (last == 10) continue; // mod-11 ne dozvoljava 10 kao cifru -> regenerisati
            randomPart.append(last);
            String val = FIXED_PREFIX + randomPart + typeVal;
            if (!accountRepository.existsByBrojRacuna(val)) return val;
        }
        throw new IllegalStateException(
                "Nije moguce generisati jedinstven broj racuna ni posle "
                        + MAX_GENERATION_ATTEMPTS + " pokusaja");
    }
}
