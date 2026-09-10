# Publishing youspot-go

Go modules have no registry to upload to: a module is published the moment a public git repository with a `go.mod` at its root carries a semver tag. The module path is `github.com/OnStartups/youspot-go`, so the public repository must live at exactly that address. It is a snapshot of this folder, the same way `OnStartups/youspot-agent-tools` mirrors `packages/youspot-agent-tools`.

## One-time setup

```bash
gh repo create OnStartups/youspot-go --public \
  --description "Official Go client for YouSpot, the AI-native personal CRM" \
  --homepage https://youspot.com/docs
```

## Release

From the monorepo root, on `main`, after the change has merged:

```bash
# 1. Bump Version in packages/youspot-go/youspot.go (the User-Agent follows it).
go vet -C packages/youspot-go ./... && go test -C packages/youspot-go ./...
# 2. Push the snapshot to the public repository and tag it.
git subtree split --prefix packages/youspot-go -b youspot-go-release
git push git@github.com:OnStartups/youspot-go.git youspot-go-release:main
git tag v0.1.0 youspot-go-release && git push git@github.com:OnStartups/youspot-go.git v0.1.0
git branch -D youspot-go-release
# 3. Warm the module proxy so pkg.go.dev has the page before an agent looks for it.
curl -s https://proxy.golang.org/github.com/!on!startups/youspot-go/@v/v0.1.0.info
open https://pkg.go.dev/github.com/OnStartups/youspot-go
```

pkg.go.dev shows the README, the license and the repository link; the README links back to youspot.com/docs, which is how a discovery scanner confirms the module is the official SDK.

## After the first release

Add the module to `PACKAGES` in `frontend/lib/agent/facts.ts` (pkg.go.dev URL) and to the install block in `frontend/lib/agent/docs/overview.ts`, then re-run `bun run sync:agent-tools`.
