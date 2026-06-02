-- trading-service-go schema baseline (migration 001) — DO NOT hand-edit.
-- Generated from: pg_dump --schema-only of the live Java-Liquibase-owned 'trading' DB,
-- at the P8 cut-over. The psql meta-commands (restrict/unrestrict) are stripped so the
-- file runs through the pgx migration runner. On a DB Java Liquibase already provisioned,
-- RunMigrations baseline-skips this (no conflict); on a fresh DB it creates the full
-- trading schema (24 tables + sequences + constraints + indexes). Regenerate if the
-- Java schema changes.

--
-- PostgreSQL database dump
--


-- Dumped from database version 16.14
-- Dumped by pg_dump version 16.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
-- SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: actuary_info; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.actuary_info (
    id bigint NOT NULL,
    employee_id bigint NOT NULL,
    "limit" numeric(19,4),
    used_limit numeric(19,4) DEFAULT 0 NOT NULL,
    need_approval boolean DEFAULT false NOT NULL,
    reserved_limit numeric(19,4) DEFAULT 0 NOT NULL
);


--
-- Name: actuary_info_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.actuary_info_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: actuary_info_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.actuary_info_id_seq OWNED BY public.actuary_info.id;


--
-- Name: analytics_client_segments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.analytics_client_segments (
    id bigint NOT NULL,
    run_id character varying(36) NOT NULL,
    user_id bigint NOT NULL,
    cluster_id integer NOT NULL,
    segment_label character varying(48) NOT NULL,
    total_portfolio_value numeric(19,4) DEFAULT 0 NOT NULL,
    total_cost_basis numeric(19,4) DEFAULT 0 NOT NULL,
    unrealized_pnl numeric(19,4) DEFAULT 0 NOT NULL,
    holdings_count integer DEFAULT 0 NOT NULL,
    max_holding_percent numeric(9,4) DEFAULT 0 NOT NULL,
    order_count integer DEFAULT 0 NOT NULL,
    average_order_value numeric(19,4) DEFAULT 0 NOT NULL,
    buy_sell_ratio numeric(19,4) DEFAULT 0 NOT NULL,
    risk_score numeric(9,4) DEFAULT 0 NOT NULL,
    computed_at timestamp without time zone NOT NULL
);


--
-- Name: analytics_client_segments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.analytics_client_segments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: analytics_client_segments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.analytics_client_segments_id_seq OWNED BY public.analytics_client_segments.id;


--
-- Name: analytics_job_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.analytics_job_runs (
    run_id character varying(36) NOT NULL,
    job_name character varying(80) NOT NULL,
    status character varying(20) NOT NULL,
    started_at timestamp without time zone NOT NULL,
    completed_at timestamp without time zone,
    message character varying(512)
);


--
-- Name: analytics_portfolio_risk; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.analytics_portfolio_risk (
    id bigint NOT NULL,
    run_id character varying(36) NOT NULL,
    user_id bigint NOT NULL,
    total_market_value numeric(19,4) DEFAULT 0 NOT NULL,
    total_cost_basis numeric(19,4) DEFAULT 0 NOT NULL,
    unrealized_pnl numeric(19,4) DEFAULT 0 NOT NULL,
    holdings_count integer DEFAULT 0 NOT NULL,
    max_holding_percent numeric(9,4) DEFAULT 0 NOT NULL,
    diversification_score numeric(9,4) DEFAULT 0 NOT NULL,
    risk_score numeric(9,4) DEFAULT 0 NOT NULL,
    risk_level character varying(16) NOT NULL,
    computed_at timestamp without time zone NOT NULL
);


--
-- Name: analytics_portfolio_risk_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.analytics_portfolio_risk_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: analytics_portfolio_risk_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.analytics_portfolio_risk_id_seq OWNED BY public.analytics_portfolio_risk.id;


--
-- Name: analytics_top_tickers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.analytics_top_tickers (
    id bigint NOT NULL,
    run_id character varying(36) NOT NULL,
    ticker_rank integer NOT NULL,
    listing_id bigint NOT NULL,
    ticker character varying(64) NOT NULL,
    traded_quantity bigint DEFAULT 0 NOT NULL,
    traded_notional numeric(19,4) DEFAULT 0 NOT NULL,
    order_count integer DEFAULT 0 NOT NULL,
    transaction_count integer DEFAULT 0 NOT NULL,
    computed_at timestamp without time zone NOT NULL
);


--
-- Name: analytics_top_tickers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.analytics_top_tickers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: analytics_top_tickers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.analytics_top_tickers_id_seq OWNED BY public.analytics_top_tickers.id;


