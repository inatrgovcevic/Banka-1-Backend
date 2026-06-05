# Banka 1 — deploy na fakultetski k8s klaster (korak po korak)

> Ovo je kompletno, izvršivo uputstvo. Klaster je **deljen** sa svim timovima — greška u tuđem
> namespace-u ruši i druge. Sve Banka 1 resurse drži u namespace-u **`banka-1`**.
>
> Cilj na kraju: app radi na `https://banka-1.radenkovic.rs`, a inter-bank veza sa **Bankom 2 (222)**
> radi automatski (tokeni/routing su već usklađeni — vidi §6).

---

## 0) Preduslovi

- **`kubectl`** instaliran (`kubectl version --client`).
- **Kubeconfig** fajl za fakultetski klaster (dobija se od asistenta/`radenkovic` — isti klaster kao
  Banka 2, vaš namespace je `banka-1`). Vidi §1.
- **GHCR image-i postoje** za svih 8 Go servisa + frontend. Ako ne — prvo `BRANCH_PROMENA.md`
  (go→main) + `cd-go.yml`/`cd-frontend.yml`, pa sačekaj da Actions `cd-go` bude zeleno. **Bez image-a
  deploy ne radi.**

---

## 1) Kubeconfig — povezivanje na klaster

Kubeconfig je fajl koji `kubectl`-u kaže GDE je klaster i KAKO se autentifikovati. Default lokacija
je **`~/.kube/config`** (Windows: `C:\Users\<ti>\.kube\config`).

```bash
# Varijanta A — stavi ga kao default config:
mkdir -p ~/.kube
cp /putanja/do/dobijenog/kubeconfig ~/.kube/config

# Varijanta B — bez diranja default-a, samo za ovu sesiju (env var):
export KUBECONFIG=/putanja/do/dobijenog/kubeconfig         # Windows PS: $env:KUBECONFIG="C:\...\kubeconfig"
```

Provera da radi:

```bash
kubectl config current-context          # treba kontekst fakultetskog klastera (npr. studenti@elab)
kubectl get nodes                       # treba lista node-ova (Ready)
kubectl get ns banka-1 || echo "namespace ce biti kreiran u koraku 3"
```

> Ako `kubectl get nodes` ne radi → kubeconfig nije ispravan/nije za ovaj klaster. Stani i proveri sa asistentom.

Postavi `banka-1` kao default namespace za sve komande (da ne kucaš `-n banka-1` svaki put):

```bash
kubectl config set-context --current --namespace=banka-1
```

---

## 2) Tokeni i tajne (šta gde ide)

`interbank-service` čita partnere ISKLJUČIVO iz jednog env stringa `INTERBANK_PARTNERS_JSON` (ne može
zaseban-env po tokenu), pa ceo JSON (sa tokenima) ide u **Secret `interbank-partners`**. Tokeni su
**već dogovoreni sa Bankom 2** (vidi §6) — ne menjaj ih.

> **Gde su prave token vrednosti:** u repo-u (`secrets.yaml.example`, `SECRETS.md`) stoje samo
> `REPLACE_ME-*` placeholder-i — realne tokene NE commit-ujemo. Prave 64-hex vrednosti + `kubectl
> create secret` komanda su **ovde** (§3.2) i u tabeli §6. Smer je verifikovan UŽIVO protiv Banke 2
> (naš outbound token je prihvaćen, 200), pa ovde važi: **OutboundToken = `65f28a2b…`** (šaljemo),
> **InboundToken = `0a1284…`** (primamo).

- `app-secrets` → `JWT_SECRET` (S2S + FE token verifikacija; isti za sve servise).
- `interbank-partners` → `INTERBANK_PARTNERS_JSON` (registracija Banke 2 + oba X-Api-Key tokena).
- `rabbitmq-credentials`, `influxdb-credentials` → po potrebi (vidi `SECRETS.md`).
- DB lozinke → **NE praviš ručno**; Crunchy operator ih auto-generiše u `db-pguser-<name>` Secret-e.

---

## 3) Deploy (tačan redosled)

Iz repo-a `Banka-1-Infrastructure-main/`:

