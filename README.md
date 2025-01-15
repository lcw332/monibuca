<!-- Improved compatibility of back to top link -->
<a id="readme-top"></a>

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]

<!-- PROJECT LOGO -->
<br />
<div align="center">
  <h1 align="center">Monibuca v5</h1>

  <p align="center">
    A highly scalable high-performance streaming server development framework developed purely in Go
    <br />
    <a href="https://github.com/Monibuca/v5/wiki"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://github.com/Monibuca/v5/issues">Report Bug</a>
    ·
    <a href="https://github.com/Monibuca/v5/issues">Request Feature</a>
  </p>
</div>

<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li><a href="#about">About</a></li>
    <li><a href="#getting-started">Getting Started</a></li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#build-tags">Build Tags</a></li>
    <li><a href="#monitoring">Monitoring</a></li>
    <li><a href="#plugin-development">Plugin Development</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
  </ol>
</details>

## About

Monibuca is a powerful streaming server framework written entirely in Go. It's designed to be:

- 🚀 **High Performance** - Built for maximum efficiency and speed
- 📦 **Modular** - Plugin-based architecture for easy extensibility
- 🔧 **Flexible** - Highly configurable to meet various streaming needs
- 💪 **Scalable** - Designed to handle large-scale deployments

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Getting Started

### Prerequisites

- Go 1.18 or higher
- Basic understanding of streaming protocols

### Installation

1. Create a new Go project
2. Add Monibuca as a dependency:
   ```sh
   go get m7s.live/v5
   ```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Usage

Here's a basic example to get you started:

```go
package main

import (
	"context"

	"m7s.live/v5"
	_ "m7s.live/v5/plugin/debug"
	_ "m7s.live/v5/plugin/flv"
	_ "m7s.live/v5/plugin/rtmp"
)

func main() {
	m7s.Run(context.Background(), "config.yaml")
}
```

For more examples, check out the [example directory](./example).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Build Tags

The following build tags can be used to customize your build:

| Build Tag | Description |
|-----------|-------------|
| disable_rm | Disables the memory pool |
| sqlite | Enables the sqlite DB |  
| sqliteCGO | Enables the sqlite cgo version DB |
| mysql | Enables the mysql DB |
| postgres | Enables the postgres DB |
| duckdb | Enables the duckdb DB |
| taskpanic | Throws panic, for testing |

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Monitoring

Monibuca supports Prometheus monitoring out of the box. Add the following to your Prometheus configuration:

```yaml
scrape_configs:
  - job_name: "monibuca"
    metrics_path: "/api/metrics"
    static_configs:
      - targets: ["localhost:8080"]
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Plugin Development

Monibuca's functionality can be extended through plugins. For information on creating plugins, see the [plugin guide](./plugin/README.md).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## License

Distributed under the MIT License. See `LICENSE` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
[contributors-shield]: https://img.shields.io/github/contributors/Monibuca/v5.svg?style=for-the-badge
[contributors-url]: https://github.com/Monibuca/v5/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/Monibuca/v5.svg?style=for-the-badge
[forks-url]: https://github.com/Monibuca/v5/network/members
[stars-shield]: https://img.shields.io/github/stars/Monibuca/v5.svg?style=for-the-badge
[stars-url]: https://github.com/Monibuca/v5/stargazers
[issues-shield]: https://img.shields.io/github/issues/Monibuca/v5.svg?style=for-the-badge
[issues-url]: https://github.com/Monibuca/v5/issues
[license-shield]: https://img.shields.io/github/license/Monibuca/v5.svg?style=for-the-badge
[license-url]: https://github.com/Monibuca/v5/blob/master/LICENSE