--
-- Name: client_fund_positions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.client_fund_positions (
    id bigint NOT NULL,
    client_id bigint NOT NULL,
    fund_id bigint NOT NULL,
    total_invested numeric(19,2) DEFAULT 0 NOT NULL,
    first_invested_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    last_modified_at timestamp without time zone,
    version bigint DEFAULT 0 NOT NULL,
    CONSTRAINT client_fund_positions_total_invested_check CHECK ((total_invested >= (0)::numeric))
);


--
-- Name: client_fund_positions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.client_fund_positions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: client_fund_positions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.client_fund_positions_id_seq OWNED BY public.client_fund_positions.id;


--
-- Name: client_fund_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.client_fund_transactions (
    id bigint NOT NULL,
    client_id bigint NOT NULL,
    fund_id bigint NOT NULL,
    amount numeric(19,2) NOT NULL,
    is_inflow boolean NOT NULL,
    status character varying(16) DEFAULT 'PENDING'::character varying NOT NULL,
    occurred_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    client_account_number character varying(50) NOT NULL,
    failure_reason character varying(255),
    CONSTRAINT client_fund_transactions_amount_check CHECK ((amount > (0)::numeric))
);


--
-- Name: client_fund_transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.client_fund_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: client_fund_transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.client_fund_transactions_id_seq OWNED BY public.client_fund_transactions.id;


--
-- Name: fund_dividend_distributions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fund_dividend_distributions (
    id bigint NOT NULL,
    fund_id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    payment_date date NOT NULL,
    dividend_per_share numeric(19,8) NOT NULL,
    source_currency character varying(8) NOT NULL,
    holding_quantity integer NOT NULL,
    gross_amount_source numeric(19,8) NOT NULL,
    gross_amount_rsd numeric(19,2) NOT NULL,
    strategy character varying(24) NOT NULL,
    status character varying(24) NOT NULL,
    reinvested_shares integer,
    reinvested_amount_rsd numeric(19,2),
    distributed_amount_rsd numeric(19,2),
    note character varying(255),
    processed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fund_dividend_distributions_dividend_per_share_check CHECK ((dividend_per_share > (0)::numeric)),
    CONSTRAINT fund_dividend_distributions_gross_amount_rsd_check CHECK ((gross_amount_rsd >= (0)::numeric)),
    CONSTRAINT fund_dividend_distributions_gross_amount_source_check CHECK ((gross_amount_source >= (0)::numeric)),
    CONSTRAINT fund_dividend_distributions_holding_quantity_check CHECK ((holding_quantity >= 0))
);


--
-- Name: fund_dividend_distributions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.fund_dividend_distributions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: fund_dividend_distributions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.fund_dividend_distributions_id_seq OWNED BY public.fund_dividend_distributions.id;


--
-- Name: fund_dividend_payouts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fund_dividend_payouts (
    id bigint NOT NULL,
    distribution_id bigint NOT NULL,
    client_id bigint NOT NULL,
    client_account_number character varying(32),
    ownership_ratio numeric(19,8) NOT NULL,
    amount_rsd numeric(19,2) NOT NULL,
    status character varying(24) NOT NULL,
    failure_reason character varying(255),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fund_dividend_payouts_amount_rsd_check CHECK ((amount_rsd >= (0)::numeric))
);


--
-- Name: fund_dividend_payouts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.fund_dividend_payouts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: fund_dividend_payouts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.fund_dividend_payouts_id_seq OWNED BY public.fund_dividend_payouts.id;


--
-- Name: fund_holdings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fund_holdings (
    id bigint NOT NULL,
    fund_id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    quantity integer NOT NULL,
    avg_unit_price numeric(19,4) NOT NULL,
    deleted boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone,
    version bigint DEFAULT 0 NOT NULL,
    CONSTRAINT fund_holdings_avg_unit_price_check CHECK ((avg_unit_price >= (0)::numeric)),
    CONSTRAINT fund_holdings_quantity_check CHECK ((quantity >= 0))
);


--
-- Name: fund_holdings_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.fund_holdings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: fund_holdings_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.fund_holdings_id_seq OWNED BY public.fund_holdings.id;


--
-- Name: fund_value_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fund_value_snapshots (
    id bigint NOT NULL,
    fund_id bigint NOT NULL,
    snapshot_date date NOT NULL,
    liquidity_value numeric(19,2) NOT NULL,
    holdings_value numeric(19,2) NOT NULL,
    total_value numeric(19,2) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT fund_value_snapshots_holdings_value_check CHECK ((holdings_value >= (0)::numeric)),
    CONSTRAINT fund_value_snapshots_liquidity_value_check CHECK ((liquidity_value >= (0)::numeric)),
    CONSTRAINT fund_value_snapshots_total_value_check CHECK ((total_value >= (0)::numeric))
);


