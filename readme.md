A small, fast, and highly configurable Go package to fetch Favicon URLs from a given URL.

<!-- @ekalinin/github-markdown-toc -->
<!--ts-->
   * [Features](#features)
   * [Options Overview](#options-overview)
      * [Concurrency](#concurrency)
      * [MaxGoRoutines](#maxgoroutines)
      * [UserAgent](#useragent)
      * [Validate](#validate)
      * [MaxDepth](#maxdepth)
      * [Timeout](#timeout)
      * [ReturnFirst](#returnfirst)
      * [Quality](#quality)
      * [Binary](#binary)

<!-- Created by https://github.com/ekalinin/github-markdown-toc -->
<!-- Added by: devnyxie, at: Thu Jul 11 06:51:42 PM CEST 2024 -->

<!--te-->

**Table of Contents**

- [Features](#features)
  - [Concurrency](#concurrency)
- [Options Overview](#options-overview)
  - [MaxGoRoutines](#maxgoroutines)
  - [UserAgent](#useragent)
  - [Validate](#validate)
  - [MaxDepth](#maxdepth)
  - [Timeout](#timeout)
  - [ReturnFirst](#returnfirst)
  - [Quality](#quality)
  - [Binary](#binary)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Features

- Concurrency 🔀
- Validation ✅
- Advanced Search 🔍
- Custom UserAgent 🕵️‍♂️
- Timeout ⏰
- and more!

## Options Overview

<details>
  <summary><h3>Concurrency</h3></summary>

Type: `boolean`

Default: `false`

Enable or disable concurrent batch fetching.

> ⚠️ Enabling concurrency can increase CPU and memory usage because multiple threads or goroutines are active simultaneously. Simultaneous network requests can increase bandwidth usage, potentially leading to throttling or rate limiting by the server.

</details>

<details>
  <summary><h3>MaxGoRoutines</h3></summary>

Type: `number`

Default: `-1` (no limit)

Requirements:

- Concurrency: `true`

Set the maximum number of concurrent goroutines. This allows for fine-tuning the concurrency level for performance optimization and resource management. The value is set using the [SetLimit](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.SetLimit) method.

</details>

<details>
  <summary><h3>UserAgent</h3></summary>

Type: `string`

Default: `""`

Specify custom User-Agent headers for requests, which can help avoid server rate limiting and blocking mechanisms.

</details>

<details>
  <summary><h3>Validate</h3></summary>

Type: `boolean`

Default: `false`

Validate the favicon URL to ensure it's reachable.

</details>

<details>
  <summary><h3>MaxDepth</h3></summary>

Type: `number`

Default: `2`

Specify the depth level to stop fetching favicons:

    🟢 1: Parsing HTML homepage & checking for favicon rel tags (fast)
    🟡 2: Sending requests for common root and "/public" locations (mediocre)
    🔴 3: Checking, parsing, and searching for favicons in Web App Manifests (slow)

</details>

<details>
  <summary><h3>Timeout</h3></summary>

Type: `number`

Default: `5000`

Set a timeout (in milliseconds) for fetching favicon URLs.

</details>

<details>
  <summary><h3>ReturnFirst</h3></summary>

Type: `boolean`

Default: `false`

When enabled (true), only the first favicon found will be returned.

</details>

<details>
  <summary><h3>Quality</h3></summary>

Type: `number`

Default: `3`

Requirements:

- ReturnFirst: `false`

Values:

1. Low
2. Medium
3. High

Sorts the slice of favicons based on their quality. E.g., `3` will return the highest quality favicons first.

</details>

<details>
  <summary><h3>Binary</h3></summary>

Type: `boolean`

Default: `false`

Requirements:

- ReturnFirst: `true`

Works only when ReturnFirst is enabled. When enabled (true), the function returns the raw binary data of the image instead of its URL.

</details>
