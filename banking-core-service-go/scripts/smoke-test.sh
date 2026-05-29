#!/usr/bin/env bash
# Smoke test za banking-core-service-go — pokriva endpointe/promene iz porta:
# role-gating (401/403/200 + hijerarhija), CORS, dev-seed (Marko), interni
# credit/debit, exchange buy/sell audit zapis, fund rezervacije, verifikacija,
# margin racuni, i dokaz scheduled job-ova iz logova.
#
# Zahtevi: bash (git-bash), openssl, docker. Servis i Postgres rade u Docker-u.
# JWT-ovi se generišu lokalno (HS256, isti JWT_SECRET), zahtevi idu kroz
# `docker exec <kontejner> curl http://localhost:8084` (port nije objavljen na host).
#
# Pokretanje (iz root-a repo-a):  bash banking-core-service-go/scripts/smoke-test.sh

BC="${BANKING_CORE_CONTAINER:-banka_banking_core_service}"
PG="${POSTGRES_CONTAINER:-banka_postgres}"
DBN="${BANKING_CORE_DB_NAME:-banking_core}"
BASE="${BASE_URL:-http://localhost:8084}"
MARKO="1110001100000000111"

# --- JWT_SECRET ---
SECRET="${JWT_SECRET:-}"
for f in ./setup/.env ../setup/.env ../../setup/.env; do
  [ -z "$SECRET" ] && [ -f "$f" ] && SECRET=$(grep -E '^JWT_SECRET=' "$f" | head -1 | cut -d= -f2- | tr -d '"\r')
done
if [ -z "$SECRET" ]; then echo "GRESKA: postavi JWT_SECRET env ili setup/.env"; exit 1; fi

PASS=0; FAIL=0
ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1"; FAIL=$((FAIL+1)); }

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
mint() { # $1 = claims json
  local h p s
  h=$(printf '%s' '{"alg":"HS256","typ":"JWT"}' | b64url)
  p=$(printf '%s' "$1" | b64url)
  s=$(printf '%s' "$h.$p" | openssl dgst -sha256 -hmac "$SECRET" -binary | b64url)
  printf '%s.%s.%s' "$h" "$p" "$s"
}
EXP=$(( $(date +%s) + 3600 ))
T_CLIENT=$(mint "{\"id\":1,\"roles\":[\"CLIENT_BASIC\"],\"exp\":$EXP}")
T_EMP=$(mint "{\"id\":100,\"roles\":[\"BASIC\"],\"exp\":$EXP}")
T_ADMIN=$(mint "{\"id\":200,\"roles\":[\"ADMIN\"],\"exp\":$EXP}")
T_SVC=$(mint "{\"sub\":\"svc\",\"roles\":[\"SERVICE\"],\"exp\":$EXP}")

# call METHOD PATH TOKEN(-|jwt) BODY(-|json) -> prints "<body>\n<status>"
call() {
  local method="$1" path="$2" token="$3" body="$4"; shift 4
  local args=(-s -m 15 -w $'\n%{http_code}' -X "$method")
  [ "$token" != "-" ] && args+=(-H "Authorization: Bearer $token")
  [ "$body" != "-" ]  && args+=(-H "Content-Type: application/json" -d "$body")
  docker exec "$BC" curl "${args[@]}" "$BASE$path"
}
status() { tail -n1 <<<"$1"; }
bodyof() { sed '$d' <<<"$1"; }
psqlv()  { docker exec "$PG" psql -U postgres -d "$DBN" -t -A -c "$1" 2>/dev/null | tr -d '[:space:]'; }

echo "=== banking-core-service-go smoke test ==="
echo "container=$BC  base=$BASE"
echo

