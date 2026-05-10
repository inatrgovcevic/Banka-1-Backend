package com.banka1.banking_service.account_service.service.implementation;

import com.banka1.banking_service.account_service.domain.*;
import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.domain.enums.Status;
import com.banka1.banking_service.account_service.dto.request.*;
import com.banka1.banking_service.account_service.dto.response.*;
import com.banka1.banking_service.account_service.rabbitMQ.*;
import com.banka1.banking_service.account_service.repository.AccountRepository;
import com.banka1.banking_service.account_service.repository.CompanyRepository;
import com.banka1.banking_service.account_service.repository.CurrencyRepository;
import com.banka1.banking_service.account_service.repository.SifraDelatnostiRepository;
import com.banka1.banking_service.account_service.rest_client.RestClientService;
import com.banka1.banking_service.account_service.service.EmployeeService;
import com.banka1.banking_service.account_service.util.AccountNumberGenerator;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Random;


//@RequiredArgsConstructor
@Service
public class EmployeeServiceImplementation implements EmployeeService {
    private final Random random;
    private final RestClientService restClientService;
    private final AccountRepository accountRepository;
    @Value("${banka.security.id}")
    private String appPropertiesId;
    private final CurrencyRepository currencyRepository;
    private final SifraDelatnostiRepository sifraDelatnostiRepository;
    private final CompanyRepository companyRepository;
    private final RabbitClient rabbitClient;

    public EmployeeServiceImplementation(@Value("${my.random.seed}") Long seed, RestClientService restClientService, CurrencyRepository currencyRepository, SifraDelatnostiRepository sifraDelatnostiRepository, CompanyRepository companyRepository, AccountRepository accountRepository, RabbitClient rabbitClient)
    {
        this.restClientService = restClientService;
        this.rabbitClient = rabbitClient;
        this.random = seed != null ? new Random(seed) : new Random();
        this.currencyRepository=currencyRepository;
        this.sifraDelatnostiRepository=sifraDelatnostiRepository;
        this.companyRepository=companyRepository;
        this.accountRepository = accountRepository;
    }

    private void validateFxDto(FxDto dto) {
        if (dto.getCurrencyCode() == CurrencyCode.RSD)
            throw new IllegalArgumentException("Ne moze RSD");
        validateOwner(dto.getIdVlasnika(), dto.getJmbg());
        if ((dto.getFirma() == null && dto.getTipRacuna() == AccountOwnershipType.BUSINESS)
                || (dto.getFirma() != null && dto.getTipRacuna() == AccountOwnershipType.PERSONAL))
            throw new IllegalArgumentException("Pogresan tip racuna");
    }

    private void validateCheckingDto(CheckingDto dto) {
        validateOwner(dto.getIdVlasnika(), dto.getJmbg());

        if ((dto.getFirma() == null && dto.getVrstaRacuna().getAccountOwnershipType() == AccountOwnershipType.BUSINESS)
                || (dto.getFirma() != null && dto.getVrstaRacuna().getAccountOwnershipType() == AccountOwnershipType.PERSONAL))
            throw new IllegalArgumentException("Pogresan tip racuna");
    }

    private void validateOwner(Long id, String jmbg) {
        if (id == null && (jmbg == null || jmbg.isBlank()))
            throw new IllegalArgumentException("Unesi id ili jmbg");
    }

    private Currency getCurrencyOrThrow(CurrencyCode code) {
        Currency currency = currencyRepository.findByOznaka(code).orElse(null);
        if (currency == null)
            throw new IllegalArgumentException("Nisu unete valute");
        if (currency.getStatus() == Status.INACTIVE)
            throw new IllegalArgumentException("Deaktivirana valuta");
        return currency;
    }
    private Company createCompanyIfNeeded(FirmaDto firmaDto) {
        if (firmaDto == null) return null;
        SifraDelatnosti sifra = sifraDelatnostiRepository
                .findBySifra(firmaDto.getSifraDelatnosti())
                .orElse(null);
        if (sifra == null)
            throw new IllegalArgumentException("Nije uneta sifra delatnosti");
        Company company = new Company(
                firmaDto.getNaziv(),
                firmaDto.getMaticniBroj(),
                firmaDto.getPoreskiBroj(),
                sifra,
                firmaDto.getAdresa(),
                firmaDto.getVlasnik()
        );

        return companyRepository.save(company);
    }

    //todo menjati gresku
    private ClientInfoResponseDto resolveClientId(Long id, String jmbg) {
        ClientInfoResponseDto clientInfoResponseDto;
        if (id != null)
            clientInfoResponseDto= restClientService.getUser(id);
        else
            clientInfoResponseDto= restClientService.getUser(jmbg);
        if(clientInfoResponseDto==null)
            throw new RuntimeException("Greska sa komunikacijom izmedju servisa");
        clientInfoResponseDto.setUsername(clientInfoResponseDto.getName()+"_"+clientInfoResponseDto.getLastName());
        return clientInfoResponseDto;
    }


