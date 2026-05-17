package com.banka1.account_service.util;

import com.banka1.account_service.repository.AccountRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Random;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class AccountNumberGeneratorTest {

    private static class StubRandom extends Random {
        private final int[] values;
        private int index = 0;

        private StubRandom(int... values) {
            this.values = values;
        }

        @Override
        public int nextInt(int bound) {
            return values[index++];
        }
    }

    @Mock
    private AccountRepository accountRepository;

    @Test
    void generateReturns18DigitNumber() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        String number = AccountNumberGenerator.generate("11", new Random(42), accountRepository);

        assertThat(number).hasSize(18).matches("\\d{18}");
    }

    @Test
    void generateStartsWithBankAndBranchCode() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        String number = AccountNumberGenerator.generate("21", new Random(1), accountRepository);

        assertThat(number).startsWith("1110001");
    }

    @Test
    void generateEndsWithProvidedTypeCode() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        String number = AccountNumberGenerator.generate("22", new Random(1), accountRepository);

        assertThat(number.substring(16, 18)).isEqualTo("22");
    }

    @Test
    void generateRetriesOnCollision() {
        // Prva dva poziva vracaju kolidirajuce brojeve, treci je jedinstven.
        when(accountRepository.existsByBrojRacuna(anyString()))
                .thenReturn(true)
                .thenReturn(true)
                .thenReturn(false);

        String number = AccountNumberGenerator.generate("11", new Random(99), accountRepository);

        assertThat(number).hasSize(18);
    }

    @Test
    void generatedNumberSatisfiesMod11Check() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        String number = AccountNumberGenerator.generate("11", new Random(7), accountRepository);

        assertThat(AccountNumberGenerator.digitSum(number) % 11).isEqualTo(0);
    }

    @Test
    void generateSkipsCandidateWhenLastDigitWouldBeTen() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        // Tip 11: fiksni zbir = 4 + 1 + 1 = 6.
        // Prvih 8 random nula -> sum=6, (11 - 6%11)%11 = 5 OK; broj se prihvata.
        // Da forsiramo skip kada bi 9. bila 10, koristimo 8 random cifara cija je suma 5
        // (5 + 6 = 11 -> 11%11=0 -> last = 0). Hmm to nije skip.
        //
        // Skip se desava kada (sum + 0..9 mod 11) zahteva da last bude 10, sto je
        // kada sum % 11 == 1 (1 -> last = 10). Sa fiksnim sumom 6, treba 8 random cija
        // suma daje sum%11==1, tj. 8 random suma = (1 - 6 + 11) mod 11 = 6.
        // 8 random cifara sa sumom 6: npr. 6,0,0,0,0,0,0,0.
        // Zatim drugi pokusaj sa svim nulama -> sum=6 -> last=5 -> validan broj.
        Random random = new StubRandom(
                6, 0, 0, 0, 0, 0, 0, 0,   // 1. pokusaj: skip
                0, 0, 0, 0, 0, 0, 0, 0    // 2. pokusaj: prihvati
        );

        String number = AccountNumberGenerator.generate("11", random, accountRepository);

        assertThat(number).isEqualTo("111000100000000511");
        assertThat(AccountNumberGenerator.digitSum(number) % 11).isEqualTo(0);
    }

    @Test
    void generateThrowsAfterMaxAttempts() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(true);

        assertThatThrownBy(() -> AccountNumberGenerator.generate("11", new Random(1), accountRepository))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("jedinstven");
    }

    @Test
    void validateAccountNumberReturnsTrueForValidNumber() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);

        String number = AccountNumberGenerator.generate("11", new Random(7), accountRepository);

        assertThat(AccountNumberGenerator.validateAccountNumber(number)).isTrue();
    }

    @Test
    void validateAccountNumberReturnsFalseForNull() {
        assertThat(AccountNumberGenerator.validateAccountNumber(null)).isFalse();
    }

    @Test
    void validateAccountNumberReturnsFalseForWrongLength() {
        assertThat(AccountNumberGenerator.validateAccountNumber("12345")).isFalse();
        // 19 cifara - bivsi format vise nije validan
        assertThat(AccountNumberGenerator.validateAccountNumber("1110001000000000115")).isFalse();
    }

    @Test
    void validateAccountNumberReturnsFalseForNonDigits() {
        assertThat(AccountNumberGenerator.validateAccountNumber("11100011234567AB11")).isFalse();
    }

    @Test
    void validateAccountNumberReturnsFalseForWrongPrefix() {
        // Prefix 222 umesto 111
        assertThat(AccountNumberGenerator.validateAccountNumber("222000110000000011")).isFalse();
    }

    @Test
    void validateAccountNumberReturnsFalseForUnknownTypeCode() {
        // Type 99 nije podrzan
        assertThat(AccountNumberGenerator.validateAccountNumber("111000112345678999")).isFalse();
    }

    @Test
    void validateAccountNumberReturnsFalseWhenSumNotDivisibleBy11() {
        when(accountRepository.existsByBrojRacuna(anyString())).thenReturn(false);
        String valid = AccountNumberGenerator.generate("11", new Random(5), accountRepository);

        // Pokvari jednu cifru tako da sum%11 vise ne bude 0.
        char[] chars = valid.toCharArray();
        int idx = 8; // jedna od random cifara
        int newDigit = (chars[idx] - '0' + 1) % 10;
        chars[idx] = (char) ('0' + newDigit);
        String corrupted = new String(chars);

        assertThat(AccountNumberGenerator.validateAccountNumber(corrupted)).isFalse();
    }

    @Test
    void digitSumComputesUnweightedSum() {
        assertThat(AccountNumberGenerator.digitSum("111000100000000011")).isEqualTo(5);
        assertThat(AccountNumberGenerator.digitSum("999")).isEqualTo(27);
        assertThat(AccountNumberGenerator.digitSum("")).isEqualTo(0);
    }
}
