--liquibase formatted sql
-- changeset interbank:32-002-sequences

-- ============================================================================
-- PR_32 Phase 1: sekvence za negotiation/contract ID generaciju.
--
-- Negotiation ID format (po spec §4): "{routing}-{seq}", npr. "111-42".
-- Contract ID format (po spec §4): "{routing}-C-{seq}", npr. "111-C-7".
-- ============================================================================

CREATE SEQUENCE interbank_negotiation_seq START 1 INCREMENT 1;
CREATE SEQUENCE interbank_contract_seq    START 1 INCREMENT 1;
