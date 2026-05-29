#!/usr/bin/env bash
# Sveobuhvatni test SVIH endpointa banking-core-service-go.
#
# Dva tipa provere:
#   expect NAME CODE ...  -> tacan ocekivani HTTP status (happy-path tokovi)
#   reach  NAME ...       -> ruta+rola+handler rade (status NIJE 401/403/405/500/000;
#                            200/201/204/400/404/409 su OK jer endpoint odgovara smisleno)
#
# Generise HS256 JWT-ove lokalno (isti JWT_SECRET), salje preko
# `docker exec banka_banking_core_service curl http://localhost:8084`, DB preko psql.
#
# Pokretanje (iz root-a repo-a):  bash banking-core-service-go/scripts/test-endpoints.sh
# Napomene: koristi i menja dev-seed podatke (Marko id=1, Ana id=2, Aleksandar id=8).
#           Account-creation i card-request zavise od user-service (oznaceni [DEP]).

BC="${BANKING_CORE_CONTAINER:-banka_banking_core_service}"
PG="${POSTGRES_CONTAINER:-banka_postgres}"
DBN="${BANKING_CORE_DB_NAME:-banking_core}"
BASE="${BASE_URL:-http://localhost:8084}"

MARKO="1110001100000000111"   # owner 1
MARKO2="1110001100000000211"  # owner 1 (kreira skripta) — za same-owner transfer
ANA="1110001200000000111"     # owner 2
STAMP="$(date +%s)$RANDOM"

SECRET="${JWT_SECRET:-}"
for f in ./setup/.env ../setup/.env ../../setup/.env; do
  [ -z "$SECRET" ] && [ -f "$f" ] && SECRET=$(grep -E '^JWT_SECRET=' "$f" | head -1 | cut -d= -f2- | tr -d '"\r')
done
[ -z "$SECRET" ] && { echo "GRESKA: postavi JWT_SECRET ili setup/.env"; exit 1; }

PASS=0; FAIL=0; INFO=0
ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1"; FAIL=$((FAIL+1)); }
inf()  { echo "  [INFO] $1"; INFO=$((INFO+1)); }

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
mint() { local h p s; h=$(printf '%s' '{"alg":"HS256","typ":"JWT"}'|b64url); p=$(printf '%s' "$1"|b64url); s=$(printf '%s' "$h.$p"|openssl dgst -sha256 -hmac "$SECRET" -binary|b64url); printf '%s.%s.%s' "$h" "$p" "$s"; }
EXP=$(( $(date +%s) + 3600 ))
T_CLIENT=$(mint "{\"id\":1,\"roles\":[\"CLIENT_BASIC\"],\"exp\":$EXP}")
T_CLIENT8=$(mint "{\"id\":8,\"roles\":[\"CLIENT_BASIC\"],\"exp\":$EXP}")
T_EMP=$(mint "{\"id\":100,\"roles\":[\"BASIC\"],\"exp\":$EXP}")
T_ADMIN=$(mint "{\"id\":200,\"roles\":[\"ADMIN\"],\"exp\":$EXP}")
T_SVC=$(mint "{\"sub\":\"svc\",\"roles\":[\"SERVICE\"],\"exp\":$EXP}")

call() { # METHOD PATH TOKEN(-|jwt) BODY(-|json)
  local m="$1" p="$2" tok="$3" body="$4"
  local a=(-s -m 20 -w $'\n%{http_code}' -X "$m")
  [ "$tok" != "-" ] && a+=(-H "Authorization: Bearer $tok")
  [ "$body" != "-" ] && a+=(-H "Content-Type: application/json" -d "$body")
  docker exec "$BC" curl "${a[@]}" "$BASE$p"
}
status() { tail -n1 <<<"$1"; }
bodyof() { sed '$d' <<<"$1"; }
psqlv()  { docker exec "$PG" psql -U postgres -d "$DBN" -t -A -c "$1" 2>/dev/null | tr -d '[:space:]'; }

