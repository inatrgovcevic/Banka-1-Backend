package com.banka1.tradingservice.otc.domain;

public enum OtcOfferStatus {
    /** Cek na akciju prodavca (poslednju kontraponudu poslao kupac). */
    PENDING_SELLER,
    /** Cek na akciju kupca (poslednju kontraponudu poslao prodavac). */
    PENDING_BUYER,
    /** Prodavac prihvatio — saga-orchestrator je inicirao premium transfer. */
    ACCEPTED,
    /** Jedna strana odustala (primila ponudu i odbila je). */
    REJECTED,
    /** Strana koja je poslala ponudu povukla je pre nego sto je druga strana odgovorila. */
    WITHDRAWN,
    /** Settlement date prosao bez prihvatanja. */
    EXPIRED
}
