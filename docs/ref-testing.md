# Go Testing – Reference
- Table‑driven tests in `<pkg>_test.go`
- `go test -race` for race detection
- `go test -coverprofile=cover.out ./...`
- `go tool cover -html=cover/profile.out` to view
- Benchmarks with `testing.B`
- Fuzzing with `go test -fuzz`
- Use `testify/assert` for richer asserts
- Document test purpose; aim for high coverage without over‑testing