--
-- Name: fund_value_snapshots_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.fund_value_snapshots_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: fund_value_snapshots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.fund_value_snapshots_id_seq OWNED BY public.fund_value_snapshots.id;


--
-- Name: interbank_option_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.interbank_option_reservations (
    negotiation_id character varying(64) NOT NULL,
    reservation_id character varying(64) NOT NULL,
    status character varying(32) NOT NULL,
    seller_user_id bigint,
    ticker character varying(32),
    quantity integer,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: interbank_stock_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.interbank_stock_reservations (
    id bigint NOT NULL,
    reservation_id uuid NOT NULL,
    transaction_id_routing integer NOT NULL,
    transaction_id_local character varying(64) NOT NULL,
    portfolio_id bigint NOT NULL,
    ticker character varying(16) NOT NULL,
    quantity integer NOT NULL,
    status character varying(16) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    CONSTRAINT interbank_stock_reservations_quantity_check CHECK ((quantity > 0))
);


--
-- Name: interbank_stock_reservations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.interbank_stock_reservations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: interbank_stock_reservations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.interbank_stock_reservations_id_seq OWNED BY public.interbank_stock_reservations.id;


--
-- Name: investment_funds; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.investment_funds (
    id bigint NOT NULL,
    naziv character varying(64) NOT NULL,
    opis character varying(1024),
    minimum_contribution numeric(19,2) NOT NULL,
    manager_id bigint NOT NULL,
    likvidna_sredstva numeric(19,2) DEFAULT 0 NOT NULL,
    account_number character varying(50) NOT NULL,
    datum_kreiranja date DEFAULT CURRENT_DATE NOT NULL,
    deleted boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    version bigint DEFAULT 0 NOT NULL,
    dividend_strategy character varying(24) DEFAULT 'REINVEST'::character varying NOT NULL,
    CONSTRAINT investment_funds_likvidna_sredstva_check CHECK ((likvidna_sredstva >= (0)::numeric)),
    CONSTRAINT investment_funds_minimum_contribution_check CHECK ((minimum_contribution >= (0)::numeric))
);


--
-- Name: investment_funds_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.investment_funds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: investment_funds_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.investment_funds_id_seq OWNED BY public.investment_funds.id;


--
-- Name: option_contracts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.option_contracts (
    id bigint NOT NULL,
    offer_id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    buyer_id bigint NOT NULL,
    seller_id bigint NOT NULL,
    amount integer NOT NULL,
    price_per_stock numeric(19,2) NOT NULL,
    settlement_date date NOT NULL,
    status character varying(16) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    exercised_at timestamp without time zone,
    version bigint DEFAULT 0 NOT NULL,
    CONSTRAINT option_contracts_amount_check CHECK ((amount >= 1)),
    CONSTRAINT option_contracts_price_per_stock_check CHECK ((price_per_stock > (0)::numeric))
);


--
-- Name: option_contracts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.option_contracts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: option_contracts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.option_contracts_id_seq OWNED BY public.option_contracts.id;


--
-- Name: orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    listing_id bigint NOT NULL,
    order_type character varying(255) NOT NULL,
    quantity integer NOT NULL,
    contract_size integer NOT NULL,
    price_per_unit numeric(19,4) NOT NULL,
    limit_value numeric(19,4),
    stop_value numeric(19,4),
    direction character varying(255) NOT NULL,
    status character varying(255) NOT NULL,
    approved_by bigint,
    is_done boolean DEFAULT false NOT NULL,
    last_modification timestamp without time zone NOT NULL,
    remaining_portions integer NOT NULL,
    after_hours boolean DEFAULT false NOT NULL,
    all_or_none boolean DEFAULT false NOT NULL,
    margin boolean DEFAULT false NOT NULL,
    account_id bigint NOT NULL,
    exchange_closed boolean DEFAULT false NOT NULL,
    reserved_limit_exposure numeric(19,4) DEFAULT 0 NOT NULL,
    purchase_for character varying(20),
    fund_id bigint
);


--
-- Name: orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: orders_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.orders_id_seq OWNED BY public.orders.id;


--
-- Name: otc_contract_expiry_reminders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.otc_contract_expiry_reminders (
    id bigint NOT NULL,
    contract_id bigint NOT NULL,
    reminder_days integer NOT NULL,
    sent_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT otc_contract_expiry_reminders_reminder_days_check CHECK ((reminder_days > 0))
);


