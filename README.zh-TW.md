# go-admin-core

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja-JP.md)

## ✨ 核心特性

### 📝 Logger 模組（企業級日誌解決方案）

- **非同步日誌** - 45x 效能提升（3,358ns → 75ns），支援 100k+ QPS
- **取樣日誌** - 29x 效能提升（3,357ns → 116ns），智慧頻率控制
- **遮罩日誌** - 自動遮罩敏感資料（手機號碼、密碼、電子郵件等），滿足法遵要求
- **Logrus 轉接器** - 完整支援 Logrus 生態，可用 Hooks 超過 50 種
- **正式環境設定** - 內建最佳實務設定（非同步 + 取樣 + 遮罩）
- **並行安全** - 通過 Race Detector 驗證，零資料競爭

### 🔐 JWT 認證

- **Gin 中介軟體** — 為 Gin 應用提供登入、續期與身分解析
- **有界的權杖生命週期** — `MaxRefresh` 限定權杖可被續期的總時長，自首次簽發起計算
- **可設定** — `Timeout` 與 `MaxRefresh` 皆由設定驅動

### 🚀 其他元件

- [x] 快取元件（支援 memory、Redis）——見 [docs/storage](docs/storage/README.md)
- [x] 佇列元件（支援 memory、Redis streams）——見 [docs/storage](docs/storage/README.md)
- [x] 設定管理（支援多種資料來源）
- [x] 日誌寫入器（支援檔案分割）

> 每次改動都會執行競態偵測、雙平台 staticcheck，以及針對每請求程式碼路徑的
> 記憶體配置預算把關。

---

## 🚀 快速開始

### 安裝

```bash
go get -u github.com/go-admin-team/go-admin-core/v2
```

**系統需求:** Go 1.25 或更高版本（以 `go.mod` 中的 `go` 指令為準）

### 基礎日誌用法

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/logger"

func main() {
    // 建立 Logrus 日誌實例
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

### 高效能非同步日誌

```go
// 建立非同步日誌（建議用於高並行情境）
baseLog := logger.NewLogrusLogger(
    logger.WithPath("logs/app.log"),
    logger.WithLevel(logger.InfoLevel),
)

asyncLog := logger.NewAsyncLogger(baseLog, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 45x 效能提升，延遲僅 75ns
asyncLog.Log(logger.InfoLevel, "High performance logging")
```

### 取樣日誌（頻率控制）

```go
// 建立取樣日誌（自動過濾高頻重複日誌）
samplingLog := logger.NewSamplingLogger(baseLog, logger.DefaultSamplingConfig)

// 每秒僅記錄前 100 筆，其後每 100 筆記錄 1 筆
for i := 0; i < 10000; i++ {
    samplingLog.Log(logger.InfoLevel, "High frequency log")
}
```

### 遮罩日誌（資料安全）

```go
// 建立遮罩日誌（自動處理敏感資料）
sanitizerLog := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)

sanitizerLog.Fields(map[string]interface{}{
    "phone":    "13812345678",  // 自動遮罩 → "138****5678"
    "password": "secret123",    // 自動遮罩 → "[REDACTED]"
    "email":    "user@example.com", // 自動遮罩 → "u***@example.com"
}).Log(logger.InfoLevel, "User login")
```

### 正式環境設定（建議）

```go
// 組合使用：遮罩 → 取樣 → 非同步
sanitized := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)
sampled := logger.NewSamplingLogger(sanitized, logger.DefaultSamplingConfig)
asyncLog := logger.NewAsyncLogger(sampled, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 34x 效能提升 + 資料安全 + 頻率控制
asyncLog.Fields(map[string]interface{}{
    "phone":   "13812345678",
    "user_id": 12345,
}).Log(logger.InfoLevel, "Production logging")
```

---

## 📦 設定管理

### 讀取設定檔

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/config"

func main() {
    source := config.FileSource("config.json")
    config.Setup(source, func() {
        // 設定變更回呼
    })
}
```

---

## 📊 效能指標

| 功能 | 效能提升 | 延遲 | 記憶體用量 | 適用情境 |
|------|---------|------|---------|---------|
| 非同步日誌 | 45x | 75ns | 120 B/op | 高並行寫入 |
| 取樣日誌 | 29x | 116ns | 16 B/op | 高頻重複日誌 |
| 正式環境設定 | 34x | 98ns | 149 B/op | 正式環境建議 |

**測試環境:** Apple M1 Pro | Go 1.25.1 | 通過 Race Detector 驗證

---

## 🧪 測試

```bash
# CI 把關執行的全部內容
make ci

# 競態偵測，涵蓋所有套件
make test-race

# 基準測試 —— 冒煙執行；需要可比較的數字請調高 -benchtime
make bench

# 持續負載、資源回收與 goroutine 生命週期
GOADMIN_SOAK=2m make soak

# 與基準版本比較，結果交由 benchstat 判定
make bench-compare BASE=origin/main
```

**每次改動都會強制執行:**

- 競態偵測，涵蓋所有套件
- darwin 與 linux 雙平台 staticcheck
- `api/core.txt` 中的匯出 API 快照必須與程式碼一致
- 每請求程式碼路徑的記憶體配置預算
- 以本儲存庫目前程式碼建置下游使用方

---

## 📦 主要相依套件

```go
github.com/casbin/casbin/v3         v3.8.1
github.com/gin-gonic/gin            v1.11.0
github.com/golang-jwt/jwt/v5        v5.3.0
gorm.io/gorm                        v1.31.1
github.com/sirupsen/logrus          v1.9.4
golang.org/x/crypto                 v0.47.0
```

以上版本為撰寫時 `go.mod` 的內容，一律以該檔案為準。

---

## 🤝 貢獻指南

歡迎提交 Issue 與 Pull Request！

1. Fork 本儲存庫
2. 基於 `main` 建立特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交變更 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. **向 `main` 開啟 Pull Request**

> **分支說明：** `main` 是唯一活躍的分支，也是所有發布版本的來源。
> `master` 與 `dev` 皆為歷史分支，不再接受改動 —— 其中 `master` 自 2022 年
> 起未再變動，且從未進入過發布線。

### 漏洞回報

安全問題請勿開立公開 issue，改用
[私密漏洞回報](https://github.com/go-admin-team/go-admin-core/security/advisories/new)。

---

## 📝 License

Apache License 2.0 - 詳見 [LICENSE](LICENSE) 檔案

---

## 🔗 相關專案

- [go-admin](https://github.com/go-admin-team/go-admin) - 基於 Gin + Vue + Element UI 的前後端分離權限管理系統
- [go-admin-ui](https://github.com/go-admin-team/go-admin-ui) - go-admin 的前端專案

---

## ⭐ Star History

如果這個專案對你有幫助，歡迎點一個 Star ⭐