expect() { # name code method path token body
  local n="$1" e="$2"; shift 2
  local r; r=$(call "$1" "$2" "$3" "${4:--}"); local c; c=$(status "$r")
  [ "$c" = "$e" ] && ok "$n ($c)" || bad "$n: ocekivano $e, dobijeno $c  $(bodyof "$r"|head -c160)"
}
reach() { # name method path token body
  local n="$1"; shift
  local r; r=$(call "$1" "$2" "$3" "${4:--}"); local c; c=$(status "$r")
  case "$c" in 200|201|202|204|400|404|409|422) ok "$n (reach $c)";; *) bad "$n: los status $c  $(bodyof "$r"|head -c160)";; esac
}
dep() { # name method path token body  (zavisi od eksternog servisa — INFO, ne FAIL)
  local n="$1"; shift
  local r; r=$(call "$1" "$2" "$3" "${4:--}"); local c; c=$(status "$r")
  case "$c" in 200|201|202|204) ok "$n ($c)";; *) inf "$n: status $c (verovatno zavisi od user-service)";; esac
}

mk_verified() { # operationType relatedEntityId -> sessionId (postavljen na VERIFIED)
  local r sid; r=$(call POST /verification/generate "$T_CLIENT" "{\"clientId\":1,\"operationType\":\"$1\",\"relatedEntityId\":\"$2\",\"clientEmail\":\"marko.markovic@banka.com\"}")
  sid=$(bodyof "$r" | sed -n 's/.*"sessionId":\([0-9]*\).*/\1/p')
  [ -n "$sid" ] && psqlv "UPDATE verification_sessions SET status='VERIFIED' WHERE id=$sid" >/dev/null
  echo "$sid"
}

echo "=== Sveobuhvatni endpoint test ==="; echo "container=$BC base=$BASE"; echo

echo "[0] Fixtures"
psqlv "INSERT INTO account_table (version,account_type,broj_racuna,ime_vlasnika_racuna,prezime_vlasnika_racuna,email,username,naziv_racuna,vlasnik,zaposlen,stanje,raspolozivo_stanje,datum_i_vreme_kreiranja,datum_isteka,currency_id,status,dnevni_limit,mesecni_limit,dnevna_potrosnja,mesecna_potrosnja,company_id,account_concrete,odrzavanje_racuna,account_ownership_type) SELECT 0,'CHECKING','$MARKO2','Marko','Markovic','marko.markovic@banka.com','marko.markovic','Tekuci 2',1,1,100000.00,100000.00,NOW(),'2031-03-25',c.id,'ACTIVE',250000.00,1000000.00,0,0,NULL,'STANDARDNI',255.00,NULL FROM currency_table c WHERE c.oznaka='RSD' ON CONFLICT (broj_racuna) DO NOTHING" >/dev/null
MARKO_ID=$(psqlv "SELECT id FROM account_table WHERE broj_racuna='$MARKO'")
echo "  Marko account id=$MARKO_ID; 2. RSD racun spreman"

echo "[A] Actuator (open)"
for p in liveness readiness; do expect "GET /actuator/health/$p" 200 GET "/actuator/health/$p" -; done
expect "GET /actuator/info" 200 GET /actuator/info -
reach  "GET /actuator/prometheus" GET /actuator/prometheus -

echo "[B] Role-gating (negativni)"
expect "401 bez tokena" 401 GET /accounts/client/accounts -
expect "403 pogresna rola (cards/auto kao CLIENT)" 403 POST /api/cards/auto "$T_CLIENT" '{}'
expect "403 by-client kao CLIENT (treba BASIC)" 403 GET "/transactions/by-client?id=1" "$T_CLIENT"

echo "[C] Currencies (CLIENT_BASIC/BASIC)"
expect "getAll" 200 GET /accounts/api/currencies/getAll "$T_CLIENT"
expect "getAllPage" 200 GET "/accounts/api/currencies/getAllPage?page=0&size=10" "$T_CLIENT"
reach  "by-query ?code=RSD" GET "/accounts/api/currencies?code=RSD" "$T_CLIENT"
reach  "by-path /RSD" GET /accounts/api/currencies/RSD "$T_CLIENT"