--
-- Name: otc_contract_expiry_reminders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.otc_contract_expiry_reminders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: otc_contract_expiry_reminders_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.otc_contract_expiry_reminders_id_seq OWNED BY public.otc_contract_expiry_reminders.id;


--
-- Name: otc_negotiation_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.otc_negotiation_history (
    id bigint NOT NULL,
    offer_id bigint NOT NULL,
    buyer_id bigint NOT NULL,
    seller_id bigint NOT NULL,
    actor_id bigint,
    actor_name character varying(128),
    event_type character varying(32) NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    old_amount integer,
    new_amount integer,
    old_price_per_stock numeric(19,2),
    new_price_per_stock numeric(19,2),
    old_premium numeric(19,2),
    new_premium numeric(19,2),
    old_settlement_date date,
    new_settlement_date date,
    old_status character varying(24),
    new_status character varying(24),
    changed_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: otc_negotiation_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.otc_negotiation_history_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: otc_negotiation_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.otc_negotiation_history_id_seq OWNED BY public.otc_negotiation_history.id;


--
-- Name: otc_offers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.otc_offers (
    id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    buyer_id bigint NOT NULL,
    seller_id bigint NOT NULL,
    amount integer NOT NULL,
    price_per_stock numeric(19,2) NOT NULL,
    premium numeric(19,2) NOT NULL,
    settlement_date date NOT NULL,
    status character varying(24) NOT NULL,
    modified_by character varying(64),
    last_modified timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    version bigint DEFAULT 0 NOT NULL,
    CONSTRAINT otc_offers_amount_check CHECK ((amount >= 1)),
    CONSTRAINT otc_offers_premium_check CHECK ((premium >= (0)::numeric)),
    CONSTRAINT otc_offers_price_per_stock_check CHECK ((price_per_stock > (0)::numeric))
);


--
-- Name: otc_offers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.otc_offers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: otc_offers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.otc_offers_id_seq OWNED BY public.otc_offers.id;


--
-- Name: portfolio; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.portfolio (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    listing_id bigint NOT NULL,
    listing_type character varying(255) NOT NULL,
    quantity integer NOT NULL,
    average_purchase_price numeric(19,4) NOT NULL,
    is_public boolean DEFAULT false NOT NULL,
    public_quantity integer DEFAULT 0 NOT NULL,
    last_modified timestamp without time zone NOT NULL,
    reserved_quantity integer DEFAULT 0 NOT NULL
);


--
-- Name: portfolio_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.portfolio_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: portfolio_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.portfolio_id_seq OWNED BY public.portfolio.id;


--
-- Name: stock_ownership_transfers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_ownership_transfers (
    transfer_id uuid NOT NULL,
    reservation_id uuid NOT NULL,
    correlation_id character varying(64),
    seller_id bigint NOT NULL,
    buyer_id bigint NOT NULL,
    listing_id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    amount integer NOT NULL,
    status character varying(16) DEFAULT 'COMPLETED'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    reversed_at timestamp without time zone
);


--
-- Name: stock_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.stock_reservations (
    reservation_id uuid NOT NULL,
    correlation_id character varying(64),
    seller_id bigint NOT NULL,
    listing_id bigint NOT NULL,
    stock_ticker character varying(16) NOT NULL,
    amount integer NOT NULL,
    status character varying(16) DEFAULT 'HELD'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    released_at timestamp without time zone
);


--
-- Name: tax_charges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tax_charges (
    id bigint NOT NULL,
    sell_transaction_id bigint NOT NULL,
    buy_transaction_id bigint NOT NULL,
    user_id bigint NOT NULL,
    listing_id bigint NOT NULL,
    source_account_id bigint NOT NULL,
    tax_period_start timestamp without time zone NOT NULL,
    tax_period_end timestamp without time zone NOT NULL,
    tax_amount numeric(19,4) NOT NULL,
    tax_amount_rsd numeric(19,4),
    status character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    charged_at timestamp without time zone,
    otc_contract_id bigint
);


--
-- Name: tax_charges_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tax_charges_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tax_charges_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tax_charges_id_seq OWNED BY public.tax_charges.id;


--
-- Name: transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.transactions (
    id bigint NOT NULL,
    order_id bigint NOT NULL,
    quantity integer NOT NULL,
    price_per_unit numeric(19,4) NOT NULL,
    total_price numeric(19,4) NOT NULL,
    commission numeric(19,4) NOT NULL,
    "timestamp" timestamp without time zone NOT NULL
);


--
-- Name: transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.transactions_id_seq OWNED BY public.transactions.id;


--
-- Name: actuary_info id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.actuary_info ALTER COLUMN id SET DEFAULT nextval('public.actuary_info_id_seq'::regclass);


--
-- Name: analytics_client_segments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_client_segments ALTER COLUMN id SET DEFAULT nextval('public.analytics_client_segments_id_seq'::regclass);


--
-- Name: analytics_portfolio_risk id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_portfolio_risk ALTER COLUMN id SET DEFAULT nextval('public.analytics_portfolio_risk_id_seq'::regclass);


--
-- Name: analytics_top_tickers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_top_tickers ALTER COLUMN id SET DEFAULT nextval('public.analytics_top_tickers_id_seq'::regclass);


--
-- Name: client_fund_positions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_positions ALTER COLUMN id SET DEFAULT nextval('public.client_fund_positions_id_seq'::regclass);


--
-- Name: client_fund_transactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_transactions ALTER COLUMN id SET DEFAULT nextval('public.client_fund_transactions_id_seq'::regclass);


--
-- Name: fund_dividend_distributions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_distributions ALTER COLUMN id SET DEFAULT nextval('public.fund_dividend_distributions_id_seq'::regclass);


--
-- Name: fund_dividend_payouts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_payouts ALTER COLUMN id SET DEFAULT nextval('public.fund_dividend_payouts_id_seq'::regclass);


--
-- Name: fund_holdings id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_holdings ALTER COLUMN id SET DEFAULT nextval('public.fund_holdings_id_seq'::regclass);


--
-- Name: fund_value_snapshots id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_value_snapshots ALTER COLUMN id SET DEFAULT nextval('public.fund_value_snapshots_id_seq'::regclass);


--
-- Name: interbank_stock_reservations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interbank_stock_reservations ALTER COLUMN id SET DEFAULT nextval('public.interbank_stock_reservations_id_seq'::regclass);


--
-- Name: investment_funds id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investment_funds ALTER COLUMN id SET DEFAULT nextval('public.investment_funds_id_seq'::regclass);


--
-- Name: option_contracts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.option_contracts ALTER COLUMN id SET DEFAULT nextval('public.option_contracts_id_seq'::regclass);


--
-- Name: orders id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders ALTER COLUMN id SET DEFAULT nextval('public.orders_id_seq'::regclass);


--
-- Name: otc_contract_expiry_reminders id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_contract_expiry_reminders ALTER COLUMN id SET DEFAULT nextval('public.otc_contract_expiry_reminders_id_seq'::regclass);


--
-- Name: otc_negotiation_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_negotiation_history ALTER COLUMN id SET DEFAULT nextval('public.otc_negotiation_history_id_seq'::regclass);


--
-- Name: otc_offers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_offers ALTER COLUMN id SET DEFAULT nextval('public.otc_offers_id_seq'::regclass);


--
-- Name: portfolio id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.portfolio ALTER COLUMN id SET DEFAULT nextval('public.portfolio_id_seq'::regclass);


--
-- Name: tax_charges id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tax_charges ALTER COLUMN id SET DEFAULT nextval('public.tax_charges_id_seq'::regclass);


--
-- Name: transactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions ALTER COLUMN id SET DEFAULT nextval('public.transactions_id_seq'::regclass);


--
-- Name: actuary_info actuary_info_employee_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.actuary_info
    ADD CONSTRAINT actuary_info_employee_id_key UNIQUE (employee_id);


--
-- Name: actuary_info actuary_info_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.actuary_info
    ADD CONSTRAINT actuary_info_pkey PRIMARY KEY (id);


--
-- Name: analytics_client_segments analytics_client_segments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_client_segments
    ADD CONSTRAINT analytics_client_segments_pkey PRIMARY KEY (id);


--
-- Name: analytics_job_runs analytics_job_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_job_runs
    ADD CONSTRAINT analytics_job_runs_pkey PRIMARY KEY (run_id);


--
-- Name: analytics_portfolio_risk analytics_portfolio_risk_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_portfolio_risk
    ADD CONSTRAINT analytics_portfolio_risk_pkey PRIMARY KEY (id);


--
-- Name: analytics_top_tickers analytics_top_tickers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_top_tickers
    ADD CONSTRAINT analytics_top_tickers_pkey PRIMARY KEY (id);


--
-- Name: client_fund_positions client_fund_positions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_positions
    ADD CONSTRAINT client_fund_positions_pkey PRIMARY KEY (id);


--
-- Name: client_fund_transactions client_fund_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_transactions
    ADD CONSTRAINT client_fund_transactions_pkey PRIMARY KEY (id);


--
-- Name: fund_dividend_distributions fund_dividend_distributions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_distributions
    ADD CONSTRAINT fund_dividend_distributions_pkey PRIMARY KEY (id);


--
-- Name: fund_dividend_payouts fund_dividend_payouts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_payouts
    ADD CONSTRAINT fund_dividend_payouts_pkey PRIMARY KEY (id);


--
-- Name: fund_holdings fund_holdings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_holdings
    ADD CONSTRAINT fund_holdings_pkey PRIMARY KEY (id);


--
-- Name: fund_value_snapshots fund_value_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_value_snapshots
    ADD CONSTRAINT fund_value_snapshots_pkey PRIMARY KEY (id);


--
-- Name: interbank_option_reservations interbank_option_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interbank_option_reservations
    ADD CONSTRAINT interbank_option_reservations_pkey PRIMARY KEY (negotiation_id);


--
-- Name: interbank_stock_reservations interbank_stock_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interbank_stock_reservations
    ADD CONSTRAINT interbank_stock_reservations_pkey PRIMARY KEY (id);


--
-- Name: interbank_stock_reservations interbank_stock_reservations_reservation_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.interbank_stock_reservations
    ADD CONSTRAINT interbank_stock_reservations_reservation_id_key UNIQUE (reservation_id);


--
-- Name: investment_funds investment_funds_account_number_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investment_funds
    ADD CONSTRAINT investment_funds_account_number_key UNIQUE (account_number);


--
-- Name: investment_funds investment_funds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.investment_funds
    ADD CONSTRAINT investment_funds_pkey PRIMARY KEY (id);


--
-- Name: option_contracts option_contracts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.option_contracts
    ADD CONSTRAINT option_contracts_pkey PRIMARY KEY (id);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: otc_contract_expiry_reminders otc_contract_expiry_reminders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_contract_expiry_reminders
    ADD CONSTRAINT otc_contract_expiry_reminders_pkey PRIMARY KEY (id);


--
-- Name: otc_negotiation_history otc_negotiation_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_negotiation_history
    ADD CONSTRAINT otc_negotiation_history_pkey PRIMARY KEY (id);


--
-- Name: otc_offers otc_offers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_offers
    ADD CONSTRAINT otc_offers_pkey PRIMARY KEY (id);


--
-- Name: portfolio portfolio_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.portfolio
    ADD CONSTRAINT portfolio_pkey PRIMARY KEY (id);


--
-- Name: stock_ownership_transfers stock_ownership_transfers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_ownership_transfers
    ADD CONSTRAINT stock_ownership_transfers_pkey PRIMARY KEY (transfer_id);


--
-- Name: stock_reservations stock_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.stock_reservations
    ADD CONSTRAINT stock_reservations_pkey PRIMARY KEY (reservation_id);


--
-- Name: tax_charges tax_charges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tax_charges
    ADD CONSTRAINT tax_charges_pkey PRIMARY KEY (id);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: client_fund_positions uk_client_fund_position_client_fund; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_positions
    ADD CONSTRAINT uk_client_fund_position_client_fund UNIQUE (client_id, fund_id);


--
-- Name: fund_dividend_distributions uk_fund_dividend_distribution; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_distributions
    ADD CONSTRAINT uk_fund_dividend_distribution UNIQUE (fund_id, stock_ticker, payment_date);


--
-- Name: fund_dividend_payouts uk_fund_dividend_payout_distribution_client; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_payouts
    ADD CONSTRAINT uk_fund_dividend_payout_distribution_client UNIQUE (distribution_id, client_id);


--
-- Name: fund_holdings uk_fund_holdings_fund_ticker; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_holdings
    ADD CONSTRAINT uk_fund_holdings_fund_ticker UNIQUE (fund_id, stock_ticker);


--
-- Name: fund_value_snapshots uk_fund_value_snapshot_fund_date; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_value_snapshots
    ADD CONSTRAINT uk_fund_value_snapshot_fund_date UNIQUE (fund_id, snapshot_date);


--
-- Name: otc_contract_expiry_reminders uk_otc_contract_expiry_reminder; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_contract_expiry_reminders
    ADD CONSTRAINT uk_otc_contract_expiry_reminder UNIQUE (contract_id, reminder_days);


--
-- Name: portfolio ukigugv4tyy7sieoc8gk2i9clem; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.portfolio
    ADD CONSTRAINT ukigugv4tyy7sieoc8gk2i9clem UNIQUE (user_id, listing_id);


--
-- Name: portfolio uq_portfolio_user_listing; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.portfolio
    ADD CONSTRAINT uq_portfolio_user_listing UNIQUE (user_id, listing_id);


--
-- Name: idx_actuary_info_employee_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_actuary_info_employee_id ON public.actuary_info USING btree (employee_id);


--
-- Name: idx_analytics_client_segments_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_client_segments_run ON public.analytics_client_segments USING btree (run_id);


--
-- Name: idx_analytics_client_segments_run_risk; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_client_segments_run_risk ON public.analytics_client_segments USING btree (run_id, risk_score DESC, user_id);


--
-- Name: idx_analytics_job_runs_status_completed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_job_runs_status_completed ON public.analytics_job_runs USING btree (status, completed_at DESC);


--
-- Name: idx_analytics_portfolio_risk_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_portfolio_risk_run ON public.analytics_portfolio_risk USING btree (run_id);


--
-- Name: idx_analytics_portfolio_risk_run_score; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_portfolio_risk_run_score ON public.analytics_portfolio_risk USING btree (run_id, risk_score DESC, user_id);


--
-- Name: idx_analytics_top_tickers_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_analytics_top_tickers_run ON public.analytics_top_tickers USING btree (run_id, ticker_rank);


--
-- Name: idx_cfp_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cfp_client_id ON public.client_fund_positions USING btree (client_id);


--
-- Name: idx_cfp_fund_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cfp_fund_id ON public.client_fund_positions USING btree (fund_id);


--
-- Name: idx_cft_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cft_client_id ON public.client_fund_transactions USING btree (client_id);


--
-- Name: idx_cft_fund_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cft_fund_id ON public.client_fund_transactions USING btree (fund_id);


--
-- Name: idx_cft_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_cft_status ON public.client_fund_transactions USING btree (status);


--
-- Name: idx_fund_dividend_distribution_fund_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_dividend_distribution_fund_id ON public.fund_dividend_distributions USING btree (fund_id);


--
-- Name: idx_fund_dividend_distribution_payment_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_dividend_distribution_payment_date ON public.fund_dividend_distributions USING btree (payment_date);


--
-- Name: idx_fund_dividend_payout_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_dividend_payout_client_id ON public.fund_dividend_payouts USING btree (client_id);


--
-- Name: idx_fund_dividend_payout_distribution_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_dividend_payout_distribution_id ON public.fund_dividend_payouts USING btree (distribution_id);


--
-- Name: idx_fund_holdings_fund_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_holdings_fund_id ON public.fund_holdings USING btree (fund_id);


--
-- Name: idx_fund_holdings_stock_ticker; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_holdings_stock_ticker ON public.fund_holdings USING btree (stock_ticker);


--
-- Name: idx_fund_value_snapshots_fund_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_value_snapshots_fund_id ON public.fund_value_snapshots USING btree (fund_id);


--
-- Name: idx_fund_value_snapshots_snapshot_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fund_value_snapshots_snapshot_date ON public.fund_value_snapshots USING btree (snapshot_date);


--
-- Name: idx_interbank_option_reservations_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_interbank_option_reservations_status ON public.interbank_option_reservations USING btree (status);


--
-- Name: idx_interbank_stock_reservations_portfolio_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_interbank_stock_reservations_portfolio_status ON public.interbank_stock_reservations USING btree (portfolio_id, status);


--
-- Name: idx_interbank_stock_reservations_tx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_interbank_stock_reservations_tx ON public.interbank_stock_reservations USING btree (transaction_id_routing, transaction_id_local);


--
-- Name: idx_investment_funds_account_number; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_investment_funds_account_number ON public.investment_funds USING btree (account_number);


--
-- Name: idx_investment_funds_manager_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_investment_funds_manager_id ON public.investment_funds USING btree (manager_id);


--
-- Name: idx_option_contracts_buyer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_option_contracts_buyer_id ON public.option_contracts USING btree (buyer_id);


--
-- Name: idx_option_contracts_seller_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_option_contracts_seller_id ON public.option_contracts USING btree (seller_id);


--
-- Name: idx_option_contracts_settlement_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_option_contracts_settlement_date ON public.option_contracts USING btree (settlement_date);


--
-- Name: idx_option_contracts_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_option_contracts_status ON public.option_contracts USING btree (status);


--
-- Name: idx_orders_listing_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_listing_id ON public.orders USING btree (listing_id);


--
-- Name: idx_orders_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_status ON public.orders USING btree (status);


--
-- Name: idx_orders_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_user_id ON public.orders USING btree (user_id);


--
-- Name: idx_otc_expiry_reminders_sent_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_expiry_reminders_sent_at ON public.otc_contract_expiry_reminders USING btree (sent_at);


--
-- Name: idx_otc_history_buyer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_history_buyer_id ON public.otc_negotiation_history USING btree (buyer_id);


--
-- Name: idx_otc_history_changed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_history_changed_at ON public.otc_negotiation_history USING btree (changed_at);


--
-- Name: idx_otc_history_new_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_history_new_status ON public.otc_negotiation_history USING btree (new_status);


--
-- Name: idx_otc_history_offer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_history_offer_id ON public.otc_negotiation_history USING btree (offer_id);


--
-- Name: idx_otc_history_seller_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_history_seller_id ON public.otc_negotiation_history USING btree (seller_id);


--
-- Name: idx_otc_offers_buyer_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_offers_buyer_id ON public.otc_offers USING btree (buyer_id);


--
-- Name: idx_otc_offers_seller_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_offers_seller_id ON public.otc_offers USING btree (seller_id);


--
-- Name: idx_otc_offers_settlement_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_offers_settlement_date ON public.otc_offers USING btree (settlement_date);


--
-- Name: idx_otc_offers_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_otc_offers_status ON public.otc_offers USING btree (status);


--
-- Name: idx_portfolio_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_portfolio_user_id ON public.portfolio USING btree (user_id);


--
-- Name: idx_tax_charges_period; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tax_charges_period ON public.tax_charges USING btree (tax_period_start, tax_period_end);


--
-- Name: idx_transactions_order_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_transactions_order_id ON public.transactions USING btree (order_id);


--
-- Name: uk_analytics_client_segments_run_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_analytics_client_segments_run_user ON public.analytics_client_segments USING btree (run_id, user_id);


--
-- Name: uk_analytics_portfolio_risk_run_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_analytics_portfolio_risk_run_user ON public.analytics_portfolio_risk USING btree (run_id, user_id);


--
-- Name: uk_analytics_top_tickers_run_rank; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_analytics_top_tickers_run_rank ON public.analytics_top_tickers USING btree (run_id, ticker_rank);


--
-- Name: uk_tax_charges_otc_contract; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_tax_charges_otc_contract ON public.tax_charges USING btree (otc_contract_id) WHERE (otc_contract_id IS NOT NULL);


--
-- Name: uk_tax_charges_sell_buy; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uk_tax_charges_sell_buy ON public.tax_charges USING btree (sell_transaction_id, buy_transaction_id);


--
-- Name: client_fund_positions client_fund_positions_fund_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_positions
    ADD CONSTRAINT client_fund_positions_fund_id_fkey FOREIGN KEY (fund_id) REFERENCES public.investment_funds(id);


--
-- Name: client_fund_transactions client_fund_transactions_fund_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.client_fund_transactions
    ADD CONSTRAINT client_fund_transactions_fund_id_fkey FOREIGN KEY (fund_id) REFERENCES public.investment_funds(id);


--
-- Name: fund_dividend_distributions fund_dividend_distributions_fund_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_distributions
    ADD CONSTRAINT fund_dividend_distributions_fund_id_fkey FOREIGN KEY (fund_id) REFERENCES public.investment_funds(id);


--
-- Name: fund_dividend_payouts fund_dividend_payouts_distribution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_dividend_payouts
    ADD CONSTRAINT fund_dividend_payouts_distribution_id_fkey FOREIGN KEY (distribution_id) REFERENCES public.fund_dividend_distributions(id);


--
-- Name: fund_holdings fund_holdings_fund_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_holdings
    ADD CONSTRAINT fund_holdings_fund_id_fkey FOREIGN KEY (fund_id) REFERENCES public.investment_funds(id);


--
-- Name: fund_value_snapshots fund_value_snapshots_fund_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fund_value_snapshots
    ADD CONSTRAINT fund_value_snapshots_fund_id_fkey FOREIGN KEY (fund_id) REFERENCES public.investment_funds(id);


--
-- Name: option_contracts option_contracts_offer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.option_contracts
    ADD CONSTRAINT option_contracts_offer_id_fkey FOREIGN KEY (offer_id) REFERENCES public.otc_offers(id);


--
-- Name: otc_contract_expiry_reminders otc_contract_expiry_reminders_contract_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_contract_expiry_reminders
    ADD CONSTRAINT otc_contract_expiry_reminders_contract_id_fkey FOREIGN KEY (contract_id) REFERENCES public.option_contracts(id);


--
-- Name: otc_negotiation_history otc_negotiation_history_offer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.otc_negotiation_history
    ADD CONSTRAINT otc_negotiation_history_offer_id_fkey FOREIGN KEY (offer_id) REFERENCES public.otc_offers(id);


--
-- Name: transactions transactions_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id);


--
-- PostgreSQL database dump complete
--