```bash
# 3.1 Namespace
kubectl apply -f 00-namespace.yaml

# 3.2 Tajne (PRE servisa — bez njih pod ne startuje)
kubectl create secret generic app-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 48)" \
  -n banka-1

# interbank-partners — CEO PARTNERS_JSON kao jedan ključ (tokeni su FINALNI, ne diraj):
kubectl create secret generic interbank-partners -n banka-1 \
  --from-literal=INTERBANK_PARTNERS_JSON='[{"Routing":222,"DisplayName":"Banka 2","BaseURL":"https://banka-2.radenkovic.rs/api","InboundToken":"0a12840537dc295ae617f3376f7f2af0a67c18577a893ae43d6022bee06db601","OutboundToken":"65f28a2bff9b02aee724e284eaf1e87fd05fc42124e70879b5d718add87ec77d"}]'

# rabbitmq + influx (vrednosti po želji; vidi SECRETS.md za pun set)
kubectl create secret generic rabbitmq-credentials -n banka-1 \
  --from-literal=RABBITMQ_DEFAULT_USER="banka1" \
  --from-literal=RABBITMQ_DEFAULT_PASS="$(openssl rand -hex 16)"
kubectl create secret generic influxdb-credentials -n banka-1 \
  --from-literal=INFLUX_ADMIN_USERNAME="banka1-admin" \
  --from-literal=INFLUX_ADMIN_PASSWORD="$(openssl rand -hex 16)" \
  --from-literal=INFLUX_ADMIN_TOKEN="$(openssl rand -hex 32)"

# (ako koristiš privatne GHCR pakete — dodaj imagePullSecret; vidi SECRETS.md)

# 3.3 Baza + aux (sačekaj da Crunchy bude ready)
kubectl apply -f db.yaml -f redis.yaml -f rabbitmq.yaml -f influxdb.yaml
kubectl wait --for=condition=ready pod \
  -l postgres-operator.crunchydata.com/cluster=db -n banka-1 --timeout=300s
# Crunchy je napravio db-pguser-* Secret-e za svaku logičku bazu — servisi ih čitaju.

# 3.4 Aplikacioni servisi
kubectl apply -f user-service.yaml -f banking-core-service.yaml -f market-service.yaml \
  -f trading-service.yaml -f credit-service.yaml -f notification-service.yaml \
  -f saga-orchestrator-service.yaml -f interbank-service.yaml -f frontend.yaml

# 3.5 Gateway ruta
kubectl apply -f route.yaml
```

---

## 4) Verifikacija

```bash
kubectl get pods -n banka-1                 # svi Running/healthy (sačekaj ~1-2 min)
kubectl rollout status deploy/interbank-service -n banka-1 --timeout=180s

# App spolja:
curl -s -o /dev/null -w "FE: %{http_code}\n" https://banka-1.radenkovic.rs/

# Inter-bank inbound (Banka 1 prima) — Banka 1 InboundToken (token koji Banka 2 salje kad zove nas):
curl -s -o /dev/null -w "public-stock (sa kljucem): %{http_code}\n" \
  https://banka-1.radenkovic.rs/public-stock \
  -H "X-Api-Key: 0a12840537dc295ae617f3376f7f2af0a67c18577a893ae43d6022bee06db601"
# Bez ključa mora 401 (NE 200 + HTML SPA — to znači da ruta ne pogađa interbank-service):
curl -s -o /dev/null -w "public-stock (bez kljuca): %{http_code}\n" \
  https://banka-1.radenkovic.rs/public-stock
```

Ako se pod ne diže: `kubectl logs deploy/<servis> -n banka-1` i
`kubectl describe pod <pod> -n banka-1` (česti uzroci: image ne postoji na GHCR → §0; pogrešna
probe putanja; Secret ključ fali).

Redeploy posle novog image-a (imagePullPolicy: Always):

```bash
kubectl rollout restart deploy/<servis> -n banka-1
```

---

## 5) Smer ka Banci 2 (outbound) — provera

`interbank-service` zove Banku 2 na `https://banka-2.radenkovic.rs/api` sa tokenom
`0a12840537...`. Banka 2 je VEĆ podešena da prihvati taj token od Banke 1. Test (kroz vaš FE OTC
discovery ili interno) — Banka 1 čita Banka 2 public-stock; očekivano 200 + realne ponude.

---

## 6) Zašto je veza automatska (routing + tokeni — već usklađeno)

