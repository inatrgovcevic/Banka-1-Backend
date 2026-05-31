package http

import (
	"net/http"

	"banka1/market-service-go/internal/auth"
	"banka1/market-service-go/internal/platform"
)

func registerRoutes(handle func(string, string, http.Handler), cfg platform.Config, app *App, jwtService *auth.JWTService, withAuth func(string, http.Handler) http.Handler) {
	handlers := NewHandlers(cfg, app)
	add := func(method, path string, handler http.Handler) {
		handle(method, path, withAuth(path, handler))
	}

	add(http.MethodGet, "/stocks/price-feed/current", http.HandlerFunc(handlers.GetCurrentPriceFeed))
	add(http.MethodGet, "/stocks/price-feed/single/", http.HandlerFunc(handlers.GetSinglePriceFeed))
	add(http.MethodGet, "/api/stock-exchanges", jwtService.Middleware(auth.RequireRoles("CLIENT_BASIC", "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ListStockExchanges))))
	add(http.MethodGet, "/api/stock-exchanges/", jwtService.Middleware(http.HandlerFunc(handlers.StockExchangeSubroutes)))
	add(http.MethodGet, "/api/listings/stocks", jwtService.Middleware(auth.RequireRoles("CLIENT_BASIC", "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ListStockListings))))
	add(http.MethodGet, "/api/listings/futures", jwtService.Middleware(auth.RequireRoles("CLIENT_BASIC", "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ListFuturesListings))))
	add(http.MethodGet, "/api/listings/forex", jwtService.Middleware(auth.RequireRoles("BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ListForexListings))))
	add(http.MethodGet, "/api/listings/", jwtService.Middleware(auth.RequireRoles("CLIENT_BASIC", "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ListingSubroutes))))
	add(http.MethodPost, "/api/internal/listings/", jwtService.Middleware(auth.RequireRoles("SERVICE")(http.HandlerFunc(handlers.InternalListingSubroutes))))
	add(http.MethodGet, "/stocks/info", http.HandlerFunc(handlers.StockInfo))
	add(http.MethodGet, "/stocks/exchange/info", jwtService.Middleware(auth.RequireRoles("CLIENT_BASIC", "BASIC", "AGENT", "SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ExchangeInfo))))
	add(http.MethodGet, "/exchange/info", http.HandlerFunc(handlers.ExchangeInfo))
	add(http.MethodGet, "/rates", jwtService.Middleware(auth.RequireRoles("ADMIN", "SERVICE", "CLIENT_BASIC")(http.HandlerFunc(handlers.GetRates))))
	add(http.MethodGet, "/rates/", jwtService.Middleware(auth.RequireRoles("ADMIN", "SERVICE", "CLIENT_BASIC")(http.HandlerFunc(handlers.GetRateByCurrency))))
	add(http.MethodGet, "/calculate", jwtService.Middleware(auth.RequireRoles("ADMIN", "SERVICE")(http.HandlerFunc(handlers.Calculate))))
	add(http.MethodGet, "/internal/calculate/no-commission", http.HandlerFunc(handlers.CalculateNoCommission))
	add(http.MethodPost, "/rates/fetch", jwtService.Middleware(auth.RequireRoles("ADMIN", "SERVICE")(http.HandlerFunc(handlers.FetchRates))))
	add(http.MethodPost, "/admin/stock-exchanges/import", jwtService.Middleware(auth.RequireRoles("SUPERVISOR", "ADMIN", "SERVICE")(http.HandlerFunc(handlers.ImportStockExchanges))))
	add(http.MethodPost, "/admin/stocks/refresh-all", jwtService.Middleware(auth.RequireRoles("ADMIN", "SUPERVISOR")(http.HandlerFunc(handlers.RefreshAllStocks))))
	add(http.MethodPost, "/admin/stocks/", jwtService.Middleware(auth.RequireRoles("ADMIN", "SUPERVISOR")(http.HandlerFunc(handlers.StockAdminSubroutes))))
}
