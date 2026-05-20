import os
import uuid
from datetime import datetime, timezone

from pyspark.ml.clustering import KMeans
from pyspark.ml.feature import StandardScaler, VectorAssembler
from pyspark.sql import SparkSession, Window
from pyspark.sql import functions as F
from pyspark.sql.types import (
    DecimalType,
    IntegerType,
    LongType,
    StringType,
    StructField,
    StructType,
    TimestampType,
)


JOB_NAME = "trading-analytics-spark"


def getenv(name, default=None):
    value = os.getenv(name, default)
    if value is None or value == "":
        raise RuntimeError(f"Missing required environment variable: {name}")
    return value


def jdbc_options(url, user, password):
    return {
        "url": url,
        "user": user,
        "password": password,
        "driver": os.getenv("POSTGRES_JDBC_DRIVER", "org.postgresql.Driver"),
    }


def read_query(spark, options, query):
    return (
        spark.read.format("jdbc")
        .options(**options)
        .option("dbtable", f"({query}) q")
        .load()
    )


def write_table(df, options, table):
    (
        df.write.format("jdbc")
        .options(**options)
        .option("dbtable", table)
        .mode("append")
        .save()
    )


def empty_df(spark, schema):
    return spark.createDataFrame([], schema)


def add_run_columns(df, run_id, computed_at):
    return df.withColumn("run_id", F.lit(run_id)).withColumn("computed_at", F.lit(computed_at))


def load_sources(spark, trading_options, market_options):
    portfolio = read_query(
        spark,
        trading_options,
        """
        select user_id, listing_id, listing_type, quantity, average_purchase_price
        from portfolio
        where quantity > 0
        """,
    )
    orders = read_query(
        spark,
        trading_options,
        """
        select id, user_id, listing_id, quantity, price_per_unit, direction, status
        from orders
        """,
    )
    transactions = read_query(
        spark,
        trading_options,
        """
        select id, order_id, quantity, price_per_unit, total_price
        from transactions
        """,
    )
    listings = read_query(
        spark,
        market_options,
        """
        select id, ticker, listing_type, price
        from listing
        """,
    )
    return portfolio, orders, transactions, listings


def build_portfolio_risk(spark, portfolio, listings, run_id, computed_at):
    schema = StructType(
        [
            StructField("run_id", StringType(), False),
            StructField("user_id", LongType(), False),
            StructField("total_market_value", DecimalType(19, 4), False),
            StructField("total_cost_basis", DecimalType(19, 4), False),
            StructField("unrealized_pnl", DecimalType(19, 4), False),
            StructField("holdings_count", IntegerType(), False),
            StructField("max_holding_percent", DecimalType(9, 4), False),
            StructField("diversification_score", DecimalType(9, 4), False),
            StructField("risk_score", DecimalType(9, 4), False),
            StructField("risk_level", StringType(), False),
            StructField("computed_at", TimestampType(), False),
        ]
    )
    if portfolio.rdd.isEmpty():
        return empty_df(spark, schema)

    holdings = (
        portfolio.alias("p")
        .join(listings.alias("l"), F.col("p.listing_id") == F.col("l.id"), "left")
        .withColumn("market_price", F.coalesce(F.col("l.price"), F.lit(0)))
        .withColumn("market_value", F.col("p.quantity") * F.col("market_price"))
        .withColumn("cost_basis", F.col("p.quantity") * F.col("p.average_purchase_price"))
    )

    aggregate = holdings.groupBy("user_id").agg(
        F.sum("market_value").alias("total_market_value"),
        F.sum("cost_basis").alias("total_cost_basis"),
        F.countDistinct("listing_id").cast("int").alias("holdings_count"),
    )

    by_holding = holdings.join(
        aggregate.select("user_id", "total_market_value"),
        "user_id",
        "inner",
    ).withColumn(
        "holding_percent",
        F.when(F.col("total_market_value") > 0, F.col("market_value") * 100 / F.col("total_market_value")).otherwise(0),
    )

    concentration = by_holding.groupBy("user_id").agg(F.max("holding_percent").alias("max_holding_percent"))

    risk = (
        aggregate.join(concentration, "user_id", "left")
        .withColumn("unrealized_pnl", F.col("total_market_value") - F.col("total_cost_basis"))
        .withColumn(
            "diversification_score",
            F.least(F.lit(100.0), F.col("holdings_count") * F.lit(20.0))
            * (F.lit(1.0) - F.coalesce(F.col("max_holding_percent"), F.lit(0.0)) / F.lit(100.0)),
        )
        .withColumn(
            "risk_score",
            F.least(
                F.lit(100.0),
                F.coalesce(F.col("max_holding_percent"), F.lit(0.0)) * F.lit(0.65)
                + (F.lit(100.0) - F.col("diversification_score")) * F.lit(0.35),
            ),
        )
        .withColumn(
            "risk_level",
            F.when(F.col("risk_score") >= 70, F.lit("HIGH"))
            .when(F.col("risk_score") >= 40, F.lit("MEDIUM"))
            .otherwise(F.lit("LOW")),
        )
    )

    return (
        add_run_columns(risk, run_id, computed_at)
        .select(
            "run_id",
            F.col("user_id").cast("long"),
            F.round("total_market_value", 4).cast("decimal(19,4)").alias("total_market_value"),
            F.round("total_cost_basis", 4).cast("decimal(19,4)").alias("total_cost_basis"),
            F.round("unrealized_pnl", 4).cast("decimal(19,4)").alias("unrealized_pnl"),
            F.col("holdings_count").cast("int"),
            F.round("max_holding_percent", 4).cast("decimal(9,4)").alias("max_holding_percent"),
            F.round("diversification_score", 4).cast("decimal(9,4)").alias("diversification_score"),
            F.round("risk_score", 4).cast("decimal(9,4)").alias("risk_score"),
            "risk_level",
            "computed_at",
        )
    )