echo "[D] Employee accounts (BASIC)"
expect "search /accounts/employee/accounts" 200 GET "/accounts/employee/accounts?page=0&size=10" "$T_EMP"
expect "bank list" 200 GET /accounts/employee/accounts/bank "$T_EMP"
expect "bank/RSD" 200 GET /accounts/employee/accounts/bank/RSD "$T_EMP"
expect "client/1" 200 GET /accounts/employee/accounts/client/1 "$T_EMP"
expect "details {num}" 200 GET "/accounts/employee/accounts/$MARKO" "$T_EMP"
reach  "PUT status" PUT "/accounts/employee/accounts/$MARKO/status" "$T_EMP" '{"status":"ACTIVE"}'
reach  "GET companies/1" GET /accounts/employee/companies/1 "$T_EMP"
reach  "PUT companies/1" PUT /accounts/employee/companies/1 "$T_EMP" '{"naziv":"Test"}'
dep    "POST accounts/checking [DEP]" POST /accounts/employee/accounts/checking "$T_EMP" '{"nazivRacuna":"T","vrstaRacuna":"STANDARDNI","idVlasnika":1}'
dep    "POST accounts/fx [DEP]" POST /accounts/employee/accounts/fx "$T_EMP" '{"nazivRacuna":"T","currencyCode":"EUR","tipRacuna":"PERSONAL","idVlasnika":1}'

echo "[E] Client accounts (CLIENT_BASIC/AGENT)"
expect "client/accounts" 200 GET /accounts/client/accounts "$T_CLIENT"
expect "by-id {id}" 200 GET "/accounts/client/accounts/$MARKO_ID" "$T_CLIENT"
expect "by-id/cards" 200 GET "/accounts/client/accounts/$MARKO_ID/cards" "$T_CLIENT"
expect "by-number /api/accounts/{num}" 200 GET "/accounts/client/api/accounts/$MARKO" "$T_CLIENT"
reach  "PUT name by-number" PUT "/accounts/client/api/accounts/$MARKO/name" "$T_CLIENT" '{"naziv":"Tekuci","newName":"Tekuci"}'
reach  "PATCH name by-id" PATCH "/accounts/client/accounts/$MARKO_ID/name" "$T_CLIENT" '{"naziv":"Tekuci","newName":"Tekuci"}'
reach  "PATCH limits by-id" PATCH "/accounts/client/accounts/$MARKO_ID/limits" "$T_CLIENT" '{"dnevniLimit":250000,"mesecniLimit":1000000}'
reach  "PUT limits by-number" PUT "/accounts/client/api/accounts/$MARKO/limits" "$T_CLIENT" '{"dnevniLimit":250000,"mesecniLimit":1000000}'

echo "[F] Internal accounts (SERVICE)"
expect "credit" 200 POST /internal/accounts/credit "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":1000,\"clientId\":1}"
expect "debit" 200 POST /internal/accounts/debit "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":1000,\"clientId\":1}"
expect "creditBank" 200 POST /internal/accounts/creditBank "$T_SVC" '{"currencyCode":"RSD","amount":100}'
expect "debitBank" 200 POST /internal/accounts/debitBank "$T_SVC" '{"currencyCode":"RSD","amount":100}'
expect "exchange/buy" 200 POST /internal/accounts/exchange/buy "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":10,\"clientId\":1}"
expect "exchange/sell" 200 POST /internal/accounts/exchange/sell "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":10,\"clientId\":1}"
expect "transaction (diff owner)" 200 POST /internal/accounts/transaction "$T_SVC" "{\"fromAccountNumber\":\"$MARKO\",\"toAccountNumber\":\"$ANA\",\"fromAmount\":100,\"toAmount\":100,\"commission\":0,\"clientId\":1}"
expect "transfer (same owner)" 200 POST /internal/accounts/transfer "$T_SVC" "{\"fromAccountNumber\":\"$MARKO\",\"toAccountNumber\":\"$MARKO2\",\"fromAmount\":100,\"toAmount\":100,\"commission\":0,\"clientId\":1}"
expect "transactionFromBank" 200 POST /internal/accounts/transactionFromBank "$T_SVC" "{\"toAccountNumber\":\"$MARKO\",\"amount\":50}"
reach  "system (createSystemAccount)" POST /internal/accounts/system "$T_SVC" '{"accountNumber":"9990001112223334","ownerId":-999,"currencyCode":"RSD","accountConcrete":"STANDARDNI","displayName":"Test sistemski"}'
expect "state/RSD" 200 GET /internal/accounts/state/RSD "$T_SVC"
reach  "info" GET /internal/accounts/info "$T_SVC"
expect "bank/RSD" 200 GET /internal/accounts/bank/RSD "$T_SVC"
expect "id/{id}/details" 200 GET "/internal/accounts/id/$MARKO_ID/details" "$T_SVC"
expect "{num}/details" 200 GET "/internal/accounts/$MARKO/details" "$T_SVC"
reach  "employee/accounts/client/1" GET /employee/accounts/client/1 "$T_EMP"
expect "internal/default/1 (open)" 200 GET /accounts/internal/default/1 -

