# Secret-i za `banka-1`

K8s manifesti sada koriste jedan zajednicki Secret:

## `banka-app-secrets`

Definisan je u [app-secrets.yaml](./app-secrets.yaml) i sadrzi:

- DB host/port/name/user/password vrednosti za servise
- `JWT_SECRET`
- RabbitMQ kredencijale
- InfluxDB admin/token vrednosti
- `MAIL_USERNAME` i `MAIL_PASSWORD`
- market API kljuceve
- interbank partner tokene i base URL

Primena:

```bash
kubectl apply -f setup/k8s/app-secrets.yaml
```

Ako zelis placeholder verziju za novu sredinu, koristi [secrets.yaml.example](./secrets.yaml.example).
