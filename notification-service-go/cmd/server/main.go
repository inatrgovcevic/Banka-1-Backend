package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"Banka1Back/notification-service-go/internal/config"
	"Banka1Back/notification-service-go/internal/messaging"
	"Banka1Back/notification-service-go/internal/service"
	"Banka1Back/notification-service-go/internal/smtp"
	"Banka1Back/notification-service-go/internal/store"
	"Banka1Back/notification-service-go/internal/template"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		log.Println("configuration load failed:", err)
		os.Exit(1)
	}

	db, err := config.NewDatabasePool(cfg.DB)
	if err != nil {
		log.Println("database connection failed:", err)
	} else {
		defer db.Close()
		log.Println("database connected")

		err = store.RunMigrations(context.Background(), db, "migrations")
		if err != nil {
			log.Println("database migrations failed:", err)
		} else {
			log.Println("database migrations applied")
		}
	}

	deliveryStore := store.NewNotificationDeliveryStore(db)

	registry := template.NewDefaultTemplateRegistry(nil)
	renderer := template.NewRenderer(registry)
	smtpSender := smtp.NewSender(cfg.SMTP)

	schedulerInterval := time.Duration(cfg.Retry.SchedulerIntervalMs) * time.Millisecond
	scheduler := service.NewTickerRetryScheduler(deliveryStore, schedulerInterval, slog.Default())

	notificationService := service.NewNotificationService(
		deliveryStore,
		renderer,
		smtpSender,
		scheduler,
		cfg.Retry,
		slog.Default(),
	)

	scheduler.SetService(notificationService)
	go scheduler.Start(context.Background())

	dispatcher := messaging.NewDispatcher(notificationService, slog.Default())
	consumer := messaging.NewConsumer(*cfg, dispatcher, slog.Default())

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := fmt.Sprintf("%d", cfg.Server.HTTPPort)
	go func() {
		log.Println("notification-service HTTP started on port " + port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			log.Println("HTTP server error:", err)
		}
	}()

	log.Println("starting AMQP consumer, queue:", cfg.Rabbit.Queue)
	if err := consumer.Run(context.Background()); err != nil {
		log.Fatal("consumer exited:", err)
	}
}
