package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type endpointSpec struct {
	Name    string            `json:"name"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func main() {
	baselineBase := flag.String("baseline-base", "http://localhost:8085", "Baseline market-service base URL")
	goBase := flag.String("go-base", "http://localhost:18085", "Go market-service base URL")
	token := flag.String("token", "", "Optional bearer token for authenticated endpoints")
	file := flag.String("endpoints-file", "", "Optional JSON file with endpoint definitions")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP timeout")
	flag.Parse()

	specs, err := loadSpecs(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load endpoint specs:", err)
		os.Exit(1)
	}
	client := &http.Client{Timeout: *timeout}
	differences := 0
	successes := 0
	for _, spec := range specs {
		baselineResp, err := callEndpoint(client, strings.TrimRight(*baselineBase, "/"), spec, *token)
		if err != nil {
			fmt.Printf("[ERROR] %s baseline call failed: %v\n", spec.Name, err)
			differences++
			continue
		}
		goResp, err := callEndpoint(client, strings.TrimRight(*goBase, "/"), spec, *token)
		if err != nil {
			fmt.Printf("[ERROR] %s go call failed: %v\n", spec.Name, err)
			differences++
			continue
		}
		baselineNorm := normalizeJSON(baselineResp.body)
		goNorm := normalizeJSON(goResp.body)
		if baselineResp.status != goResp.status || baselineNorm != goNorm {
			fmt.Printf("[DIFF] %s\n", spec.Name)
			fmt.Printf("  baseline status=%d body=%s\n", baselineResp.status, baselineNorm)
			fmt.Printf("  go       status=%d body=%s\n", goResp.status, goNorm)
			differences++
			continue
		}
		successes++
		fmt.Printf("[OK] %s\n", spec.Name)
	}
	fmt.Printf("\nSummary: %d ok, %d diffs over %d endpoints (baseline=%s go=%s)\n", successes, differences, len(specs), *baselineBase, *goBase)
	if differences > 0 {
		os.Exit(1)
	}
}

type responseSnapshot struct {
	status int
	body   []byte
}

func callEndpoint(client *http.Client, base string, spec endpointSpec, token string) (responseSnapshot, error) {
	var body io.Reader
	if spec.Body != "" {
		body = strings.NewReader(spec.Body)
	}
	req, err := http.NewRequest(spec.Method, base+spec.Path, body)
	if err != nil {
		return responseSnapshot{}, err
	}
	for key, value := range spec.Headers {
		req.Header.Set(key, value)
	}
	if spec.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return responseSnapshot{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return responseSnapshot{}, err
	}
	return responseSnapshot{status: resp.StatusCode, body: raw}, nil
}

func loadSpecs(path string) ([]endpointSpec, error) {
	if strings.TrimSpace(path) == "" {
		return defaultSpecs(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var specs []endpointSpec
	if err := json.Unmarshal(raw, &specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func defaultSpecs() []endpointSpec {
	return []endpointSpec{
		{Name: "actuator-info", Method: http.MethodGet, Path: "/actuator/info"},
		{Name: "stock-info", Method: http.MethodGet, Path: "/stocks/info"},
		{Name: "exchange-info", Method: http.MethodGet, Path: "/exchange/info"},
		{Name: "price-feed-single", Method: http.MethodGet, Path: "/stocks/price-feed/single/AAPL"},
		{Name: "price-feed-current", Method: http.MethodGet, Path: "/stocks/price-feed/current?tickers=AAPL,MSFT"},
		{Name: "internal-calculate-no-commission", Method: http.MethodGet, Path: "/internal/calculate/no-commission?fromCurrency=USD&toCurrency=USD&amount=100.00"},
	}
}

func normalizeJSON(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	normalized := normalizeValue(value)
	out, err := json.Marshal(normalized)
	if err != nil {
		return string(trimmed)
	}
	return string(out)
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			if key == "timestamp" || key == "lastRefresh" || key == "createdAt" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(keys))
		for _, key := range keys {
			out[key] = normalizeValue(typed[key])
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeValue(item))
		}
		return out
	default:
		return typed
	}
}