def build_top_tickers(spark, orders, transactions, listings, run_id, computed_at):
    schema = StructType(
        [
            StructField("run_id", StringType(), False),
            StructField("ticker_rank", IntegerType(), False),
            StructField("listing_id", LongType(), False),
            StructField("ticker", StringType(), False),
            StructField("traded_quantity", LongType(), False),
            StructField("traded_notional", DecimalType(19, 4), False),
            StructField("order_count", IntegerType(), False),
            StructField("transaction_count", IntegerType(), False),
            StructField("computed_at", TimestampType(), False),
        ]
    )
    if orders.rdd.isEmpty():
        return empty_df(spark, schema)

    order_metrics = (
        orders.withColumn("order_notional", F.col("quantity") * F.col("price_per_unit"))
        .groupBy("listing_id")
        .agg(
            F.sum("quantity").cast("long").alias("traded_quantity"),
            F.sum("order_notional").alias("traded_notional"),
            F.count("*").cast("int").alias("order_count"),
        )
    )

    transaction_metrics = (
        transactions.alias("t")
        .join(orders.select(F.col("id").alias("order_id"), "listing_id"), "order_id", "inner")
        .groupBy("listing_id")
        .agg(F.count("*").cast("int").alias("transaction_count"))
    )

    ranked = (
        order_metrics.join(transaction_metrics, "listing_id", "left")
        .join(listings.select(F.col("id").alias("listing_id"), "ticker"), "listing_id", "left")
        .withColumn("transaction_count", F.coalesce(F.col("transaction_count"), F.lit(0)))
        .withColumn("ticker", F.coalesce(F.col("ticker"), F.concat(F.lit("LISTING-"), F.col("listing_id"))))
        .withColumn("ticker_rank", F.row_number().over(Window.orderBy(F.col("traded_notional").desc(), F.col("listing_id").asc())))
    )

    return (
        add_run_columns(ranked, run_id, computed_at)
        .select(
            "run_id",
            F.col("ticker_rank").cast("int"),
            F.col("listing_id").cast("long"),
            "ticker",
            F.col("traded_quantity").cast("long"),
            F.round("traded_notional", 4).cast("decimal(19,4)").alias("traded_notional"),
            F.col("order_count").cast("int"),
            F.col("transaction_count").cast("int"),
            "computed_at",
        )
    )


