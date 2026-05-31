#!/usr/bin/env bash
BASE="http://localhost:8084"

check() {
  local label="$1" method="$2" url="$3"
  shift 3
  local code body
  code=$(curl -s -o /tmp/resp.txt -w "%{http_code}" -X "$method" "$@" "$url" 2>/dev/null)
  body=$(head -c 120 /tmp/resp.txt | tr '\n' ' ')
  local icon="OK"; [[ "$code" -ge 500 ]] && icon="XX"
  printf "%-6s %-55s %s  %s\n" "$method" "$label" "$icon $code" "$body"
}

CLI=(-H "X-Client-Id: 1"  -H "X-User-Roles: ROLE_CLIENT_BASIC")
EMP=(-H "X-User-Id: 1"    -H "X-User-Roles: ROLE_BASIC")
SVC=(-H "X-User-Id: -1"   -H "X-User-Roles: SERVICE")
ADMN=(-H "X-User-Id: 1"   -H "X-User-Roles: ROLE_ADMIN")
SVC_ADM=(-H "X-User-Id: 1" -H "X-User-Roles: ROLE_ADMIN")
JSON=(-H "Content-Type: application/json")

echo "== HEALTH =="
check "/actuator/health/liveness"       GET "$BASE/actuator/health/liveness"
check "/actuator/health/readiness"      GET "$BASE/actuator/health/readiness"
check "/actuator/health"                GET "$BASE/actuator/health"
check "/actuator/info"                  GET "$BASE/actuator/info"
check "/actuator/prometheus"            GET "$BASE/actuator/prometheus"
echo "== CURRENCIES =="
check "/accounts/api/currencies/getAll"    GET "$BASE/accounts/api/currencies/getAll"    "${CLI[@]}"
check "/accounts/api/currencies"           GET "$BASE/accounts/api/currencies"           "${EMP[@]}"
check "/accounts/api/currencies?code=RSD"  GET "$BASE/accounts/api/currencies?code=RSD"  "${CLI[@]}"
echo "== ACCOUNTS INTERNAL =="
check "/accounts/internal/default/1"  GET "$BASE/accounts/internal/default/1"
echo "== ACCOUNTS EMPLOYEE =="
check "/accounts/employee/accounts"                     GET "$BASE/accounts/employee/accounts"                      "${EMP[@]}"
check "/accounts/employee/accounts/ACC123"              GET "$BASE/accounts/employee/accounts/ACC123"               "${EMP[@]}"
check "/accounts/employee/accounts/ACC123/status PUT"   PUT "$BASE/accounts/employee/accounts/ACC123/status"        "${EMP[@]}" "${JSON[@]}" -d '{"status":"ACTIVE"}'
check "/accounts/employee/accounts/bank"                GET "$BASE/accounts/employee/accounts/bank"                 "${EMP[@]}"
check "/accounts/employee/accounts/bank/RSD"            GET "$BASE/accounts/employee/accounts/bank/RSD"             "${EMP[@]}"
check "/accounts/employee/accounts/client/1"            GET "$BASE/accounts/employee/accounts/client/1"             "${EMP[@]}"
check "/accounts/employee/accounts/checking POST"       POST "$BASE/accounts/employee/accounts/checking"            "${EMP[@]}" "${JSON[@]}" -d '{"vlasnik":1,"currency":"RSD"}'
check "/accounts/employee/accounts/fx POST"             POST "$BASE/accounts/employee/accounts/fx"                  "${EMP[@]}" "${JSON[@]}" -d '{"vlasnik":1,"currency":"EUR"}'
check "/accounts/employee/companies/1 GET"              GET "$BASE/accounts/employee/companies/1"                   "${EMP[@]}"
check "/accounts/employee/companies/1 PUT"              PUT "$BASE/accounts/employee/companies/1"                   "${EMP[@]}" "${JSON[@]}" -d '{"name":"Test"}'
echo "== ACCOUNTS CLIENT =="
check "/accounts/client/accounts"                GET "$BASE/accounts/client/accounts"                     "${CLI[@]}"
check "/accounts/client/api/accounts/1"          GET "$BASE/accounts/client/api/accounts/1"               "${CLI[@]}"
check "/accounts/client/api/accounts/ACC123"     GET "$BASE/accounts/client/api/accounts/ACC123"          "${CLI[@]}"
check "/accounts/client/api/accounts/1/cards"    GET "$BASE/accounts/client/api/accounts/1/cards"         "${CLI[@]}"
check "/accounts/client/api/accounts/1/name PUT" PUT "$BASE/accounts/client/api/accounts/1/name"          "${CLI[@]}" "${JSON[@]}" -d '{"name":"Racun A"}'
check "/accounts/client/api/accounts/A/name PUT" PUT "$BASE/accounts/client/api/accounts/ACC123/name"     "${CLI[@]}" "${JSON[@]}" -d '{"name":"Racun B"}'
check "/accounts/client/api/accounts/1/limits"   PUT "$BASE/accounts/client/api/accounts/1/limits"        "${CLI[@]}" "${JSON[@]}" -d '{"dailyLimit":5000}'
echo "== MARGIN =="
check "/accounts/createMarginAccount"         POST "$BASE/accounts/createMarginAccount"         "${CLI[@]}" "${JSON[@]}" -d '{"employeeId":1,"userId":2,"initialMargin":"1000","maintenanceMargin":"500","bankParticipation":"0.5"}'
check "/accounts/company/createMarginAccount" POST "$BASE/accounts/company/createMarginAccount" "${CLI[@]}" "${JSON[@]}" -d '{"employeeId":1,"companyId":2,"initialMargin":"2000","maintenanceMargin":"800","bankParticipation":"0.4"}'
check "/accounts/getMarginUser/2"             GET  "$BASE/accounts/getMarginUser/2"              "${CLI[@]}"
check "/accounts/company/getMarginCompany/2"  GET  "$BASE/accounts/company/getMarginCompany/2"   "${CLI[@]}"
echo "== INTERNAL ACCOUNTS =="
check "/internal/accounts/debit"               POST   "$BASE/internal/accounts/debit"          "${SVC[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC123","amount":"100","clientId":1}'
check "/internal/accounts/credit"              POST   "$BASE/internal/accounts/credit"         "${SVC[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC123","amount":"100","clientId":1}'
check "/internal/accounts/creditBank"          POST   "$BASE/internal/accounts/creditBank"     "${SVC[@]}" "${JSON[@]}" -d '{"currencyCode":"RSD","amount":100}'
check "/internal/accounts/debitBank"           POST   "$BASE/internal/accounts/debitBank"      "${SVC[@]}" "${JSON[@]}" -d '{"currencyCode":"RSD","amount":100}'
check "/internal/accounts/ACC123/details"      GET    "$BASE/internal/accounts/ACC123/details"  "${SVC[@]}"
check "/internal/accounts/id/1/details"        GET    "$BASE/internal/accounts/id/1/details"    "${SVC[@]}"
check "/internal/accounts/bank/RSD"            GET    "$BASE/internal/accounts/bank/RSD"         "${SVC[@]}"
check "/internal/accounts/state/RSD"           GET    "$BASE/internal/accounts/state/RSD"        "${SVC[@]}"
check "/internal/accounts/info"                GET    "$BASE/internal/accounts/info?fromBankNumber=111&toBankNumber=222" "${SVC[@]}"
check "/internal/accounts/system"              POST   "$BASE/internal/accounts/system"          "${EMP[@]}" "${JSON[@]}" -d '{"ownerType":"BANK","currency":"RSD"}'
check "/internal/accounts/transaction"         POST   "$BASE/internal/accounts/transaction"     "${SVC[@]}" "${JSON[@]}" -d '{"fromAccount":"A1","toAccount":"A2","amount":"50"}'
check "/internal/accounts/exchange/buy"        POST   "$BASE/internal/accounts/exchange/buy"    "${SVC[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC","amount":"100","currency":"EUR"}'
check "/internal/accounts/exchange/sell"       POST   "$BASE/internal/accounts/exchange/sell"   "${SVC[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC","amount":"100","currency":"EUR"}'
check "/internal/accounts/transactionFromBank" POST   "$BASE/internal/accounts/transactionFromBank" "${SVC[@]}" "${JSON[@]}" -d '{"toAccount":"ACC","amount":"100","currency":"RSD"}'
check "/internal/accounts/transfer"            POST   "$BASE/internal/accounts/transfer"        "${SVC[@]}" "${JSON[@]}" -d '{"fromAccountNumber":"A1","toAccountNumber":"A2","amount":"50"}'
echo "== CARDS =="
check "/api/cards/all (BASIC)"                  GET  "$BASE/api/cards/all"                        "${EMP[@]}"
check "/api/cards/account/ACC123 (BASIC)"        GET  "$BASE/api/cards/account/ACC123"              "${EMP[@]}"
check "/api/cards/client/1 (CLIENT_BASIC)"       GET  "$BASE/api/cards/client/1"                   "${CLI[@]}"
check "/api/cards/internal/account/ACC (SVC)"    GET  "$BASE/api/cards/internal/account/ACC123"     "${SVC[@]}"
check "/api/cards/id/1 (CLIENT_BASIC)"           GET  "$BASE/api/cards/id/1"                       "${CLI[@]}"
check "/api/cards/id/1/block (CLIENT_BASIC)"     PUT  "$BASE/api/cards/id/1/block"                  "${CLI[@]}"
check "/api/cards/id/1/unblock (BASIC)"          PUT  "$BASE/api/cards/id/1/unblock"                "${EMP[@]}"
check "/api/cards/id/1/deactivate (BASIC)"       PUT  "$BASE/api/cards/id/1/deactivate"             "${EMP[@]}"
check "/api/cards/request (CLIENT_BASIC)"        POST "$BASE/api/cards/request"                    "${CLI[@]}" "${JSON[@]}" -d '{"accountId":1,"cardType":"DEBIT","cardBrand":"VISA"}'
check "/api/cards/auto (SERVICE+ADMIN)"          POST "$BASE/api/cards/auto"                       "${SVC_ADM[@]}" "${JSON[@]}" -d '{"accountId":1,"cardBrand":"VISA"}'
echo "== VERIFICATION =="
check "/verification/generate" POST "$BASE/verification/generate" "${CLI[@]}" "${JSON[@]}" -d '{"operationType":"PAYMENT","clientId":1}'
check "/verification/validate" POST "$BASE/verification/validate" "${CLI[@]}" "${JSON[@]}" -d '{"operationType":"PAYMENT","clientId":1,"otpCode":"123456","sessionId":"test"}'
echo "== TRANSACTIONS =="
check "/transactions/payment (CLIENT_BASIC)"               POST   "$BASE/transactions/payment"                    "${CLI[@]}" "${JSON[@]}" -d '{"fromAccount":"A1","toAccount":"A2","amount":"50","currency":"RSD"}'
check "/transactions/by-client?id=1 (BASIC)"               GET    "$BASE/transactions/by-client?id=1"             "${EMP[@]}"
check "/transactions/by-this-client (CLIENT_BASIC)"         GET    "$BASE/transactions/by-this-client"             "${CLI[@]}"
check "/transactions/by-sender-client?id=1 (BASIC)"         GET    "$BASE/transactions/by-sender-client?id=1"      "${EMP[@]}"
check "/transactions/by-recipient-client?id=1 (BASIC)"      GET    "$BASE/transactions/by-recipient-client?id=1"   "${EMP[@]}"
check "/transactions/by-this-sender-client (CLI)"           GET    "$BASE/transactions/by-this-sender-client"      "${CLI[@]}"
check "/transactions/api/payments (CLI)"                    GET    "$BASE/transactions/api/payments?accountNumber=A" "${CLI[@]}"
check "/transactions/accounts/ACC123 (CLI)"                 GET    "$BASE/transactions/accounts/ACC123"            "${CLI[@]}"
check "/transactions/employee/accounts/ACC123 (BASIC)"      GET    "$BASE/transactions/employee/accounts/ACC123"   "${EMP[@]}"
check "/transactions/internal/reserve-funds (SVC)"          POST   "$BASE/transactions/internal/reserve-funds"    "${SVC[@]}" "${JSON[@]}" -d '{"ownerId":1,"amount":"100"}'
check "/transactions/internal/reservations/x/commit (SVC)"  POST   "$BASE/transactions/internal/reservations/00000000-0000-0000-0000-000000000001/commit" "${SVC[@]}"
check "/transactions/internal/reservations/x DEL (SVC)"     DELETE "$BASE/transactions/internal/reservations/00000000-0000-0000-0000-000000000001" "${SVC[@]}"
check "/transactions/internal/transfer (SVC)"               POST   "$BASE/transactions/internal/transfer"         "${SVC[@]}" "${JSON[@]}" -d '{"fromAccountNumber":"A1","toAccountNumber":"A2","amount":"50"}'
check "/transactions/internal/transfers/x/reverse (SVC)"    POST   "$BASE/transactions/internal/transfers/00000000-0000-0000-0000-000000000001/reverse" "${SVC[@]}"
echo "== MARGIN TX =="
check "/transactions/stockBuyMarginTransaction"    POST "$BASE/transactions/stockBuyMarginTransaction"  "${CLI[@]}" "${JSON[@]}" -d '{"userId":2,"amount":"100"}'
check "/transactions/stockSellMarginTransaction"   POST "$BASE/transactions/stockSellMarginTransaction" "${CLI[@]}" "${JSON[@]}" -d '{"userId":2,"amount":"50"}'
check "/transactions/addToMargin/2"                POST "$BASE/transactions/addToMargin/2"               "${CLI[@]}" "${JSON[@]}" -d '{"amount":"200"}'
check "/transactions/withdrawFromMargin/2"         POST "$BASE/transactions/withdrawFromMargin/2"        "${CLI[@]}" "${JSON[@]}" -d '{"amount":"50"}'
check "/transactions/getAllMarginTransactions/A"    GET  "$BASE/transactions/getAllMarginTransactions/ACC123" "${CLI[@]}"
echo "== TRANSFERS =="
check "/transfers POST"              POST   "$BASE/transfers"               "${CLI[@]}" "${JSON[@]}" -d '{"fromAccount":"A1","toAccount":"A2","amount":"50","currency":"RSD"}'
check "/transfers?clientId=1 GET"    GET    "$BASE/transfers?clientId=1"    "${CLI[@]}"
check "/transfers/ORDER123 GET"      GET    "$BASE/transfers/ORDER123"      "${CLI[@]}"
check "/transfers/accounts/ACC GET"  GET    "$BASE/transfers/accounts/ACC123" "${CLI[@]}"
echo "== PAYMENT RECIPIENTS =="
check "/payment-recipients GET"    GET    "$BASE/payment-recipients"    "${CLI[@]}"
check "/payment-recipients POST"   POST   "$BASE/payment-recipients"    "${CLI[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC123","name":"Test primalac","ownerName":"Pera"}'
check "/payment-recipients/1 PUT"  PUT    "$BASE/payment-recipients/1"  "${CLI[@]}" "${JSON[@]}" -d '{"accountNumber":"ACC123","name":"Test primalac 2","ownerName":"Pera"}'
check "/payment-recipients/1 DEL"  DELETE "$BASE/payment-recipients/1"  "${CLI[@]}"
echo "== INTERBANK =="
check "/internal/interbank/reserve-monas"         POST   "$BASE/internal/interbank/reserve-monas"                                            "${SVC[@]}" "${JSON[@]}" -d '{"accountNum":"ACC","currency":"RSD","amount":"100","transactionIdRouting":1,"transactionIdLocal":"TX1"}'
check "/internal/interbank/reservations/x/commit" POST   "$BASE/internal/interbank/reservations/00000000-0000-0000-0000-000000000001/commit-monas" "${SVC[@]}"
check "/internal/interbank/reservations/x DELETE" DELETE "$BASE/internal/interbank/reservations/00000000-0000-0000-0000-000000000001"         "${SVC[@]}"
check "/internal/interbank/account-resolve"       GET    "$BASE/internal/interbank/account-resolve?num=ACC123"                               "${SVC[@]}"
check "/internal/interbank/account-by-owner"      GET    "$BASE/internal/interbank/account-by-owner?ownerId=1&currency=RSD"                  "${SVC[@]}"
echo "== AUTH GUARDS =="
check "No auth -> CLIENT endpoint"       GET    "$BASE/accounts/client/accounts"
check "Wrong role -> EMPLOYEE endpoint"  GET    "$BASE/accounts/employee/accounts"   -H "X-User-Id: 1" -H "X-User-Roles: ROLE_CLIENT_BASIC"
check "No auth -> SERVICE endpoint"      GET    "$BASE/internal/accounts/bank/RSD"
check "405 - known path wrong method"   DELETE "$BASE/actuator/health"
check "404 - unknown path"              GET    "$BASE/nepostojeca/putanja/xyz"
