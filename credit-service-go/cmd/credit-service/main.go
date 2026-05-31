package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"Banka1Back/credit-service-go/internal/api"
	"Banka1Back/credit-service-go/internal/client"
	"Banka1Back/credit-service-go/internal/config"
	"Banka1Back/credit-service-go/internal/messaging"
	"Banka1Back/credit-service-go/internal/service"
	"Banka1Back/credit-service-go/internal/store"
)

func main() {
	db, err := config.NewDatabasePool()
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

	loanRequestStore := store.NewLoanRequestStore(db)
	loanStore := store.NewLoanStore(db)
	installmentStore := store.NewInstallmentStore(db)
	accountClient := client.NewAccountClient()
	exchangeClient := client.NewExchangeClient()
	clientServiceClient := client.NewClientServiceClient()

	rabbitClient, err := messaging.NewRabbitClient()
	if err != nil {
		log.Println("rabbitmq connection failed, notifications disabled:", err)
	} else {
		defer rabbitClient.Close()
		log.Println("rabbitmq connected")
	}

	loanService := service.NewLoanService(
		loanRequestStore,
		loanStore,
		installmentStore,
		accountClient,
		rabbitClient,
		exchangeClient,
		clientServiceClient,
	)

	service.StartInstallmentScheduler(context.Background(), loanService)

	mux := http.NewServeMux()
	handler := api.NewLoanHandler(loanService)

	mux.HandleFunc("/health", handler.Health)
	mux.HandleFunc("/api/loans/client", handler.FindClientLoans)
	mux.HandleFunc("/api/loans/all", handler.FindAllLoans)
	mux.HandleFunc("/api/loans/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handler.GetLoanInfo(w, r)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/loans/requests", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handler.CreateLoanRequest(w, r)
		case http.MethodGet:
			handler.FindAllLoanRequests(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/loans/requests/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			handler.ConfirmLoanRequest(w, r)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})
	port := "8085"
	fmt.Println("credit-service started on port " + port)

	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