def build_client_segments(spark, portfolio_risk, orders, run_id, computed_at):
    schema = StructType(
        [
            StructField("run_id", StringType(), False),
            StructField("user_id", LongType(), False),
            StructField("cluster_id", IntegerType(), False),
            StructField("segment_label", StringType(), False),
            StructField("total_portfolio_value", DecimalType(19, 4), False),
            StructField("total_cost_basis", DecimalType(19, 4), False),
            StructField("unrealized_pnl", DecimalType(19, 4), False),
            StructField("holdings_count", IntegerType(), False),
            StructField("max_holding_percent", DecimalType(9, 4), False),
            StructField("order_count", IntegerType(), False),
            StructField("average_order_value", DecimalType(19, 4), False),
            StructField("buy_sell_ratio", DecimalType(19, 4), False),
            StructField("risk_score", DecimalType(9, 4), False),
            StructField("computed_at", TimestampType(), False),
        ]
    )
    if portfolio_risk.rdd.isEmpty():
        return empty_df(spark, schema)

    order_features = (
        orders.withColumn("order_value", F.col("quantity") * F.col("price_per_unit"))
        .groupBy("user_id")
        .agg(
            F.count("*").cast("int").alias("order_count"),
            F.avg("order_value").alias("average_order_value"),
            F.sum(F.when(F.col("direction") == "BUY", 1).otherwise(0)).alias("buy_count"),
            F.sum(F.when(F.col("direction") == "SELL", 1).otherwise(0)).alias("sell_count"),
        )
        .withColumn("buy_sell_ratio", F.col("buy_count") / F.greatest(F.col("sell_count"), F.lit(1)))
    )

    features = (
        portfolio_risk.alias("r")
        .join(order_features.alias("o"), F.col("r.user_id") == F.col("o.user_id"), "left")
        .select(
            F.col("r.user_id").cast("long").alias("user_id"),
            F.col("r.total_market_value").cast("double").alias("total_portfolio_value"),
            F.col("r.total_cost_basis").cast("double").alias("total_cost_basis"),
            F.col("r.unrealized_pnl").cast("double").alias("unrealized_pnl"),
            F.col("r.holdings_count").cast("int").alias("holdings_count"),
            F.col("r.max_holding_percent").cast("double").alias("max_holding_percent"),
            F.col("r.risk_score").cast("double").alias("risk_score"),
            F.coalesce(F.col("o.order_count"), F.lit(0)).cast("int").alias("order_count"),
            F.coalesce(F.col("o.average_order_value"), F.lit(0.0)).cast("double").alias("average_order_value"),
            F.coalesce(F.col("o.buy_sell_ratio"), F.lit(0.0)).cast("double").alias("buy_sell_ratio"),
        )
    )

    feature_columns = [
        "total_portfolio_value",
        "holdings_count",
        "max_holding_percent",
        "order_count",
        "average_order_value",
        "buy_sell_ratio",
        "risk_score",
    ]
    count = features.count()
    if count > 1:
        k = min(int(os.getenv("ANALYTICS_KMEANS_K", "4")), count)
        assembler = VectorAssembler(inputCols=feature_columns, outputCol="raw_features")
        scaler = StandardScaler(inputCol="raw_features", outputCol="features", withMean=True, withStd=True)
        assembled = assembler.transform(features)
        scaled_model = scaler.fit(assembled)
        scaled = scaled_model.transform(assembled)
        model = KMeans(k=k, seed=42, featuresCol="features", predictionCol="cluster_id").fit(scaled)
        clustered = model.transform(scaled)
    else:
        clustered = features.withColumn("cluster_id", F.lit(0))

    labelled = clustered.withColumn(
        "segment_label",
        F.when((F.col("order_count") <= 1) & (F.col("total_portfolio_value") < 10000), F.lit("LOW_ACTIVITY"))
        .when((F.col("max_holding_percent") >= 70) | (F.col("risk_score") >= 70), F.lit("CONCENTRATED_RISK"))
        .when((F.col("total_portfolio_value") >= 100000) | (F.col("order_count") >= 10), F.lit("HIGH_EXPOSURE_TRADER"))
        .otherwise(F.lit("DIVERSIFIED_INVESTOR")),
    )

    return (
        add_run_columns(labelled, run_id, computed_at)
        .select(
            "run_id",
            "user_id",
            F.col("cluster_id").cast("int"),
            "segment_label",
            F.round("total_portfolio_value", 4).cast("decimal(19,4)").alias("total_portfolio_value"),
            F.round("total_cost_basis", 4).cast("decimal(19,4)").alias("total_cost_basis"),
            F.round("unrealized_pnl", 4).cast("decimal(19,4)").alias("unrealized_pnl"),
            F.col("holdings_count").cast("int"),
            F.round("max_holding_percent", 4).cast("decimal(9,4)").alias("max_holding_percent"),
            F.col("order_count").cast("int"),
            F.round("average_order_value", 4).cast("decimal(19,4)").alias("average_order_value"),
            F.round("buy_sell_ratio", 4).cast("decimal(19,4)").alias("buy_sell_ratio"),
            F.round("risk_score", 4).cast("decimal(9,4)").alias("risk_score"),
            "computed_at",
        )
    )


