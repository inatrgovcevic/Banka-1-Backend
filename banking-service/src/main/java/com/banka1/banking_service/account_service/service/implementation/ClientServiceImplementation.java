package com.banka1.banking_service.account_service.service.implementation;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.enums.Status;
import com.banka1.banking_service.account_service.dto.request.EditAccountLimitDto;
import com.banka1.banking_service.account_service.dto.request.EditAccountNameDto;
import com.banka1.banking_service.account_service.dto.request.EditStatus;
import com.banka1.banking_service.account_service.dto.response.AccountDetailsResponseDto;
import com.banka1.banking_service.account_service.dto.response.AccountResponseDto;
import com.banka1.banking_service.account_service.dto.response.CardResponseDto;
import com.banka1.banking_service.account_service.dto.response.VerificationStatusResponse;
import com.banka1.banking_service.account_service.exception.BusinessException;
import com.banka1.banking_service.account_service.exception.ErrorCode;
import com.banka1.banking_service.account_service.rabbitMQ.*;
import com.banka1.banking_service.account_service.repository.AccountRepository;
import com.banka1.banking_service.account_service.rest_client.RestClientService;
import com.banka1.banking_service.account_service.rest_client.VerificationService;
import com.banka1.banking_service.account_service.service.ClientService;
import com.banka1.banking_service.card_service.dto.card_management.response.CardInternalSummaryDTO;
import com.banka1.banking_service.card_service.service.CardLifecycleService;
import lombok.RequiredArgsConstructor;
import org.jspecify.annotations.NonNull;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.util.List;

@Service
@RequiredArgsConstructor
public class ClientServiceImplementation implements ClientService {
    private final AccountRepository accountRepository;
    @Value("${banka.security.id}")
    private String appPropertiesId;

    private final VerificationService verificationService;
    @Value("${account.verification.skip:true}")
    private boolean skipVerification;
    private final RabbitClient rabbitClient;
    private final RestClientService restClientService;
    private final CardLifecycleService cardLifeCycleService;


//    @Override
//    public String newPayment(Jwt jwt, NewPaymentDto newPaymentDto) {
//        return "";
//    }
//
//    //todo kada dodje mobile
//    @Override
//    public String approveTransaction(Jwt jwt, Long id, ApproveDto newPaymentDto) {
//        return "";
//    }


    @Transactional
    @Override
    public Page<AccountResponseDto> findMyAccounts(Jwt jwt, int page, int size) {
        return accountRepository.findByVlasnikAndStatus(((Number) jwt.getClaim(appPropertiesId)).longValue(), Status.ACTIVE,PageRequest.of(page, size)).map(AccountResponseDto::new);
    }

//    @Transactional
//    @Override
//    public Page<TransactionResponseDto> findAllTransactions(Jwt jwt, Long id, int page, int size) {
//        Account account=accountRepository.findById(id).orElse(null);
//        if(account==null)
//            throw new IllegalArgumentException("Ne postoji unet racun");
//        if(!account.getVlasnik().equals(((Number) jwt.getClaim(appPropertiesId)).longValue()))
//            throw new IllegalArgumentException("Nisi vlasnik racuna");
//
//        return null;
//    }

    //TODO menjati exceptione
    private void  validation(Account account,Jwt jwt)
    {
        if(account==null)
            throw new IllegalArgumentException("Ne postoji unet racun");
        if(!account.getVlasnik().equals(((Number) jwt.getClaim(appPropertiesId)).longValue()))
            throw new IllegalArgumentException("Nisi vlasnik racuna");
    }

    @Transactional
    @Override
    public String editAccountName(Jwt jwt, Long id, EditAccountNameDto editAccountNameDto) {
        Account account=accountRepository.findById(id).orElse(null);
        return editName(jwt, editAccountNameDto, account);
    }

    @Transactional
    @Override
    public String editAccountName(Jwt jwt, String accountNumber, EditAccountNameDto editAccountNameDto) {
        Account account=accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        return editName(jwt, editAccountNameDto, account);
    }

    @NonNull
    private String editName(Jwt jwt, EditAccountNameDto editAccountNameDto, Account account) {
        validation(account,jwt);
        if(account.getNazivRacuna().equalsIgnoreCase(editAccountNameDto.getAccountName()))
            throw new IllegalArgumentException("Ime ne sme biti isto");
        if(accountRepository.existsByVlasnikAndNazivRacuna(account.getVlasnik(),editAccountNameDto.getAccountName()))
            throw new IllegalArgumentException("Vlasnik poseduje racun sa ovim imenom");
        account.setNazivRacuna(editAccountNameDto.getAccountName());
        return "Uspesno editovano ime";
    }

