# Contributing to rstmdb-go

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

```bash
git clone https://github.com/rstmdb/rstmdb-go.git
cd rstmdb-go
make          # runs fmt, vet, build, test -race
```

Requires Go 1.21+.

## Making Changes

1. Fork the repo and create a branch from `main`.
2. Write your code. No external dependencies — standard library only.
3. Add or update tests for your changes.
4. Run the full check:
   ```bash
   make check    # fmt + vet + build + test -race
   ```
5. Open a pull request.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep the public API surface small. Export only what users need.
- Use table-driven tests where appropriate.
- No external test dependencies — use the standard `testing` package.

## Tests

All tests use a mock TCP server that speaks the RCPX binary protocol. No running rstmdb server is needed.

```bash
make test           # quick run
make test-verbose   # with -v
make test-race      # with race detector
make test-cover     # coverage report
```

## Pull Request Guidelines

- Keep PRs focused. One feature or fix per PR.
- Write a clear description of what changed and why.
- Make sure all tests pass and the race detector is clean.
- Update examples if you change the public API.

## Reporting Issues

Open an issue at https://github.com/rstmdb/rstmdb-go/issues with:

- What you expected to happen
- What actually happened
- Steps to reproduce
- Go version (`go version`)

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