| | Banka 1 (vi) | Banka 2 (mi) |
|---|---|---|
| Routing | **111** | **222** |
| Inbound URL (spolja) | `https://banka-1.radenkovic.rs/interbank` (BARE, bez `/api`) | `https://banka-2.radenkovic.rs/api/interbank` (sa `/api`) |
| Token koji Banka 1 ŠALJE (vi→mi) = **OutboundToken** | `65f28a2bff9b02aee724e284eaf1e87fd05fc42124e70879b5d718add87ec77d` | — |
| Token koji Banka 1 PRIMA (mi→vi) = **InboundToken** | `0a12840537dc295ae617f3376f7f2af0a67c18577a893ae43d6022bee06db601` | — |

- **Naša (Banka 2) strana je već uживо podešena**: `app-secrets` + deployment imaju ova dva tokena za
  partnera 111, i `PARTNER1_BASE_URL=https://banka-1.radenkovic.rs`. Inbound od vas smo testirali
  (token `65f28a2b` → 200).
- Vaš `interbank-service.yaml` (preko Secret-a `interbank-partners`) ima identičan par.
- **Zato: čim digneš app na k8s sa ovim manifestima — povezani smo u oba smera, bez ijedne dodatne
  akcije sa naše strane.** (Outbound mi→vi proradi čim je vaš `/interbank` dostupan spolja, što
  `route.yaml` obezbeđuje.)

---

## 7) Posle merge-a PR-ova — DevOps runbook (korak po korak)

Tri PR-a (Backend / Frontend / Infrastructure) idu na `upstream/main`. Kad se **merge-uju**, redosled
za DevOps tim je:

**7.1 Sačekaj da CI objavi image-e (preduslov za k8s).**
- Merge na `main` u **Backend** repo-u okida `cd-go.yml` → build + push 8 Go image-a na GHCR
  (`ghcr.io/raf-si-2025/banka-1-{user,banking-core,market,trading,credit,notification,saga-orchestrator,interbank}-service:latest`).
- Merge na `main` u **Frontend** repo-u okida `cd-frontend.yml` → `ghcr.io/raf-si-2025/banka-1-frontend:latest`.
- Sačekaj da oba Actions run-a budu **zelena** (GitHub → Actions). Bez image-a pod-ovi ne mogu da se povuku.
- Proveri da paketi postoje: GitHub → Organization `RAF-SI-2025` → Packages. Ako su **private**, ili ih
  prebaci na public, ili u namespace-u napravi pull secret:
  ```bash
  kubectl create secret docker-registry ghcr-pull -n banka-1 \
    --docker-server=ghcr.io --docker-username=<gh-user> --docker-password=<gh-PAT-read:packages>
  ```
  (i dodaj `imagePullSecrets: [{name: ghcr-pull}]` u Deployment-e, ili na default ServiceAccount).

**7.2 Konektuj se na klaster.** Kubeconfig + `kubectl config set-context --current --namespace=banka-1` (§1).

**7.3 Kreiraj Secret-e PRE servisa (§2 / §3.2).** `app-secrets` (JWT), **`interbank-partners`** sa
**pravim** tokenima (vrednosti iz §3.2 / tabele §6 — NISU u git-u), `rabbitmq-credentials`,
`influxdb-credentials`, `mail-credentials`. (DB lozinke ne praviš — Crunchy ih auto-generiše.)

**7.4 Apply manifesta tačnim redosledom (§3.3–3.5):**
`00-namespace.yaml` → `db.yaml`+`redis`+`rabbitmq`+`influxdb` (sačekaj `kubectl wait` da je Crunchy
ready) → 8 app servisa + `frontend.yaml` → `route.yaml`.

**7.5 Verifikuj (§4 + §5):** svi pod-ovi `Running/healthy`; `FE → 200`; inbound `public-stock` sa
InboundToken `0a1284…` → 200, bez ključa → 401; outbound (Banka 1 čita Banka 2 public-stock) → 200.

**7.6 Redeploy posle novih image-a:** `kubectl rollout restart deploy/<servis> -n banka-1`
(`imagePullPolicy: Always` povuče najnoviji `:latest`).

> Posle 7.4 + 7.5 inter-bank veza sa Bankom 2 radi automatski — routing/tokeni su već usklađeni (§6).
