# katsuragi

![Go Build](https://github.com/devnyxie/katsuragi/actions/workflows/go.yml/badge.svg)
[![codecov](https://codecov.io/github/devnyxie/katsuragi/branch/main/graph/badge.svg?token=XFRMNJA858)](https://codecov.io/github/devnyxie/katsuragi)

A Go toolkit for web content processing, analysis, and SEO optimization, offering utilities to efficiently extract titles, descriptions and favicons.

> [!NOTE]
> Each method is thoroughly tested and optimized for performance, but the package is still in development and may contain unseen bugs. Please don't hesitate to report any issues you encounter!

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

**Table of Contents**

- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
  - [Title](#title)
  - [Description](#description)
  - [Favicons](#favicons)
- [Local Development](#local-development)
  - [Testing](#testing)
- [License](#license)

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

The GetTitle() function currently supports the following title meta tags:

- `<title>Title</title>`
- `<meta name="twitter:title" content="Title">`
- `<meta property="og:title" content="Title">`

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

The GetDescription() function currently supports the following description meta tags:

- `<meta name="description" content="Description">`
- `<meta name="twitter:description" content="Description">`
- `<meta property="og:description" content="Description">`

```go
...
  // Get website's description
  description, err := fetcher.GetDescription("https://www.example.com")
...
```

## Favicons

The GetFavicons() function currently supports the following favicon meta tags:

- `<link rel="icon" href="favicon.ico">`
- `<link rel="apple-touch-icon" href="favicon.png">`
- `<meta property="og:image" content="favicon.png">`
  > Open Graph image (`og:image`) will be used only if both `og:image:width` and `og:image:height` are present and equal, forming a square image.

```go
...
  // Get website's favicons
  favicons, err := fetcher.GetFavicons("https://www.example.com")
  // [https://www.example.com/favicon.ico, https://www.example.com/favicon.png]
...
```

# Local Development

## Testing

```bash
go test -v
```

## Code Coverage

```bash
# Generate coverage.out report, generate HTML report from coverage.out, and open the HTML report in the browser:
go test -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html && open coverage.html
```

# License

This project is licensed under the GNU General Public License (GPL). You can find the full text of the license [here](https://www.gnu.org/licenses/gpl-3.0.en.html).
