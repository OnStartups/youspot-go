// Package youspot is the official Go client for YouSpot, the AI-native
// personal CRM: the same surface as the youspot npm and PyPI packages.
//
// Everything except CallTool works with no credential. CallTool needs a
// member's token, because every tool reads one person's own CRM; the
// sandbox runs the read tools against a demo account with no credential.
package youspot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const Version = "0.1.0"

// DefaultBaseURL is where youspot.com answers; YOUSPOT_BASE_URL overrides it.
const DefaultBaseURL = "https://youspot.com"

// Error is what every failed call returns: the HTTP status when there was
// one, the stable error code when the body carried one, and the URL.
type Error struct {
	Message string
	Status  int
	Code    string
	URL     string
}

func (e *Error) Error() string { return e.Message }

// Client talks to one YouSpot origin. The zero value is usable.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// New returns a client for YOUSPOT_BASE_URL (or youspot.com), reading a
// member token from YOUSPOT_TOKEN when one is set.
func New() *Client {
	base := strings.TrimRight(os.Getenv("YOUSPOT_BASE_URL"), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{BaseURL: base, Token: os.Getenv("YOUSPOT_TOKEN")}
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *Client) request(ctx context.Context, method, path string, body any, accept string, token string, out any) error {
	target := c.base() + path
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "youspot-go/"+Version)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.httpClient().Do(req)
	if err != nil {
		return &Error{Message: target + " could not be reached: " + err.Error(), URL: target}
	}
	defer res.Body.Close()
	text, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if accept != "application/json" {
		if res.StatusCode >= 400 {
			return &Error{Message: fmt.Sprintf("%s answered %d", target, res.StatusCode), Status: res.StatusCode, URL: target}
		}
		*(out.(*string)) = string(text)
		return nil
	}
	var parsed map[string]any
	if len(text) > 0 {
		if err := json.Unmarshal(text, &parsed); err != nil {
			return &Error{Message: fmt.Sprintf("%s answered %d with something other than JSON", target, res.StatusCode), Status: res.StatusCode, URL: target}
		}
	}
	if res.StatusCode >= 400 {
		return apiError(parsed, res.StatusCode, target)
	}
	return json.Unmarshal(text, out)
}

func apiError(parsed map[string]any, status int, target string) *Error {
	e := &Error{Status: status, URL: target, Message: fmt.Sprintf("%s answered %d", target, status)}
	switch v := parsed["error"].(type) {
	case string:
		e.Code = v
		e.Message = v
	case map[string]any:
		if code, ok := v["code"].(string); ok {
			e.Code = code
		}
		if msg, ok := v["message"].(string); ok {
			e.Message = msg
		}
	}
	if desc, ok := parsed["error_description"].(string); ok && desc != "" {
		e.Message = desc
	}
	return e
}

type rpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) rpc(ctx context.Context, path, method string, params any, token string) (json.RawMessage, error) {
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	var envelope rpcEnvelope
	if err := c.request(ctx, http.MethodPost, path, body, "application/json", token, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, &Error{Message: envelope.Error.Message, Code: strconv.Itoa(envelope.Error.Code), URL: c.base() + path}
	}
	return envelope.Result, nil
}

// Index is /v1: every endpoint, and what each one costs in credentials.
func (c *Client) Index(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	return out, c.request(ctx, http.MethodGet, "/v1", nil, "application/json", "", &out)
}

// Ask answers a question about YouSpot from its own public pages.
func (c *Client) Ask(ctx context.Context, query string, limit int) (map[string]any, error) {
	q := url.Values{"query": {query}, "limit": {strconv.Itoa(limit)}}
	var out map[string]any
	return out, c.request(ctx, http.MethodGet, "/ask?"+q.Encode(), nil, "application/json", "", &out)
}

// SearchDocs searches the developer docs over the documentation MCP server.
func (c *Client) SearchDocs(ctx context.Context, query string, limit int) (json.RawMessage, error) {
	return c.rpc(ctx, "/mcp/docs", "tools/call", map[string]any{
		"name": "search_docs", "arguments": map[string]any{"query": query, "limit": limit},
	}, "")
}