echo "[A] Health + CORS"
r=$(call GET /actuator/health/readiness - -); [ "$(status "$r")" = "200" ] && ok "health readiness 200" || bad "health readiness -> $(status "$r")"
cors=$(docker exec "$BC" curl -s -D - -o /dev/null -X OPTIONS "$BASE/accounts/client/accounts" -H "Origin: http://localhost:4200" -H "Access-Control-Request-Method: GET")
echo "$cors" | grep -qi 'access-control-allow-origin: http://localhost:4200' && ok "CORS preflight Allow-Origin" || bad "CORS preflight bez Allow-Origin"
echo "$cors" | grep -qi 'access-control-allow-credentials: true' && ok "CORS Allow-Credentials" || bad "CORS bez Allow-Credentials"

echo "[B] Auth / role gating"
r=$(call GET /accounts/client/accounts - -);                 [ "$(status "$r")" = "401" ] && ok "bez tokena -> 401" || bad "bez tokena -> $(status "$r")"
r=$(call POST /api/cards/auto "$T_CLIENT" '{}');             [ "$(status "$r")" = "403" ] && ok "CLIENT_BASIC na /api/cards/auto -> 403" || bad "cards/auto wrong role -> $(status "$r")"
r=$(call GET /accounts/api/currencies/getAll "$T_CLIENT" -); [ "$(status "$r")" = "200" ] && ok "currencies (CLIENT_BASIC) -> 200" || bad "currencies -> $(status "$r")"
r=$(call "GET" "/transactions/by-client?id=1" "$T_CLIENT" -);[ "$(status "$r")" = "403" ] && ok "by-client (CLIENT_BASIC) -> 403 (treba BASIC)" || bad "by-client client -> $(status "$r")"
r=$(call "GET" "/transactions/by-client?id=1" "$T_EMP" -);   [ "$(status "$r")" = "200" ] && ok "by-client (BASIC) -> 200" || bad "by-client BASIC -> $(status "$r")"
r=$(call "GET" "/transactions/by-client?id=1" "$T_ADMIN" -); [ "$(status "$r")" = "200" ] && ok "by-client (ADMIN, hijerarhija) -> 200" || bad "by-client ADMIN -> $(status "$r")"

echo "[C] Dev seed (Marko id=1)"
r=$(call GET /accounts/client/accounts "$T_CLIENT" -)
[ "$(status "$r")" = "200" ] && ok "client accounts -> 200" || bad "client accounts -> $(status "$r")"
bodyof "$r" | grep -q "$MARKO" && ok "Markov racun $MARKO prisutan" || bad "Markov racun nije u odgovoru"
r=$(call GET "/internal/accounts/$MARKO/details" "$T_SVC" -); [ "$(status "$r")" = "200" ] && ok "internal account details (SERVICE) -> 200" || bad "internal details -> $(status "$r")"

echo "[D] Interni credit/debit (SERVICE) + balans"
b0=$(psqlv "SELECT raspolozivo_stanje FROM account_table WHERE broj_racuna='$MARKO'")
r=$(call POST /internal/accounts/credit "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":1000,\"clientId\":1}"); [ "$(status "$r")" = "200" ] && ok "credit 1000 -> 200" || bad "credit -> $(status "$r")"
b1=$(psqlv "SELECT raspolozivo_stanje FROM account_table WHERE broj_racuna='$MARKO'")
awk "BEGIN{exit !($b1==$b0+1000)}" && ok "balans porastao za 1000 ($b0 -> $b1)" || bad "balans pogresan ($b0 -> $b1)"
r=$(call POST /internal/accounts/debit "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":1000,\"clientId\":1}"); [ "$(status "$r")" = "200" ] && ok "debit 1000 -> 200" || bad "debit -> $(status "$r")"
b2=$(psqlv "SELECT raspolozivo_stanje FROM account_table WHERE broj_racuna='$MARKO'")
awk "BEGIN{exit !($b2==$b0)}" && ok "balans vracen na pocetni ($b2)" || bad "balans nije vracen ($b0 -> $b2)"