echo "[G] Fund rezervacije + interni transfer (SERVICE)"
r=$(call POST /transactions/internal/reserve-funds "$T_SVC" '{"ownerId":1,"amount":500}'); [ "$(status "$r")" = 200 ] && ok "reserve-funds (200)" || bad "reserve-funds $(status "$r")"
RID=$(bodyof "$r"|sed -n 's/.*"reservationId":"\([^"]*\)".*/\1/p')
[ -n "$RID" ] && expect "commit reservation" 200 POST "/transactions/internal/reservations/$RID/commit" "$T_SVC" || bad "nema reservationId"
r=$(call POST /transactions/internal/reserve-funds "$T_SVC" '{"ownerId":1,"amount":300}'); RID2=$(bodyof "$r"|sed -n 's/.*"reservationId":"\([^"]*\)".*/\1/p')
[ -n "$RID2" ] && expect "release reservation (DELETE)" 200 DELETE "/transactions/internal/reservations/$RID2" "$T_SVC" || bad "nema reservationId2"
r=$(call POST /transactions/internal/transfer "$T_SVC" "{\"fromAccountNumber\":\"$MARKO\",\"toAccountNumber\":\"$MARKO2\",\"amount\":100}"); [ "$(status "$r")" = 200 ] && ok "internal/transfer (200)" || bad "internal/transfer $(status "$r")"
TID=$(bodyof "$r"|sed -n 's/.*"transferId":"\([^"]*\)".*/\1/p')
[ -n "$TID" ] && expect "reverse transfer" 200 POST "/transactions/internal/transfers/$TID/reverse" "$T_SVC" || bad "nema transferId"

echo "[H] Interbank (SERVICE)"
# interbank_reservations.account_number je VARCHAR(18) (isto kao Java) -> koristi 18-cifren bank racun
IBACC="111000110000000312"
r=$(call POST /internal/interbank/reserve-monas "$T_SVC" "{\"accountNum\":\"$IBACC\",\"currency\":\"RSD\",\"amount\":100,\"transactionIdRouting\":1,\"transactionIdLocal\":\"tx-$STAMP\"}")
[ "$(status "$r")" = 200 ] && ok "reserve-monas (200)" || bad "reserve-monas $(status "$r")  $(bodyof "$r"|head -c120)"
MRID=$(bodyof "$r"|sed -n 's/.*"reservationId":"\([^"]*\)".*/\1/p')
[ -n "$MRID" ] && reach "commit-monas" POST "/internal/interbank/reservations/$MRID/commit-monas" "$T_SVC"
r=$(call POST /internal/interbank/reserve-monas "$T_SVC" "{\"accountNum\":\"$IBACC\",\"currency\":\"RSD\",\"amount\":100,\"transactionIdRouting\":1,\"transactionIdLocal\":\"tx2-$STAMP\"}"); MRID2=$(bodyof "$r"|sed -n 's/.*"reservationId":"\([^"]*\)".*/\1/p')
[ -n "$MRID2" ] && reach "release-monas (DELETE)" DELETE "/internal/interbank/reservations/$MRID2" "$T_SVC"
expect "account-by-owner" 200 GET "/internal/interbank/account-by-owner?ownerId=1&currency=RSD" "$T_SVC"
expect "account-resolve" 200 GET "/internal/interbank/account-resolve?num=$MARKO" "$T_SVC"