    @Transactional
    @Override
    public String editAccountLimit(Jwt jwt, Long id, EditAccountLimitDto editAccountLimitDto) {
        Account account=accountRepository.findById(id).orElse(null);
        return editLimit(jwt, editAccountLimitDto, account);
    }

    @Transactional
    @Override
    public String editAccountLimit(Jwt jwt, String accountNumber, EditAccountLimitDto editAccountLimitDto) {
        Account account=accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        return editLimit(jwt, editAccountLimitDto, account);
    }

    @NonNull
    private String editLimit(Jwt jwt, EditAccountLimitDto editAccountLimitDto, Account account) {
        validation(account,jwt);

        if(editAccountLimitDto.getDailyLimit().compareTo(editAccountLimitDto.getMonthlyLimit()) > 0)
            throw new IllegalArgumentException("Dnevni limit mora biti manji ili jednak od mesecnog");

        if (!skipVerification) {
            VerificationStatusResponse verificationStatusResponse = verificationService.getStatus(editAccountLimitDto.getVerificationSessionId());
            if (verificationStatusResponse == null || !verificationStatusResponse.isVerified())
                throw new BusinessException(ErrorCode.VERIFICATION_FAILED, ErrorCode.VERIFICATION_FAILED.getTitle());
        }

        account.setDnevniLimit(editAccountLimitDto.getDailyLimit());
        account.setMesecniLimit(editAccountLimitDto.getMonthlyLimit());

        return "Uspesno setovani limiti";
    }

    @Transactional
    @Override
    public AccountDetailsResponseDto getDetails(Jwt jwt, Long id) {
        Account account=accountRepository.findById(id).orElse(null);
        validation(account,jwt);
        AccountDetailsResponseDto dto = new AccountDetailsResponseDto(account);
        dto.setCards(toCardResponseDtos(cardLifeCycleService.getInternalCardsByAccountNumber(account.getBrojRacuna())));
        return dto;
    }

    @Override
    @Transactional
    public AccountDetailsResponseDto getDetails(Jwt jwt, String accountNumber) {
        Account account=accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        validation(account,jwt);
        AccountDetailsResponseDto dto = new AccountDetailsResponseDto(account);
        dto.setCards(toCardResponseDtos(cardLifeCycleService.getInternalCardsByAccountNumber(account.getBrojRacuna())));
        return dto;
    }

    @Override
    public Page<CardResponseDto> findAllCards(Jwt jwt, Long id, int page, int size) {
        Account account = accountRepository.findById(id).orElse(null);
        validation(account, jwt);
        List<CardResponseDto> cards = toCardResponseDtos(cardLifeCycleService.getInternalCardsByAccountNumber(account.getBrojRacuna()));
        int start = page * size;
        int end = Math.min(start + size, cards.size());
        List<CardResponseDto> pageContent = start >= cards.size() ? List.of() : cards.subList(start, end);
        return new org.springframework.data.domain.PageImpl<>(pageContent, PageRequest.of(page, size), cards.size());
    }

    private List<CardResponseDto> toCardResponseDtos(List<CardInternalSummaryDTO> cards) {
        return cards.stream()
                .map(card -> new CardResponseDto(
                        card.getId(),
                        card.getCardNumber(),
                        card.getCardType(),
                        card.getStatus(),
                        card.getExpiryDate(),
                        card.getAccountNumber()
                ))
                .toList();
    }

    @Override
    @Transactional
    public String editStatus(Jwt jwt, String accountNumber, EditStatus editStatus) {
        Account account=accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        if(account==null)
            throw new IllegalArgumentException("Ne postoji racun: " + accountNumber);
        account.setStatus(editStatus.getStatus());
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                if (editStatus.getStatus() == Status.INACTIVE) {
                    if (account.getUsername() != null && account.getEmail() != null) {
                        rabbitClient.sendEmailNotification(new EmailDto(account.getUsername(), account.getEmail(), EmailType.ACCOUNT_DEACTIVATED));
                    }
                    rabbitClient.sendCardEvent(new CardEventDto(account.getVlasnik(), account.getBrojRacuna(), CardEventType.CARD_DEACTIVATE));
                }
            }
        });
        return "Uspesno editovan status";
    }
}