echo "[E] Exchange buy/sell -> payment_table audit (nasa ispravka)"
sp0=$(psqlv "SELECT count(*) FROM payment_table WHERE payment_purpose='Stock purchase'")
r=$(call POST /internal/accounts/exchange/buy "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":10,\"clientId\":1}"); [ "$(status "$r")" = "200" ] && ok "exchange buy -> 200" || bad "exchange buy -> $(status "$r")"
sp1=$(psqlv "SELECT count(*) FROM payment_table WHERE payment_purpose='Stock purchase'")
[ "$sp1" = "$((sp0+1))" ] && ok "'Stock purchase' zapis dodat ($sp0 -> $sp1)" || bad "Stock purchase zapis nije dodat ($sp0 -> $sp1)"
ss0=$(psqlv "SELECT count(*) FROM payment_table WHERE payment_purpose='Stock sale'")
r=$(call POST /internal/accounts/exchange/sell "$T_SVC" "{\"accountNumber\":\"$MARKO\",\"amount\":10,\"clientId\":1}"); [ "$(status "$r")" = "200" ] && ok "exchange sell -> 200" || bad "exchange sell -> $(status "$r")"
ss1=$(psqlv "SELECT count(*) FROM payment_table WHERE payment_purpose='Stock sale'")
[ "$ss1" = "$((ss0+1))" ] && ok "'Stock sale' zapis dodat ($ss0 -> $ss1)" || bad "Stock sale zapis nije dodat ($ss0 -> $ss1)"

echo "[F] Fund rezervacije (SERVICE)"
r=$(call POST /transactions/internal/reserve-funds "$T_SVC" '{"ownerId":1,"amount":500}')
[ "$(status "$r")" = "200" ] && ok "reserve-funds -> 200" || bad "reserve-funds -> $(status "$r")"
RID=$(bodyof "$r" | sed -n 's/.*"reservationId":"\([^"]*\)".*/\1/p')
if [ -n "$RID" ]; then
  r=$(call POST "/transactions/internal/reservations/$RID/commit" "$T_SVC" -); [ "$(status "$r")" = "200" ] && ok "commit rezervacije -> 200" || bad "commit -> $(status "$r")"
else bad "nije vracen reservationId"; fi

echo "[G] Verifikacija (authenticated)"
REL="smoke-$(date +%s)-$RANDOM"
r=$(call POST /verification/generate "$T_CLIENT" "{\"clientId\":1,\"operationType\":\"PAYMENT\",\"relatedEntityId\":\"$REL\",\"clientEmail\":\"marko.markovic@banka.com\"}")
[ "$(status "$r")" = "200" ] && ok "verification generate -> 200" || bad "verification generate -> $(status "$r")"
SID=$(bodyof "$r" | sed -n 's/.*"sessionId":\([0-9]*\).*/\1/p')
if [ -n "$SID" ]; then
  r=$(call GET "/verification/$SID/status" "$T_CLIENT" -); [ "$(status "$r")" = "200" ] && ok "verification status -> 200" || bad "verification status -> $(status "$r")"
else bad "nije vracen sessionId"; fi

echo "[H] Margin racun (authenticated)"
MUID=$(( (RANDOM*RANDOM % 900000) + 100000 ))
r=$(call POST /accounts/createMarginAccount "$T_CLIENT" "{\"employeeId\":100,\"userId\":$MUID,\"initialMargin\":1000,\"maintenanceMargin\":500,\"bankParticipation\":0.5}")
[ "$(status "$r")" = "201" ] && ok "createMarginAccount -> 201" || bad "createMarginAccount -> $(status "$r")  $(bodyof "$r")"
r=$(call GET "/accounts/getMarginUser/$MUID" "$T_CLIENT" -); [ "$(status "$r")" = "200" ] && ok "getMarginUser -> 200" || bad "getMarginUser -> $(status "$r")"

echo "[I] Scheduled jobovi (dokaz iz logova)"
docker logs "$BC" 2>&1 | grep -qiE "stuck payments|spending reset|maintenance" && ok "scheduled job log prisutan" || echo "  [INFO] nema log linije jos (stuck-cleanup ide ~na start; reset/fee idu o ponoci/1. u mesecu)"

echo
echo "=== REZIME: PASS=$PASS  FAIL=$FAIL ==="
[ "$FAIL" -eq 0 ]