// ReadDoc reads one documentation page over the documentation MCP server.
func (c *Client) ReadDoc(ctx context.Context, path string) (json.RawMessage, error) {
	return c.rpc(ctx, "/mcp/docs", "tools/call", map[string]any{
		"name": "read_doc", "arguments": map[string]any{"path": path},
	}, "")
}

// ListDocs lists every documentation page.
func (c *Client) ListDocs(ctx context.Context) (json.RawMessage, error) {
	return c.rpc(ctx, "/mcp/docs", "tools/call", map[string]any{
		"name": "list_docs", "arguments": map[string]any{},
	}, "")
}

// ReadMarkdown reads any public page as markdown, for example "/docs/mcp".
func (c *Client) ReadMarkdown(ctx context.Context, path string) (string, error) {
	clean := path
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	target := "/index.md"
	if clean != "/" {
		target = strings.TrimSuffix(clean, ".md") + ".md"
	}
	var out string
	return out, c.request(ctx, http.MethodGet, target, nil, "text/markdown", "", &out)
}

// Tools lists the tools on the product MCP server. No credential.
func (c *Client) Tools(ctx context.Context) (json.RawMessage, error) {
	return c.rpc(ctx, "/mcp/v1", "tools/list", nil, "")
}

// CallTool calls one tool on the product MCP server as the member whose
// token the client holds.
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	if c.Token == "" {
		return nil, &Error{Code: "no_token", Message: fmt.Sprintf(
			"Calling %s needs a token, because every tool reads one person's own CRM. Mint one at %s/user/mcp and set YOUSPOT_TOKEN, or read %s/auth.md for the OAuth flow.",
			name, c.base(), c.base())}
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	return c.rpc(ctx, "/mcp/v1", "tools/call", map[string]any{"name": name, "arguments": arguments}, c.Token)
}

// SandboxTools lists the tools on the sandbox MCP server.
func (c *Client) SandboxTools(ctx context.Context) (json.RawMessage, error) {
	return c.rpc(ctx, "/mcp/sandbox", "tools/list", nil, "")
}

// CallSandboxTool runs a read tool against the demo account. No credential.
func (c *Client) CallSandboxTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	if arguments == nil {
		arguments = map[string]any{}
	}
	return c.rpc(ctx, "/mcp/sandbox", "tools/call", map[string]any{"name": name, "arguments": arguments}, "")
}

// Identity is /agent/identity: what identity the caller can hold here.
func (c *Client) Identity(ctx context.Context, identityType string) (map[string]any, error) {
	if identityType == "" {
		identityType = "anonymous"
	}
	var out map[string]any
	return out, c.request(ctx, http.MethodPost, "/agent/identity", map[string]any{"type": identityType}, "application/json", c.Token, &out)
}

// Member reads one public member profile.
func (c *Client) Member(ctx context.Context, username string) (map[string]any, error) {
	var out map[string]any
	return out, c.request(ctx, http.MethodGet, "/api/human/"+url.PathEscape(username), nil, "application/json", "", &out)
}

// Directory reads one page of the public member directory.
func (c *Client) Directory(ctx context.Context, limit, page int) (map[string]any, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}, "page": {strconv.Itoa(page)}}
	var out map[string]any
	return out, c.request(ctx, http.MethodGet, "/api/network/members?"+q.Encode(), nil, "application/json", "", &out)
}

// BatchItem is one read inside a Batch call.
type BatchItem struct {
	ID    string            `json:"id,omitempty"`
	Path  string            `json:"path"`
	Query map[string]string `json:"query,omitempty"`
}

// Batch runs up to 20 reads in one round trip; each answer carries its own
// status and body.
func (c *Client) Batch(ctx context.Context, items []BatchItem) (map[string]any, error) {
	var out map[string]any
	return out, c.request(ctx, http.MethodPost, "/api/batch", map[string]any{"requests": items}, "application/json", c.Token, &out)
}
