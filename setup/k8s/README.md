# Kubernetes manifesti

Ovaj folder sada prati profesorsku konvenciju iz [DEPLOY_K8S_Banka-1.md](./DEPLOY_K8S_Banka-1.md):

- svi deploy manifesti su flat u `setup/k8s/`
- namespace je iskljucivo `banka-1`
- baza ide preko Crunchy `PostgresCluster`
- svi aplikacioni servisi koriste zajednicki `app-secrets.yaml` (`banka-app-secrets`)
- `app-secrets.yaml` sadrzi DB kredencijale, JWT, RabbitMQ, mail i market/interbank secret vrednosti

## Glavni fajlovi

- `00-namespace.yaml`
- `db.yaml`
- `redis.yaml`
- `rabbitmq.yaml`
- `influxdb.yaml`
- `user-service.yaml`
- `banking-core-service.yaml`
- `market-service.yaml`
- `trading-service.yaml`
- `credit-service.yaml`
- `notification-service.yaml`
- `saga-orchestrator-service.yaml`
- `interbank-service.yaml`
- `frontend.yaml`
- `route.yaml`

## Secret dokumentacija

- [SECRETS.md](./SECRETS.md) - kako se koristi `app-secrets.yaml`
- [secrets.yaml.example](./secrets.yaml.example) - placeholder primer za `banka-app-secrets`

## Primena

Najprecizniji redosled je vec opisan u [DEPLOY_K8S_Banka-1.md](./DEPLOY_K8S_Banka-1.md).

Ako zelis samo render/proveru:

```bash
kubectl kustomize setup/k8s
```

Ako hoces da primenis secret direktno:

```bash
kubectl apply -f setup/k8s/app-secrets.yaml
```
