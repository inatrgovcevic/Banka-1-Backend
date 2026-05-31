package com.banka1.marketservice.stock.service;

import com.banka1.marketservice.stock.client.AlphaVantageClient;
import com.banka1.marketservice.stock.dto.StockPriceSnapshotDto;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

/**
 * Stock price feed cache (PR_12 C12.1; PR_13 C13.2 AlphaVantage; PR_19 C19.X Redis L2).
 *
 * <p>Pri prvom pozivu za neki ticker, dohvata sa AlphaVantage Quote API-ja
 * (kroz {@link AlphaVantageClient}). Posle toga vraca rezultat iz cache-a do TTL-a.
 * Ako nema API key-a ili AlphaVantage ne odgovori, vraca dev-mock snapshot tako da
 * frontend OTC vizualizacija nikad ne padne na blank stranici.
 *
 * <p>Cache hijerarhija:
 * <ol>
 *   <li>Redis L2 (kada je {@code spring.data.redis.host} setovan) — cross-replica sharing,
 *       redz Redis container shared 7-servisnog setup-a (PR_19 C19.X).</li>
 *   <li>In-process ConcurrentHashMap fallback — koristi se u unit testovima i kada
 *       Redis konekcija nije dostupna (graceful degradacija).</li>
 * </ol>
 * 15s default TTL je pragmatic kompromis — dovoljno brz da frontend ne probija
 * AlphaVantage free-tier limit (5 zahteva/min, 500/dan).
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class StockPriceFeedService {

    private static final String REDIS_KEY_PREFIX = "stock:price:";

    private final Map<String, CachedSnapshot> localCache = new ConcurrentHashMap<>();

    @Autowired(required = false)
    private AlphaVantageClient alphaVantageClient;

    @Autowired(required = false)
    private RedisTemplate<String, StockPriceSnapshotDto> stockPriceRedisTemplate;

    @Value("${stock.price-feed.cache-ttl-seconds:15}")
    private long cacheTtlSeconds = 15;

    public StockPriceSnapshotDto getCurrentPrice(String ticker) {
        String upper = ticker.toUpperCase();

        StockPriceSnapshotDto cached = readFromCache(upper);
        if (cached != null) {
            return cached;
        }

        StockPriceSnapshotDto fresh = fetchFromUpstream(upper);
        if (fresh != null) {
            writeToCache(upper, fresh);
        }
        return fresh;
    }

    public List<StockPriceSnapshotDto> getCurrentPrices(List<String> tickers) {
        return tickers.stream()
                .map(this::getCurrentPrice)
                .filter(java.util.Objects::nonNull)
                .collect(Collectors.toList());
    }

    private StockPriceSnapshotDto readFromCache(String ticker) {
        if (stockPriceRedisTemplate != null) {
            try {
                StockPriceSnapshotDto redisHit = stockPriceRedisTemplate.opsForValue().get(REDIS_KEY_PREFIX + ticker);
                if (redisHit != null) {
                    return redisHit;
                }
            } catch (DataAccessException ex) {
                log.warn("Redis unavailable za {} cache read — fallback na in-process: {}", ticker, ex.getMessage());
            }
        }
        CachedSnapshot localHit = localCache.get(ticker);
        if (localHit != null && Instant.now().isBefore(localHit.expiresAt)) {
            return localHit.snapshot;
        }
        return null;
    }

    private void writeToCache(String ticker, StockPriceSnapshotDto snapshot) {
        Duration ttl = Duration.ofSeconds(cacheTtlSeconds);
        if (stockPriceRedisTemplate != null) {
            try {
                stockPriceRedisTemplate.opsForValue().set(REDIS_KEY_PREFIX + ticker, snapshot, ttl);
                return;
            } catch (DataAccessException ex) {
                log.warn("Redis unavailable za {} cache write — fallback na in-process: {}", ticker, ex.getMessage());
            }
        }
        localCache.put(ticker, new CachedSnapshot(snapshot, Instant.now().plus(ttl)));
    }

    /**
     * Fetch sa AlphaVantage-a (PR_13 C13.2 real integracija).
     *
     * <p>Ako je AlphaVantage klijent dostupan i API key konfigurisan, poziva ga.
     * Ako vrati null (rate-limit, prazan response, network failure), vraca mock
     * snapshot tako da frontend OTC vizualizacija nikad ne padne. Mock je
     * dev-fallback; production deploy mora imati validan API key.
     */
    private StockPriceSnapshotDto fetchFromUpstream(String ticker) {
        log.debug("Fetching price feed za {} (cache miss)", ticker);

        if (alphaVantageClient != null) {
            AlphaVantageClient.Quote quote = alphaVantageClient.fetchQuote(ticker);
            if (quote != null && quote.price() != null) {
                return StockPriceSnapshotDto.builder()
                        .ticker(quote.ticker())
                        .currentPrice(quote.price())
                        .openPrice(quote.open())
                        .previousClose(quote.previousClose())
                        .changePercent(quote.changePercent())
                        .volume(quote.volume())
                        .currency("USD")  // AlphaVantage vraca USD za US listings; multi-currency je TBD
                        .timestamp(Instant.now())
                        .build();
            }
        }

        // Fallback dev-mock kada nema AlphaVantage konekcije.
        log.debug("Vracam dev-mock za {} (AlphaVantage unavailable)", ticker);
        return StockPriceSnapshotDto.builder()
                .ticker(ticker)
                .currentPrice(new BigDecimal("150.25"))
                .openPrice(new BigDecimal("148.00"))
                .previousClose(new BigDecimal("149.50"))
                .changePercent(new BigDecimal("0.50"))
                .volume(1_000_000L)
                .currency("USD")
                .timestamp(Instant.now())
                .build();
    }

    private record CachedSnapshot(StockPriceSnapshotDto snapshot, Instant expiresAt) {}
}