echo "[I] Payment + transfer (CLIENT_BASIC, VERIFIED sesija)"
SID_PAY=$(mk_verified PAYMENT "pay-$STAMP")
r=$(call POST /transactions/payment "$T_CLIENT" "{\"fromAccountNumber\":\"$MARKO\",\"toAccountNumber\":\"$ANA\",\"amount\":100,\"recipientName\":\"Ana Anic\",\"paymentCode\":\"289\",\"paymentPurpose\":\"test\",\"verificationSessionId\":$SID_PAY}")
c=$(status "$r"); echo "$(bodyof "$r")" | grep -q COMPLETED && [ "$c" = 200 ] && ok "payment -> 200 COMPLETED" || bad "payment status=$c body=$(bodyof "$r"|head -c160)"
expect "GET /transactions/api/payments" 200 GET "/transactions/api/payments?page=0&size=10" "$T_CLIENT"
expect "by-this-client" 200 GET "/transactions/by-this-client?page=0&size=10" "$T_CLIENT"
expect "by-this-sender-client" 200 GET "/transactions/by-this-sender-client?page=0&size=10" "$T_CLIENT"
expect "by-this-recipient-client" 200 GET "/transactions/by-this-recipient-client?page=0&size=10" "$T_CLIENT"
expect "by-client (BASIC)" 200 GET "/transactions/by-client?id=1&page=0&size=10" "$T_EMP"
expect "by-sender-client (BASIC)" 200 GET "/transactions/by-sender-client?id=1&page=0&size=10" "$T_EMP"
expect "by-recipient-client (BASIC)" 200 GET "/transactions/by-recipient-client?id=2&page=0&size=10" "$T_EMP"
expect "transactions/accounts/{num}" 200 GET "/transactions/accounts/$MARKO?page=0&size=10" "$T_CLIENT"
expect "transactions/employee/accounts/{num}" 200 GET "/transactions/employee/accounts/$MARKO?page=0&size=10" "$T_EMP"
SID_TR=$(mk_verified TRANSFER "tr-$STAMP")
reach "POST /transfers" POST /transfers "$T_CLIENT" "{\"fromAccountNumber\":\"$MARKO\",\"toAccountNumber\":\"$MARKO2\",\"amount\":100,\"verificationSessionId\":$SID_TR}"
expect "GET /transfers" 200 GET "/transfers?clientId=1&page=0&size=10" "$T_CLIENT"
expect "GET /transfers/accounts/{num}" 200 GET "/transfers/accounts/$MARKO?page=0&size=10" "$T_CLIENT"
reach "GET /transfers/{orderNumber}" GET "/transfers/BANKA1-1" "$T_CLIENT"

echo "[J] Payment recipients (CLIENT_BASIC)"
r=$(call POST /payment-recipients "$T_CLIENT" "{\"naziv\":\"Primalac $STAMP\",\"brojRacuna\":\"$ANA\"}"); [ "$(status "$r")" = 201 ] && ok "create recipient (201)" || bad "create recipient $(status "$r")"
PRID=$(bodyof "$r"|sed -n 's/.*"id":\([0-9]*\).*/\1/p')
expect "list recipients" 200 GET "/payment-recipients?page=0&size=10" "$T_CLIENT"
[ -n "$PRID" ] && expect "update recipient" 200 PUT "/payment-recipients/$PRID" "$T_CLIENT" "{\"naziv\":\"Primalac $STAMP b\",\"brojRacuna\":\"$ANA\"}"
[ -n "$PRID" ] && reach "delete recipient" DELETE "/payment-recipients/$PRID" "$T_CLIENT"

echo "[K] Margin racuni + transakcije (authenticated)"
MUID=$(( (RANDOM*RANDOM % 800000) + 100000 ))
expect "createMarginAccount (novi user)" 201 POST /accounts/createMarginAccount "$T_CLIENT" "{\"employeeId\":100,\"userId\":$MUID,\"initialMargin\":1000,\"maintenanceMargin\":500,\"bankParticipation\":0.5}"
expect "getMarginUser (novi)" 200 GET "/accounts/getMarginUser/$MUID" "$T_CLIENT"
CMID=$(( (RANDOM*RANDOM % 800000) + 100000 ))
expect "company/createMarginAccount" 201 POST /accounts/company/createMarginAccount "$T_CLIENT" "{\"employeeId\":100,\"companyId\":$CMID,\"initialMargin\":1000,\"maintenanceMargin\":500,\"bankParticipation\":0.5}"
expect "company/getMarginCompany" 200 GET "/accounts/company/getMarginCompany/$CMID" "$T_CLIENT"
# Margin za Marka (user 1) za add/withdraw/history; idempotentno
call POST /accounts/createMarginAccount "$T_CLIENT" '{"employeeId":100,"userId":1,"initialMargin":2000,"maintenanceMargin":500,"bankParticipation":0.5}' >/dev/null
r=$(call GET /accounts/getMarginUser/1 "$T_CLIENT" -); MARGIN_ACC=$(bodyof "$r"|sed -n 's/.*"accountNumber":"\([^"]*\)".*/\1/p')
echo "  margin acc (user 1) = ${MARGIN_ACC:-<none>}"
reach "addToMargin/1" POST /transactions/addToMargin/1 "$T_CLIENT" "{\"amount\":100,\"fromAccountNumber\":\"$MARKO\"}"
reach "withdrawFromMargin/1" POST /transactions/withdrawFromMargin/1 "$T_CLIENT" "{\"amount\":50,\"fromAccountNumber\":\"$MARKO\"}"
reach "stockBuyMarginTransaction" POST /transactions/stockBuyMarginTransaction "$T_CLIENT" '{"userId":1,"amount":50}'
reach "stockSellMarginTransaction" POST /transactions/stockSellMarginTransaction "$T_CLIENT" '{"userId":1,"amount":50}'
[ -n "$MARGIN_ACC" ] && expect "getAllMarginTransactions/{acc}" 200 GET "/transactions/getAllMarginTransactions/$MARGIN_ACC" "$T_CLIENT" || inf "preskocen margin history (nema accountNumber)"