def build_run_row(spark, run_id, started_at, completed_at, status, message):
    schema = StructType(
        [
            StructField("run_id", StringType(), False),
            StructField("job_name", StringType(), False),
            StructField("status", StringType(), False),
            StructField("started_at", TimestampType(), False),
            StructField("completed_at", TimestampType(), True),
            StructField("message", StringType(), True),
        ]
    )
    return spark.createDataFrame([(run_id, JOB_NAME, status, started_at, completed_at, message)], schema)


def run_analytics(spark, trading_options, market_options, run_id, started_at):
    portfolio, orders, transactions, listings = load_sources(spark, trading_options, market_options)
    computed_at = datetime.now(timezone.utc).replace(tzinfo=None)

    portfolio_risk = build_portfolio_risk(spark, portfolio, listings, run_id, computed_at)
    top_tickers = build_top_tickers(spark, orders, transactions, listings, run_id, computed_at)
    client_segments = build_client_segments(spark, portfolio_risk, orders, run_id, computed_at)

    write_table(portfolio_risk, trading_options, "analytics_portfolio_risk")
    write_table(top_tickers, trading_options, "analytics_top_tickers")
    write_table(client_segments, trading_options, "analytics_client_segments")

    completed_at = datetime.now(timezone.utc).replace(tzinfo=None)
    run_row = build_run_row(
        spark,
        run_id,
        started_at,
        completed_at,
        "COMPLETED",
        "Analytics run completed.",
    )
    write_table(run_row, trading_options, "analytics_job_runs")


def main():
    started_at = datetime.now(timezone.utc).replace(tzinfo=None)
    run_id = str(uuid.uuid4())
    spark = None
    trading_options = None

    try:
        spark = (
            SparkSession.builder.appName(JOB_NAME)
            .config("spark.sql.session.timeZone", "UTC")
            .getOrCreate()
        )

        trading_options = jdbc_options(
            getenv("TRADING_JDBC_URL"),
            getenv("TRADING_DB_USER"),
            getenv("TRADING_DB_PASSWORD"),
        )
        market_options = jdbc_options(
            getenv("MARKET_JDBC_URL"),
            getenv("MARKET_DB_USER"),
            getenv("MARKET_DB_PASSWORD"),
        )

        run_analytics(spark, trading_options, market_options, run_id, started_at)
    except Exception as exc:
        if spark is not None and trading_options is not None:
            try:
                completed_at = datetime.now(timezone.utc).replace(tzinfo=None)
                message = f"Analytics run failed: {exc}"[:512]
                failed_row = build_run_row(spark, run_id, started_at, completed_at, "FAILED", message)
                write_table(failed_row, trading_options, "analytics_job_runs")
            except Exception as write_exc:
                print(f"Failed to write analytics failure row: {write_exc}")
        raise
    finally:
        if spark is not None:
            spark.stop()


if __name__ == "__main__":
    main()
