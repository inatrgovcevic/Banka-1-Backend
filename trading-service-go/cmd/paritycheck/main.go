// Command paritycheck calls the Java trading-service and the Go trading-service-go
// with the same requests and diffs the responses (status + normalized JSON).
// Exit code 1 on any difference. Endpoint specs come from a JSON file
// (-endpoints-file) or the built-in defaults. Mirrors market-service-go.
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
	javaBase := flag.String("java-base", "http://localhost:8088", "Java trading-service base URL")
	goBase := flag.String("go-base", "http://localhost:18088", "Go trading-service-go base URL")
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
		javaResp, err := callEndpoint(client, strings.TrimRight(*javaBase, "/"), spec, *token)
		if err != nil {
			fmt.Printf("[ERROR] %s java call failed: %v\n", spec.Name, err)
			differences++
			continue
		}
		goResp, err := callEndpoint(client, strings.TrimRight(*goBase, "/"), spec, *token)
		if err != nil {
			fmt.Printf("[ERROR] %s go call failed: %v\n", spec.Name, err)
			differences++
			continue
		}
		javaNorm := normalizeJSON(javaResp.body)
		goNorm := normalizeJSON(goResp.body)
		if javaResp.status != goResp.status || javaNorm != goNorm {
			fmt.Printf("[DIFF] %s\n", spec.Name)
			fmt.Printf("  java status=%d body=%s\n", javaResp.status, javaNorm)
			fmt.Printf("  go   status=%d body=%s\n", goResp.status, goNorm)
			differences++
			continue
		}
		successes++
		fmt.Printf("[OK] %s\n", spec.Name)
	}
	fmt.Printf("\nSummary: %d ok, %d diffs over %d endpoints (java=%s go=%s)\n", successes, differences, len(specs), *javaBase, *goBase)
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

// defaultSpecs are the only endpoints trading-service-go serves at P0 (scaffold):
// the public actuator endpoints. Domain specs are added to the endpoints JSON
// files as each phase lands.
func defaultSpecs() []endpointSpec {
	return []endpointSpec{
		{Name: "actuator-info", Method: http.MethodGet, Path: "/actuator/info"},
		{Name: "actuator-liveness", Method: http.MethodGet, Path: "/actuator/health/liveness"},
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

// normalizeValue recursively sorts map keys and drops volatile fields so
// trailing-zero/scale and timestamp noise does not cause false diffs. Value
// differences still fail the comparison.
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