    private String generateAccountNumber(String typeVal) {
        return AccountNumberGenerator.generate(typeVal, random, accountRepository);
    }

    private void populateAccount(Account account,
                                 String broj,
                                 String naziv,
                                 Long vlasnikId,
                                 String name,
                                 String surname,
                                 String username,
                                 String email,
                                 Jwt jwt,
                                 Currency currency,
                                 Company company,
                                 BigDecimal balance) {

        account.setBrojRacuna(broj);
        account.setImeVlasnikaRacuna(name);
        account.setPrezimeVlasnikaRacuna(surname);
        account.setNazivRacuna(naziv);
        account.setVlasnik(vlasnikId);
        account.setUsername(username);
        account.setEmail(email);
        account.setZaposlen(((Number) jwt.getClaim(appPropertiesId)).longValue());
        account.setDatumIVremeKreiranja(LocalDateTime.now());
        account.setCurrency(currency);
        account.setCompany(company);
        account.setStanje(balance);
        account.setRaspolozivoStanje(balance);
        account.setDnevniLimit(new BigDecimal("250000.00"));
        account.setMesecniLimit(new BigDecimal("1000000.00"));
        account.setDatumIsteka(LocalDate.now().plusYears(5));
    }

    @Transactional
    @Override
    public AccountDetailsResponseDto createFxAccount(Jwt jwt, FxDto fxDto) {
        validateFxDto(fxDto);
        Currency currency = getCurrencyOrThrow(fxDto.getCurrencyCode());
        Company company = createCompanyIfNeeded(fxDto.getFirma());
        ClientInfoResponseDto clientInfoResponseDto = resolveClientId(fxDto.getIdVlasnika(), fxDto.getJmbg());
        String broj = generateAccountNumber(String.valueOf(fxDto.getTipRacuna().getVal()));
        Account account = new FxAccount(fxDto.getTipRacuna());
        populateAccount(account, broj, fxDto.getNazivRacuna(), clientInfoResponseDto.getId(), clientInfoResponseDto.getName(),clientInfoResponseDto.getLastName(), clientInfoResponseDto.getUsername(), clientInfoResponseDto.getEmail(), jwt, currency, company,fxDto.getInitialBalance());
        accountRepository.save(account);
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                rabbitClient.sendEmailNotification(new EmailDto(clientInfoResponseDto.getUsername(),clientInfoResponseDto.getEmail(), EmailType.ACCOUNT_CREATED));
                if (Boolean.TRUE.equals(fxDto.getCreateCard())) {
                    rabbitClient.sendCardEvent(new CardEventDto(clientInfoResponseDto.getId(), account.getBrojRacuna(), CardEventType.CARD_CREATE));
                }
            }
        });
        return new AccountDetailsResponseDto(account);
    }

    @Transactional
    @Override
    public AccountDetailsResponseDto createCheckingAccount(Jwt jwt, CheckingDto checkingDto) {
        validateCheckingDto(checkingDto);
        Currency currency = getCurrencyOrThrow(CurrencyCode.RSD);
        Company company = createCompanyIfNeeded(checkingDto.getFirma());
        ClientInfoResponseDto clientInfoResponseDto = resolveClientId(checkingDto.getIdVlasnika(), checkingDto.getJmbg());
        String broj = generateAccountNumber(String.valueOf(checkingDto.getVrstaRacuna().getVal()));
        Account account = new CheckingAccount(checkingDto.getVrstaRacuna());
        populateAccount(account, broj, checkingDto.getNazivRacuna(), clientInfoResponseDto.getId(), clientInfoResponseDto.getName(),clientInfoResponseDto.getLastName(), clientInfoResponseDto.getUsername(), clientInfoResponseDto.getEmail(), jwt, currency, company,checkingDto.getInitialBalance());
        accountRepository.save(account);
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                rabbitClient.sendEmailNotification(new EmailDto(clientInfoResponseDto.getUsername(),clientInfoResponseDto.getEmail(), EmailType.ACCOUNT_CREATED));
                if (Boolean.TRUE.equals(checkingDto.getCreateCard())) {
                    rabbitClient.sendCardEvent(new CardEventDto(clientInfoResponseDto.getId(), account.getBrojRacuna(), CardEventType.CARD_CREATE));
                }
            }
        });
        return new AccountDetailsResponseDto(account);
    }

    private String myTrim(String s)
    {
        if(s!=null)
            return s.trim();
        return s;
    }

    @Transactional
    public Page<AccountSearchResponseDto> searchAllAccounts(Jwt jwt,String ime,String prezime,String accountNumber,int page,int size)
    {
        return accountRepository.searchAccounts(myTrim(accountNumber),myTrim(ime),myTrim(prezime),PageRequest.of(page,size)).map(AccountSearchResponseDto::new);
    }

    @Override
    public List<AccountDetailsResponseDto> getBankAccounts() {
        return accountRepository.findAllBankAccounts().stream()
                .map(AccountDetailsResponseDto::new)
                .toList();
    }

    @Override
    public AccountDetailsResponseDto getBankAccountByCurrency(CurrencyCode currencyCode) {
        Account account = accountRepository.findBankAccountByCurrencyCode(currencyCode)
                .orElseThrow(() -> new IllegalArgumentException("Nije pronadjen bankarski racun za valutu: " + currencyCode));
        return new AccountDetailsResponseDto(account);
    }

    @Override
    @Transactional
    public AccountDetailsResponseDto getAccountDetails(String accountNumber) {
        Account account = accountRepository.findByBrojRacuna(accountNumber)
                .orElseThrow(() -> new IllegalArgumentException("Ne postoji racun: " + accountNumber));
        return new AccountDetailsResponseDto(account);
    }

    @Override
    @Transactional
    public Page<AccountDetailsResponseDto> getClientAccounts(Long clientId, int page, int size) {
        return accountRepository.findByVlasnikAndStatus(clientId, Status.ACTIVE, PageRequest.of(page, size))
                .map(AccountDetailsResponseDto::new);
    }

    // Cards are managed by the Card Service — call Card Service directly instead
    @Override
    public Page<CardResponseDto> getAccountCards(String accountNumber, int page, int size) {
        throw new UnsupportedOperationException("Card management is handled by the Card Service");
//        Account account = accountRepository.findByBrojRacuna(accountNumber)
//                .orElseThrow(() -> new IllegalArgumentException("Ne postoji racun: " + accountNumber));
//        List<CardResponseDto> cards = cardServiceRestClient.getCardsForAccount(account.getBrojRacuna());
//        int start = page * size;
//        int end = Math.min(start + size, cards.size());
//        List<CardResponseDto> pageContent = start >= cards.size() ? List.of() : cards.subList(start, end);
//        return new org.springframework.data.domain.PageImpl<>(pageContent, PageRequest.of(page, size), cards.size());
    }

    @Override
    @Transactional
    public CompanyResponseDto getCompany(Long id) {
        Company company = companyRepository.findById(id)
                .orElseThrow(() -> new IllegalArgumentException("Ne postoji firma sa id: " + id));
        return new CompanyResponseDto(company);
    }

    @Override
    @Transactional
    public CompanyResponseDto updateCompany(Long id, UpdateCompanyDto dto) {
        Company company = companyRepository.findById(id)
                .orElseThrow(() -> new IllegalArgumentException("Ne postoji firma sa id: " + id));
        SifraDelatnosti sifra = sifraDelatnostiRepository.findBySifra(dto.getSifraDelatnosti())
                .orElseThrow(() -> new IllegalArgumentException("Nije uneta sifra delatnosti: " + dto.getSifraDelatnosti()));
        company.setNaziv(dto.getNaziv());
        company.setSifraDelatnosti(sifra);
        company.setAdresa(dto.getAdresa());
        if (dto.getVlasnik() != null) {
            company.setVlasnik(dto.getVlasnik());
        }
        return new CompanyResponseDto(companyRepository.save(company));
    }

    //todo rabit mq
    @Transactional
    @Override
    public String updateCard(Jwt jwt, Long id, UpdateCardDto updateCardDto) {
//        Account account=accountRepository.findById(id).orElse(null);
//        if(account==null)
//            throw new IllegalArgumentException("Ne postoji racun sa tim id");
//        if(!account.getVlasnik().equals(((Number) jwt.getClaim(appPropertiesId)).longValue()))
//            throw new IllegalArgumentException("Nisi vlasnik racuna");
//        if(account.getStatus()==updateCardDto.getCardStatus())
//            throw new IllegalArgumentException("Kartica je vec "+account.getStatus().name());
//        if(account.getStatus()==CardStatus.DEACTIVATED && updateCardDto.getCardStatus()==CardStatus.BLOCKED
//            || account.getStatus()==CardStatus.BLOCKED && updateCardDto.getCardStatus()==CardStatus.ACTIVATED)
//            throw new IllegalArgumentException("Ne moze "+account.getStatus().name()+" u "+updateCardDto.getCardStatus());
//
//        account.setStatus(updateCardDto.getCardStatus());
        return "Uspesno updetovan status";
    }
}
