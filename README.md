# youspot-go

The official Go client for [YouSpot](https://youspot.com), the AI-native personal CRM. Standard library only, Go 1.21 or newer.

```bash
go get github.com/OnStartups/youspot-go
```

Everything except `CallTool` works with no credential. Every YouSpot tool reads one person's own contacts, companies, notes and files, so a token always belongs to a person: mint one at [youspot.com/user/mcp](https://youspot.com/user/mcp) and set `YOUSPOT_TOKEN`, or follow the OAuth 2.1 flow in [auth.md](https://youspot.com/auth.md).

```go
package main

import (
	"context"
	"fmt"

	youspot "github.com/OnStartups/youspot-go"
)

func main() {
	ctx := context.Background()
	c := youspot.New()

	answer, err := c.Ask(ctx, "how do I connect an MCP client", 3)
	if err != nil {
		panic(err)
	}
	fmt.Println(answer["answer"])

	tools, _ := c.Tools(ctx)
	fmt.Println(string(tools))

	demo, _ := c.CallSandboxTool(ctx, "search_graph_objects", map[string]any{"query": "consulting"})
	fmt.Println(string(demo))

	summary, err := c.CallTool(ctx, "get_connections_summary", nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(summary))
}
```

| Method                              | What it does                                           | Credential      |
| ----------------------------------- | ------------------------------------------------------ | --------------- |
| `Index`                             | Every endpoint, and what each one costs in credentials | none            |
| `Ask`                               | A question about YouSpot, answered from its own pages  | none            |
| `SearchDocs`, `ReadDoc`, `ListDocs` | The developer docs over the documentation MCP server   | none            |
| `ReadMarkdown`                      | Any public page as markdown                            | none            |
| `Tools`                             | The tools on the product MCP server                    | none            |
| `CallTool`                          | One tool, as the member whose token the client holds   | `YOUSPOT_TOKEN` |
| `SandboxTools`, `CallSandboxTool`   | The read tools over a demo account                     | none            |
| `Member`, `Directory`               | Public profiles and the public directory               | none            |
| `Batch`                             | Up to 20 reads in one round trip                       | none            |
| `Identity`                          | What identity the caller can hold here                 | optional        |

Every failure is a `*youspot.Error` carrying the HTTP status, the stable error code and the URL. `YOUSPOT_BASE_URL` overrides the origin.

The machine descriptions this client follows: [openapi.json](https://youspot.com/openapi.json), [the MCP server card](https://youspot.com/.well-known/mcp/server-card.json), [agents.md](https://youspot.com/agents.md) and [llms.txt](https://youspot.com/llms.txt). Developer docs: <https://youspot.com/docs>.

MIT licensed. Support: support@youspot.com.
