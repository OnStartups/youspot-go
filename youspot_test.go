package youspot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type seen struct {
	method, path, auth, accept, body string
}

func serve(t *testing.T, status int, contentType, body string) (*Client, *seen) {
	t.Helper()
	got := &seen{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		*got = seen{r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"), r.Header.Get("Accept"), string(raw)}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Client{BaseURL: server.URL}, got
}

func TestCallToolSendsTheBearerAndUnwrapsTheResult(t *testing.T) {
	c, got := serve(t, 200, "application/json", `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"3 people"}]}}`)
	c.Token = "tok"
	result, err := c.CallTool(context.Background(), "get_connections_summary", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/mcp" || got.auth != "Bearer tok" {
		t.Fatalf("sent %s with %q", got.path, got.auth)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil {
		t.Fatal(err)
	}
	params := body["params"].(map[string]any)
	if params["name"] != "get_connections_summary" || params["arguments"] == nil {
		t.Fatalf("params were %v", params)
	}
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil || parsed["content"] == nil {
		t.Fatalf("result was %s", result)
	}
}

func TestCallToolWithoutATokenNeverReachesTheServer(t *testing.T) {
	c, got := serve(t, 200, "application/json", `{}`)
	_, err := c.CallTool(context.Background(), "search_graph_objects", nil)
	var e *Error
	if !errors.As(err, &e) || e.Code != "no_token" {
		t.Fatalf("got %v", err)
	}
	if got.path != "" {
		t.Fatalf("a request was sent: %s", got.path)
	}
}

func TestAJsonRpcErrorBecomesAnError(t *testing.T) {
	c, _ := serve(t, 200, "application/json", `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	_, err := c.SandboxTools(context.Background())
	var e *Error
	if !errors.As(err, &e) || e.Code != "-32601" || e.Message != "Method not found" {
		t.Fatalf("got %v", err)
	}
}

func TestAnHttpErrorCarriesTheStableCode(t *testing.T) {
	c, _ := serve(t, 404, "application/json", `{"success":false,"error":"not_found","error_description":"No such member."}`)
	_, err := c.Member(context.Background(), "nobody")
	var e *Error
	if !errors.As(err, &e) || e.Status != 404 || e.Code != "not_found" || e.Message != "No such member." {
		t.Fatalf("got %v", err)
	}
}

func TestReadMarkdownAsksForTheTwin(t *testing.T) {
	c, got := serve(t, 200, "text/markdown", "# Docs\n")
	body, err := c.ReadMarkdown(context.Background(), "docs/mcp")
	if err != nil || body != "# Docs\n" {
		t.Fatalf("got %q, %v", body, err)
	}
	if got.path != "/docs/mcp.md" || got.accept != "text/markdown" {
		t.Fatalf("asked for %s with Accept %s", got.path, got.accept)
	}
}

func TestBatchPostsTheItemsAsRequests(t *testing.T) {
	c, got := serve(t, 200, "application/json", `{"object":"batch","count":1,"responses":[{"id":"me","status":200,"body":{}}]}`)
	out, err := c.Batch(context.Background(), []BatchItem{{ID: "me", Path: "/api/human/dharmesh"}})
	if err != nil || out["count"].(float64) != 1 {
		t.Fatalf("got %v, %v", out, err)
	}
	if got.method != http.MethodPost || got.path != "/api/batch" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	var body map[string][]map[string]any
	if err := json.Unmarshal([]byte(got.body), &body); err != nil || body["requests"][0]["path"] != "/api/human/dharmesh" {
		t.Fatalf("body was %s", got.body)
	}
}
