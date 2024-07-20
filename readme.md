A Go toolkit for web content processing, analysis, and SEO optimization, offering utilities to efficiently extract titles, descriptions and favicons.

> [!NOTE]
> Each method is thoroughly tested and optimized for performance, but the package is still in development and may contain unseen bugs. Please don't hesitate to report any issues you encounter!

# Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Title](#title)
  - [Description](#description)
  - [Favicons](#favicons)
- [Local Development](#local-development)
  - [Testing](#testing)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Features

- LRU Caching
- Timeout
- User-Agent

# Installation

```bash
go get github.com/devnyxie/katsuragi
```

# Usage

## Title

```go
import (
	. "github.com/devnyxie/katsuragi"
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

## Description

```go
...
  // Get website's description
  description, err := fetcher.GetDescription("https://www.example.com")
...
```

## Favicons

```go
...
  // Get website's favicons
  favicons, err := fetcher.GetFavicons("https://www.example.com")
...
```

# Local Development

## Testing

```bash
go test -v
```
