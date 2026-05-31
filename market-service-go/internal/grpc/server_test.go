package grpc

import (
	"context"
	"net"
	"testing"

	"banka1/market-service-go/internal/fx"
	httpapi "banka1/market-service-go/internal/http"
	"banka1/market-service-go/internal/platform"
	marketv1 "banka1/market-service-go/proto/market/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestConvertRPC(t *testing.T) {
	app := &httpapi.App{
		Config: platform.Config{FX: platform.FXConfig{CommissionPercentage: "0.70"}},
	}
	app.FXService = fx.NewService(app.Config, &fx.Repository{})
	app.PriceFeed = nil
	server := NewServer(app)
	listener := bufconn.Listen(1024 * 1024)
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	defer conn.Close()

	client := marketv1.NewMarketServiceClient(conn)
	resp, err := client.Convert(context.Background(), &marketv1.ConvertRequest{
		FromCurrency:      "USD",
		ToCurrency:        "USD",
		Amount:            "100.00",
		Date:              "2026-05-26",
		IncludeCommission: false,
	})
	if err != nil {
		t.Fatalf("invoke convert: %v", err)
	}
	if resp.GetJsonPayload() == "" {
		t.Fatal("expected non-empty grpc response")
	}
}