echo "[L] Kartice (/api/cards)"
r=$(call POST /api/cards/auto "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"clientId\":1}"); c=$(status "$r")
case "$c" in 200|201) ok "auto-create card ($c)";; *) bad "auto-create card $c  $(bodyof "$r"|head -c160)";; esac
CARD_ID=$(bodyof "$r"|sed -n 's/.*"id":\([0-9]*\).*/\1/p')
[ -z "$CARD_ID" ] && CARD_ID=$(psqlv "SELECT id FROM cards WHERE account_number='$MARKO' ORDER BY id DESC LIMIT 1")
echo "  card id = ${CARD_ID:-<none>}"
expect "cards/client/1" 200 GET /api/cards/client/1 "$T_CLIENT"
expect "cards/account/{num}" 200 GET "/api/cards/account/$MARKO" "$T_EMP"
expect "cards/all" 200 GET /api/cards/all "$T_EMP"
expect "cards/internal/account/{num}" 200 GET "/api/cards/internal/account/$MARKO" "$T_SVC"
if [ -n "$CARD_ID" ]; then
  expect "cards/id/{id}" 200 GET "/api/cards/id/$CARD_ID" "$T_CLIENT"
  reach  "card block" PUT "/api/cards/id/$CARD_ID/block" "$T_CLIENT" '{}'
  reach  "card unblock" PUT "/api/cards/id/$CARD_ID/unblock" "$T_EMP" '{}'
  reach  "card limit" PUT "/api/cards/id/$CARD_ID/limit" "$T_CLIENT" '{"newLimit":50000,"limit":50000}'
  reach  "card deactivate" PUT "/api/cards/id/$CARD_ID/deactivate" "$T_EMP" '{}'
else inf "preskoceni card lifecycle testovi (nema card id)"; fi
dep "POST cards/request [DEP]" POST /api/cards/request "$T_CLIENT" "{\"accountNumber\":\"$MARKO\",\"clientId\":1}"
dep "POST cards/request/business [DEP]" POST /api/cards/request/business "$T_CLIENT" "{\"accountNumber\":\"$MARKO\",\"clientId\":1}"

echo "[M] Verifikacija"
REL="ver-$STAMP"
r=$(call POST /verification/generate "$T_CLIENT" "{\"clientId\":1,\"operationType\":\"PAYMENT\",\"relatedEntityId\":\"$REL\",\"clientEmail\":\"marko.markovic@banka.com\"}")
[ "$(status "$r")" = 200 ] && ok "verification generate (200)" || bad "verification generate $(status "$r")"
VSID=$(bodyof "$r"|sed -n 's/.*"sessionId":\([0-9]*\).*/\1/p')
[ -n "$VSID" ] && expect "verification status" 200 GET "/verification/$VSID/status" "$T_CLIENT"
reach "verification validate (pogresan kod -> 4xx ok)" POST /verification/validate "$T_CLIENT" "{\"sessionId\":$VSID,\"code\":\"000000\"}"

echo
echo "=== REZIME: PASS=$PASS  FAIL=$FAIL  INFO=$INFO ==="
[ "$FAIL" -eq 0 ]
