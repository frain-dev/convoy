# Go Code Validation

Validate Go code to ensure it's ready for review by running linting and vetting tools.

## Steps

1. **Run `go vet`** on the entire codebase:
   ```bash
   go vet ./...
   ```

2. **Check formatting** with `gofmt`:
   ```bash
   gofmt -s -l .
   ```
   If any files are listed, they need formatting. Run `gofmt -s -w .` to fix.

3. **Run `golangci-lint`** with the project's configuration:
   ```bash
   golangci-lint run --config=.golangci.yml
   ```

4. **Report results** in a summary table:

   | Check | Result |
   |-------|--------|
   | `go vet ./...` | ✅ or ❌ |
   | `gofmt -s -l .` | ✅ or ❌ |
   | `golangci-lint run` | ✅ or ❌ (with issue count) |

5. If there are any issues, list them and offer to fix automatically fixable ones.

## Optional Arguments

- If a specific package path is provided (e.g., `./internal/batch_retries/...`), run validation only on that package.
- Without arguments, validate the entire codebase.

## Example Usage

```
/validate
/validate ./internal/batch_retries/...
/validate ./cmd/worker/...
```
