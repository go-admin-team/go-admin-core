# go-admin-core

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja-JP.md)

## ✨ Core Features

### 📝 Logger Module (Enterprise-Grade Logging Solution)

- **Async Logger** - 45x performance boost (3,358ns → 75ns), supports 100k+ QPS
- **Sampling Logger** - 29x performance boost (3,357ns → 116ns), intelligent frequency control
- **Sanitizer Logger** - Auto-sanitize sensitive data (phone, password, email, etc.), compliance-ready
- **Logrus Adapter** - Full Logrus ecosystem support with 50+ hooks available
- **Production-Ready Config** - Built-in best practices (async + sampling + sanitization)
- **Concurrency Safe** - Verified with Race Detector, zero data races

### 🔐 JWT Authentication

- **Gin middleware** - Login, refresh and identity handling for Gin applications
- **Bounded token lifetime** - `MaxRefresh` caps how long a token may be renewed
  for, measured from its original issuance
- **Configurable** - `Timeout` and `MaxRefresh` are both driven by configuration

### 🚀 Other Components

- [x] Cache component (memory and Redis) — see [docs/storage](docs/storage/README.md)
- [x] Queue component (memory and Redis streams) — see [docs/storage](docs/storage/README.md)
- [x] Configuration management (multiple data sources)
- [x] Log writer (file rotation support)

> Every change runs the race detector, staticcheck on two platforms, and
> allocation budgets on the paths that execute per request.

---

## 🚀 Quick Start

### Installation

```bash
go get -u github.com/go-admin-team/go-admin-core/v2
```

**System Requirements:** Go 1.25 or higher (see the `go` directive in `go.mod`)

### Basic Logging

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/logger"

func main() {
    // Create Logrus logger instance
    log := logger.NewLogrusLogger(
        logger.WithPath("logs/app.log"),
        logger.WithLevel(logger.InfoLevel),
    )
    
    log.Log(logger.InfoLevel, "Application started")
    log.Fields(map[string]interface{}{
        "user_id": 12345,
        "action":  "login",
    }).Log(logger.InfoLevel, "User action")
}
```

### High-Performance Async Logger

```go
// Create async logger (recommended for high-concurrency scenarios)
baseLog := logger.NewLogrusLogger(
    logger.WithPath("logs/app.log"),
    logger.WithLevel(logger.InfoLevel),
)

asyncLog := logger.NewAsyncLogger(baseLog, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 45x performance boost, only 75ns latency
asyncLog.Log(logger.InfoLevel, "High performance logging")
```

### Sampling Logger (Frequency Control)

```go
// Create sampling logger (auto-filter high-frequency duplicate logs)
samplingLog := logger.NewSamplingLogger(baseLog, logger.DefaultSamplingConfig)

// Only logs first 100 per second, then 1 out of every 100
for i := 0; i < 10000; i++ {
    samplingLog.Log(logger.InfoLevel, "High frequency log")
}
```

### Sanitizer Logger (Data Security)

```go
// Create sanitizer logger (auto-handle sensitive data)
sanitizerLog := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)

sanitizerLog.Fields(map[string]interface{}{
    "phone":    "13812345678",  // Auto-sanitized → "138****5678"
    "password": "secret123",    // Auto-sanitized → "[REDACTED]"
    "email":    "user@example.com", // Auto-sanitized → "u***@example.com"
}).Log(logger.InfoLevel, "User login")
```

### Production-Grade Configuration (Recommended)

```go
// Combine: Sanitizer → Sampling → Async
sanitized := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)
sampled := logger.NewSamplingLogger(sanitized, logger.DefaultSamplingConfig)
asyncLog := logger.NewAsyncLogger(sampled, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 34x performance boost + data security + frequency control
asyncLog.Fields(map[string]interface{}{
    "phone":   "13812345678",
    "user_id": 12345,
}).Log(logger.InfoLevel, "Production logging")
```

---

## 📦 Configuration Management

### Read Configuration Files

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/config"

func main() {
    source := config.FileSource("config.json")
    config.Setup(source, func() {
        // Configuration change callback
    })
}
```

---

## 📊 Performance Metrics

| Feature | Performance Boost | Latency | Memory | Use Case |
|---------|------------------|---------|--------|----------|
| Async Logger | 45x | 75ns | 120 B/op | High-concurrency writes |
| Sampling Logger | 29x | 116ns | 16 B/op | High-frequency duplicate logs |
| Production Config | 34x | 98ns | 149 B/op | Production environments |

**Test Environment:** Apple M1 Pro | Go 1.25.1 | Race Detector verified

---

## 🧪 Testing

```bash
# Everything the CI gate runs
make ci

# Race detector, across every package
make test-race

# Benchmarks — a smoke run; raise -benchtime for numbers worth comparing
make bench

# Sustained load, resource reclamation and goroutine lifecycles
GOADMIN_SOAK=2m make soak

# Measure this tree against a base revision, through benchstat
make bench-compare BASE=origin/main
```

**Enforced on every change:**

- Race detector, every package
- staticcheck for both darwin and linux
- The exported API snapshot in `api/core.txt` must match the code
- Allocation budgets on the paths that run per request
- Downstream consumers are built against this tree

---

## 📦 Main Dependencies

```go
github.com/casbin/casbin/v3         v3.8.1
github.com/gin-gonic/gin            v1.11.0
github.com/golang-jwt/jwt/v5        v5.3.0
gorm.io/gorm                        v1.31.1
github.com/sirupsen/logrus          v1.9.4
golang.org/x/crypto                 v0.47.0
```

Versions above reflect `go.mod` at the time of writing; that file is the source
of truth.

---

## 🤝 Contributing

We welcome Issues and Pull Requests!

1. Fork this repository
2. Create your feature branch from `main` (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request **against `main`**

> **Branches:** `main` is the only active branch and the source of every release.
> `master` and `dev` are historical and no longer accept changes — `master` in
> particular has not moved since 2022 and was never part of the release lineage.

### Reporting a vulnerability

Please do not open a public issue for security problems. Use
[private vulnerability reporting](https://github.com/go-admin-team/go-admin-core/security/advisories/new)
instead.

---

## 📝 License

Apache License 2.0 - See [LICENSE](LICENSE) file for details

---

## 🔗 Related Projects

- [go-admin](https://github.com/go-admin-team/go-admin) - Gin + Vue + Element UI based RBAC system with separated frontend/backend
- [go-admin-ui](https://github.com/go-admin-team/go-admin-ui) - go-admin frontend project

---

## ⭐ Star History

If this project helps you, please give us a Star ⭐
