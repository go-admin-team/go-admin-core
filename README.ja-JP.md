# go-admin-core

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja-JP.md)

## ✨ 主な機能

### 📝 Logger モジュール（エンタープライズ向けロギング）

- **非同期ロガー** - 45 倍の高速化（3,358ns → 75ns）、100k+ QPS に対応
- **サンプリングロガー** - 29 倍の高速化（3,357ns → 116ns）、出力頻度を自動制御
- **マスキングロガー** - 機密情報（電話番号・パスワード・メールアドレスなど）を自動マスクし、コンプライアンス要件に対応
- **Logrus アダプター** - Logrus エコシステムに完全対応、50 種類以上の Hook が利用可能
- **本番向け設定** - ベストプラクティス（非同期 + サンプリング + マスキング）を同梱
- **並行安全** - Race Detector で検証済み、データ競合ゼロ

### 🔐 JWT 認証

- **Gin ミドルウェア** — Gin アプリケーションにログイン・更新・ID 解決を提供
- **有限なトークン寿命** — `MaxRefresh` がトークンを更新できる総時間を、最初の発行時点から起算して制限
- **設定可能** — `Timeout` と `MaxRefresh` はいずれも設定ファイルから制御

### 🚀 その他のコンポーネント

- [x] キャッシュ（memory / Redis）—— [docs/storage](docs/storage/README.md) を参照
- [x] キュー（memory / Redis Streams）—— [docs/storage](docs/storage/README.md) を参照
- [x] 設定管理（複数のデータソースに対応）
- [x] ログライター（ファイルローテーション対応）

> 変更のたびに、データ競合検出、2 プラットフォームでの staticcheck、
> およびリクエストごとに実行されるコードパスのメモリアロケーション予算を検証します。

---

## 🚀 クイックスタート

### インストール

```bash
go get -u github.com/go-admin-team/go-admin-core/v2
```

**動作要件:** Go 1.25 以降（`go.mod` の `go` ディレクティブを参照）

### 基本的なロギング

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/logger"

func main() {
    // Logrus ロガーを生成
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

### 高性能な非同期ロギング

```go
// 非同期ロガーを生成（高並行環境で推奨）
baseLog := logger.NewLogrusLogger(
    logger.WithPath("logs/app.log"),
    logger.WithLevel(logger.InfoLevel),
)

asyncLog := logger.NewAsyncLogger(baseLog, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 45 倍の高速化、レイテンシはわずか 75ns
asyncLog.Log(logger.InfoLevel, "High performance logging")
```

### サンプリングロギング（頻度制御）

```go
// サンプリングロガーを生成（高頻度の重複ログを自動的に間引く）
samplingLog := logger.NewSamplingLogger(baseLog, logger.DefaultSamplingConfig)

// 毎秒最初の 100 件のみ記録し、以降は 100 件ごとに 1 件
for i := 0; i < 10000; i++ {
    samplingLog.Log(logger.InfoLevel, "High frequency log")
}
```

### マスキングロギング（データ保護）

```go
// マスキングロガーを生成（機密情報を自動処理）
sanitizerLog := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)

sanitizerLog.Fields(map[string]interface{}{
    "phone":    "13812345678",  // 自動マスク → "138****5678"
    "password": "secret123",    // 自動マスク → "[REDACTED]"
    "email":    "user@example.com", // 自動マスク → "u***@example.com"
}).Log(logger.InfoLevel, "User login")
```

### 本番向け構成（推奨）

```go
// 組み合わせ: マスキング → サンプリング → 非同期
sanitized := logger.NewSanitizerLogger(baseLog, logger.DefaultSanitizerConfig)
sampled := logger.NewSamplingLogger(sanitized, logger.DefaultSamplingConfig)
asyncLog := logger.NewAsyncLogger(sampled, logger.DefaultAsyncConfig)
defer asyncLog.(interface{ Close() error }).Close()

// 34 倍の高速化 + データ保護 + 頻度制御
asyncLog.Fields(map[string]interface{}{
    "phone":   "13812345678",
    "user_id": 12345,
}).Log(logger.InfoLevel, "Production logging")
```

---

## 📦 設定管理

### 設定ファイルの読み込み

```go
package main

import "github.com/go-admin-team/go-admin-core/v2/config"

func main() {
    source := config.FileSource("config.json")
    config.Setup(source, func() {
        // 設定変更時のコールバック
    })
}
```

---

## 📊 パフォーマンス

| 機能 | 高速化 | レイテンシ | メモリ使用量 | 適した用途 |
|------|---------|------|---------|---------|
| 非同期ロガー | 45x | 75ns | 120 B/op | 高並行での書き込み |
| サンプリングロガー | 29x | 116ns | 16 B/op | 高頻度の重複ログ |
| 本番向け構成 | 34x | 98ns | 149 B/op | 本番環境で推奨 |

**測定環境:** Apple M1 Pro | Go 1.25.1 | Race Detector 検証済み

---

## 🧪 テスト

```bash
# CI ゲートが実行する内容すべて
make ci

# データ競合検出（全パッケージ）
make test-race

# ベンチマーク —— スモーク実行。比較可能な数値が必要なら -benchtime を上げる
make bench

# 継続的負荷・リソース回収・goroutine のライフサイクル
GOADMIN_SOAK=2m make soak

# ベースリビジョンと比較し、benchstat で判定
make bench-compare BASE=origin/main
```

**変更のたびに強制されるもの:**

- データ競合検出（全パッケージ）
- darwin と linux 両方での staticcheck
- `api/core.txt` の公開 API スナップショットがコードと一致すること
- リクエストごとに実行されるコードパスのアロケーション予算
- このツリーに対する下流利用者のビルド

---

## 📦 主な依存関係

```go
github.com/casbin/casbin/v3         v3.8.1
github.com/gin-gonic/gin            v1.11.0
github.com/golang-jwt/jwt/v5        v5.3.0
gorm.io/gorm                        v1.31.1
github.com/sirupsen/logrus          v1.9.4
golang.org/x/crypto                 v0.47.0
```

上記は執筆時点の `go.mod` の内容です。実際のバージョンは同ファイルを参照してください。

---

## 🤝 コントリビューション

Issue と Pull Request を歓迎します。

1. 本リポジトリを Fork
2. `main` から機能ブランチを作成 (`git checkout -b feature/AmazingFeature`)
3. 変更をコミット (`git commit -m 'Add some AmazingFeature'`)
4. ブランチへプッシュ (`git push origin feature/AmazingFeature`)
5. **`main` に向けて Pull Request を作成**

> **ブランチについて:** `main` が唯一のアクティブなブランチであり、すべての
> リリースの供給元です。`master` と `dev` は履歴用のブランチで、変更は
> 受け付けていません。`master` は 2022 年以降更新されておらず、リリース
> ラインに乗ったこともありません。

### 脆弱性の報告

セキュリティに関する問題は公開 issue を立てず、
[非公開の脆弱性報告](https://github.com/go-admin-team/go-admin-core/security/advisories/new)
をご利用ください。

---

## 📝 License

Apache License 2.0 - 詳細は [LICENSE](LICENSE) を参照してください。

---

## 🔗 関連プロジェクト

- [go-admin](https://github.com/go-admin-team/go-admin) - Gin + Vue + Element UI によるフロントエンド・バックエンド分離型の権限管理システム
- [go-admin-ui](https://github.com/go-admin-team/go-admin-ui) - go-admin のフロントエンド

---

## ⭐ Star History

このプロジェクトがお役に立ちましたら、Star ⭐ をいただけると励みになります。
