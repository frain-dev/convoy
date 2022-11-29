### Installation

Install convoy-go with

```bash[terminal]
$ go get github.com/frain-dev/convoy-go
```

### Configuration

```go[example]
import (
    convoy "github.com/frain-dev/convoy-go"
)

  c := convoy.New(convoy.Options{
      APIKey: "your_api_key",
  })
```
