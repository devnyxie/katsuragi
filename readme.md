# FaviJäger

A small, fast, and highly configurable Go package to fetch Favicon URLs from a given URL.

## Features

- **Concurrency** 🔀
- **Validation** ✅
- **Advanced Search** 🔍
- **UserAgent** 🕵️‍♂️
- **Timeout** ⏰
- and more!

## Documentation

### Options Overview

- **Concurrency**: `boolean` (default: `false`)

  - Enable or disable concurrent batch fetching.
    > ⚠️ Enabling concurrency can increase CPU and memory usage because multiple threads or goroutines are active simultaneously. Simultaneous network requests can increase bandwidth usage, potentially leading to throttling or rate limiting by the server.

- **MaxGoRoutines**: `number` (default: `-1` (no limit))

  - Set the maximum number of concurrent goroutines. This allows for fine-tuning the concurrency level for performance optimization and resource management. The value is set using the [SetLimit](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.SetLimit) method.

- **UserAgent**: `string` (default: `""`)

  - Specify custom User-Agent headers for requests, which can help avoid server rate limiting and blocking mechanisms.

- **Validate**: `boolean` (default: `false`)

  - Validate the favicon URL to ensure it's reachable.

- **maxDepth**: `number` (default: `1`)

  - Specify the depth level to stop fetching favicons:
    - 🟢 `1`: Parsing HTML homepage & checking for favicon rel tags (fast)
    - 🟡 `2`: Sending requests for common root and "/public" locations (mediocre)
    - 🔴 `3`: Checking, parsing, and searching for favicons in Web App Manifests (slow)

- **Timeout**: `number` (default: `5000`)

  - Set a timeout (in milliseconds) for fetching favicon URLs.

- **ReturnFirst**: `boolean` (default: `false`)

  - When enabled (`true`), only the first favicon found will be returned.

- **Quality**: `string` (default: `""`, values: `High`, `Low`)

  > ⚠️ The quality will be determined by the file extension. E.g., `favicon.ico` will be considered `Low` quality, and `favicon.png` will be considered `High` quality.

  1. High:

  - `png`
  - `webp`
  - `svg`

  2. Medium:

  - `jpg`
  - `jpeg`

  3.  Low:

  - `ico`

  > **Why we are not using the size or resolution to determine the quality?** <br/>
  > Because the size or resolution of the favicon is not always a good indicator of its quality. Furthermore, we would have to download the image to determine its size or resolution, which would be inefficient.

  > 💡 Please read the specification below before using this option, as the **ReturnFirst** option plays a crucial role here.

  - **ReturnFirst** disabled (false): Sorts the slice of favicons based on their quality. E.g., `High` will return the highest quality favicons first, and `Low` will return the lowest quality favicons first.
  - **ReturnFirst** enabled (true) AND **Concurrency** is enabled (true): Fetches all favicons concurrently and returns one favicon of the `highest`/`lowest` quality.
    > if Concurrency is disabled and ReturnFirst is enabled, the function will consider this option as `""` (no quality selected).

- **Binary**: `boolean` (default: `false`)

  - Works only when **ReturnFirst** is enabled. When enabled (`true`), the function returns the raw binary data of the image instead of its URL.
