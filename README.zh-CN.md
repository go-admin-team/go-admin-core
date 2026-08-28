# go-admin-core

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja-JP.md)

## ✨ 核心特性

### 📝 Logger 模块（企业级日志解决方案）

- **异步日志** - 45x 性能提升（3,358ns → 75ns），支持 100k+ QPS
- **采样日志** - 29x 性能提升（3,357ns → 116ns），智能频率控制
- **脱敏日志** - 自动脱敏敏感数据（手机号、密码、邮箱等），满足合规要求
- **Logrus 适配器** - 完整 Logrus 生态支持，50+ Hooks 可用
- **生产级配置** - 内置最佳实践配置（异步 + 采样 + 脱敏）
- **并发安全** - Race Detector 验证通过，零数据竞争

### 🔐 JWT 认证

- **Gin 中间件** — 为 Gin 应用提供登录、续期与身份解析
- **有界的令牌生命周期** — `MaxRefresh` 限定令牌可被续期的总时长，自首次签发起计算
- **可配置** — `Timeout` 与 `MaxRefresh` 均由配置驱动

### 🚀 其他组件

- [x] 缓存组件（支持 memory、Redis）——见 [docs/storage](docs/storage/README.md)
- [x] 队列组件（支持 memory、Redis streams）——见 [docs/storage](docs/storage/README.md)
- [x] 配置管理（支持多种数据源）
- [x] 日志写入器（支持文件分割）

> 每次改动都会执行竞态检测、双平台 staticcheck，以及针对每请求代码路径的
> 内存分配预算门禁。

---

## 🚀 快速开始

### 安装

```bash
go get -u github.com/go-admin-team/go-admin-core/v2
```

**系统要求:** Go 1.25 或更高版本（以 `go.mod` 中的 `go` 指令为准）

### 基础日志使用

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/logger"

func main() {
    // 创建 Logrus 日志实例
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

### 高性能异步日志

```go
// 创建异步日志（推荐用于高并发场景）
baseLog := logger.NewLogrusLogger(
    logger.WithPath("logs/app.log"),
    logger.WithLevel(logger.InfoLevel),
)

asyncLog := logger.NewAsyncLogger(baseLog, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 45x 性能提升，延迟仅 75ns
asyncLog.Log(logger.InfoLevel, "High performance logging")
```

### 采样日志（频率控制）

```go
// 创建采样日志（自动过滤高频重复日志）
samplingLog := logger.NewSamplingLogger(baseLog, logger.DefaultSamplingConfig)

// 每秒仅记录前 100 条，其后每 100 条记录 1 条
for i := 0; i < 10000; i++ {
    samplingLog.Log(logger.InfoLevel, "High frequency log")
}
```

### 脱敏日志（数据安全）

```go
// 创建脱敏日志（自动处理敏感数据）
sanitizerLog := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)

sanitizerLog.Fields(map[string]interface{}{
    "phone":    "13812345678",  // 自动脱敏 → "138****5678"
    "password": "secret123",    // 自动脱敏 → "[REDACTED]"
    "email":    "user@example.com", // 自动脱敏 → "u***@example.com"
}).Log(logger.InfoLevel, "User login")
```

### 生产级配置（推荐）

```go
// 组合使用：脱敏 → 采样 → 异步
sanitized := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)
sampled := logger.NewSamplingLogger(sanitized, logger.DefaultSamplingConfig)
asyncLog := logger.NewAsyncLogger(sampled, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 34x 性能提升 + 数据安全 + 频率控制
asyncLog.Fields(map[string]interface{}{
    "phone":   "13812345678",
    "user_id": 12345,
}).Log(logger.InfoLevel, "Production logging")
```

---

## 📦 配置管理

### 配置文件读取

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/config"

func main() {
    source := config.FileSource("config.json")
    config.Setup(source, func() {
        // 配置变更回调
    })
}
```

---

## 📊 性能指标

| 功能 | 性能提升 | 延迟 | 内存占用 | 适用场景 |
|------|---------|------|---------|---------|
| 异步日志 | 45x | 75ns | 120 B/op | 高并发写入 |
| 采样日志 | 29x | 116ns | 16 B/op | 高频重复日志 |
| 生产配置 | 34x | 98ns | 149 B/op | 生产环境推荐 |

**测试环境:** Apple M1 Pro | Go 1.25.1 | Race Detector 验证通过

---

## 🧪 测试

```bash
# CI 门禁跑的全部内容
make ci

# 竞态检测，覆盖所有包
make test-race

# 基准测试 —— 冒烟运行；需要可比较的数字请调高 -benchtime
make bench

# 持续负载、资源回收与 goroutine 生命周期
GOADMIN_SOAK=2m make soak

# 与基准版本对比，结果交给 benchstat 判定
make bench-compare BASE=origin/main
```

**每次改动都会强制执行:**

- 竞态检测，覆盖所有包
- darwin 与 linux 双平台 staticcheck
- `api/core.txt` 中的导出 API 快照必须与代码一致
- 每请求代码路径的内存分配预算
- 以本仓库当前代码构建下游使用方

---

## 📦 主要依赖

```go
github.com/casbin/casbin/v3         v3.8.1
github.com/gin-gonic/gin            v1.11.0
github.com/golang-jwt/jwt/v5        v5.3.0
gorm.io/gorm                        v1.31.1
github.com/sirupsen/logrus          v1.9.4
golang.org/x/crypto                 v0.47.0
```

以上版本为撰写时 `go.mod` 的内容，该文件为准。

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 基于 `main` 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. **向 `main` 开启 Pull Request**

> **分支说明：** `main` 是唯一活跃的分支，也是所有发布版本的来源。
> `master` 与 `dev` 均为历史分支，不再接受改动 —— 其中 `master` 自 2022 年
> 起未再变动，且从未进入过发布线。

### 漏洞报告

安全问题请勿开公开 issue，改用
[私密漏洞报告](https://github.com/go-admin-team/go-admin-core/security/advisories/new)。

---

## 📝 License

Apache License 2.0 - 详见 [LICENSE](LICENSE) 文件

---

## 🔗 相关项目

- [go-admin](https://github.com/go-admin-team/go-admin) - 基于 Gin + Vue + Element UI 的前后端分离权限管理系统
- [go-admin-ui](https://github.com/go-admin-team/go-admin-ui) - go-admin 的前端项目

---

## ⭐ Star History

如果这个项目对你有帮助，欢迎点一个 Star ⭐
