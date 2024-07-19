A Go toolkit for web content processing, analysis, and SEO optimization, offering utilities to efficiently extract titles, descriptions and favicons.

# Features

- Configurable Caching
- Timeout
- User-Agent

# Installation

```bash
go get github.com/devnyxie/katsuragi
```

# Usage

```go
import (
	. "katsuragi"
)

func main() {
  // Create a new fetcher with a timeout of 3 seconds and a cache capacity of 10
  fetcher := NewFetcher(
    &FetcherProps{
      Timeout:       3000, // 3 seconds
      CacheCap: 10, // 10 Network Requests will be cached
    },
  )

  defer fetcher.ClearCache()

  // Get website's title
  title, err := fetcher.GeTitle("https://www.example.com")
}
```

# Local Development

## Testing

```bash
go test -v
```
