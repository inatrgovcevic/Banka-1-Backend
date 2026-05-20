# Analytics Spark Job

Batch analytics job for the Banka 1 project. The job is designed to run on Kubernetes
through the Spark Operator and materialize analytics results into the `trading`
Postgres database.

## Outputs

- `analytics_job_runs`
- `analytics_client_segments`
- `analytics_portfolio_risk`
- `analytics_top_tickers`

## Required environment

- `TRADING_JDBC_URL`
- `TRADING_DB_USER`
- `TRADING_DB_PASSWORD`
- `MARKET_JDBC_URL`
- `MARKET_DB_USER`
- `MARKET_DB_PASSWORD`

Optional:

- `ANALYTICS_KMEANS_K` defaults to `4`
- `POSTGRES_JDBC_DRIVER` defaults to `org.postgresql.Driver`

## Kubernetes

`k8s/scheduled-spark-application.yaml` expects the Spark Operator CRDs and a
`banka-analytics-db` secret in the same namespace. `k8s/secret.example.yaml`
shows the required keys.

## Local submit example

```bash
spark-submit \
  --packages org.postgresql:postgresql:42.7.4 \
  analytics-spark-job/app/analytics_job.py
```
