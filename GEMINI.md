# 本プロジェクトについて

Pythonの実装からgoへの移行を行っています。プロジェクトルート階層はPython実装、gorsudp/ 以下はGo実装となっています。

あなたはGo/Next.js開発者としてgorsudp/ ディレクトリー内にのみ変更権限を持ちます。プロジェクトルート直下のPython実装を参照することはできますが、ルート階層のPython実装を変更するとあなたは罰されます。

## CLAUDE.md について
ソースコードの実装方針やディレクトリーの構造、ソースコードの責務に変更があった場合にはこのファイル(./CLAUDE.md)を更新して、後任者にわかるように伝えてください。

## gorsudp/ ディレクトリー構造

### 実行バイナリ (cmd/)
- `cmd/gorsudp/main.go`: メインエントリーポイント。設定読み込み、プロデューサー・コンシューマーの起動を行う
- `cmd/gorsudp-settings/`: 設定ファイル生成ツール
- `cmd/gorsudp-test/`: テストデータ生成ツール

### 内部パッケージ (internal/)
- `internal/producer/udp.go`: UDP/IP データストリーム受信。Raspberry Shake デバイスからの地震データを受信
- `internal/broker/`: メッセージブローカー。プロデューサーとコンシューマー間のデータフロー制御
- `internal/alert/`: 地震検出アラート処理。STA/LTA アルゴリズムによる地震イベント検出
- `internal/plot/`: リアルタイム波形・スペクトログラム可視化。D3.js による Web インターフェース
- `internal/notify/`: 通知システム（Telegram、Discord）
- `internal/writer/`: miniSEED ファイル書き込み
- `internal/screenshot/`: グラフのスクリーンショット機能
- `internal/testdata/`: テストデータ生成器

### 共有パッケージ (pkg/)
- `pkg/config/`: 設定ファイル管理と検証
- `pkg/dsp/`: デジタル信号処理（フィルタ、STA/LTA、デコンボリューション）
- `pkg/shakenet/`: Raspberry Shake パケット処理
- `pkg/stream/`: データストリーム管理
- `pkg/monitoring/`: ヘルスチェックとメトリクス

### Web インターフェース
- `web/`: Go側の既存Web実装（レガシー、段階的廃止予定）
- `webapp/`: Next.js フロントエンドアプリケーション（TypeScript + Tailwind CSS + D3.js）

### データディレクトリ
- `data/`: サンプル miniSEED ファイル
- `output/`: 処理済みファイル出力先
- `logs/`: ログファイル
- `screenshots/`: 生成されたスクリーンショット

### 設定・デプロイ
- `test-config.json`: テスト用設定ファイル
- `deploy/docker/`: Docker コンテナ設定
- `deploy/systemd/`: systemd サービス設定

## アーキテクチャ概要

1. **Producer**: UDP パケット受信 → データパース → ブローカーへ送信
2. **Broker**: プロデューサーからデータ受信 → 複数コンシューマーへ配信
3. **Consumers**: 
   - Alert: 地震検出・アラート
   - Plot: Web 可視化
   - Writer: ファイル保存
   - Notify: 外部通知

データフローは Producer → Broker → Consumers の単方向で、ブローカーがファンアウト配信を担当します。

## 開発時の起動方法

### 開発環境（フロントエンド・バックエンド分離）

1. **Go バックエンド起動**:
```bash
cd gorsudp/gorsudp
cp test-config.json config.json
go run cmd/gorsudp/main.go
```
- UDP: `localhost:8888`
- Web API: `localhost:8080`

2. **Next.js フロントエンド起動**:
```bash
cd gorsudp/gorsudp/webapp
npm install  # 初回のみ
npm run dev
```
- フロントエンド: `http://localhost:3000`
- API プロキシ設定により `/api/*` と `/ws` が Go バックエンドに転送される

3. **テストデータ生成（オプション）**:
```bash
cd gorsudp/gorsudp
go run internal/testdata/live_generator.go
```

### 本番環境（統合サーバー）

1. **フロントエンドビルド**:
```bash
cd gorsudp/gorsudp/webapp
npm run build  # dist/ ディレクトリに静的ファイル生成
```

2. **Go サーバー起動**:
```bash
cd gorsudp/gorsudp
./gorsudp -config config.json
```
- `http://localhost:8080` で API と静的ファイルの両方を配信

## 開発・コミット時の必須事項

### Go 側の開発ルール

**コミット前に必ず実行**:
```bash
# gorsudp/ ディレクトリで以下を順番に実行
go fmt ./...
go vet ./...
go test ./...
go mod tidy
```

**品質チェック**:
- テストカバレッジ80%以上を維持
- `go test -race` でレースコンディションチェック
- `golangci-lint run` で静的解析（利用可能な場合）

### Next.js (webapp/) 側の開発ルール

**コミット前に必ず実行**:
```bash
# webapp/ ディレクトリで以下を順番に実行
npm run type-check    # TypeScript型チェック
npm run lint          # ESLint + Next.js ルール
npm run build         # 本番ビルド確認
```

**重要事項**:
- **型安全性**: TypeScript エラーが1つでもある状態でのコミット禁止
- **ビルド確認**: `npm run build` が成功することを確認してからコミット
- **Lintエラー**: ESLint エラー・警告は必ず修正してからコミット
- **依存関係管理**: 
  - `package-lock.json` を直接編集禁止
  - 依存関係の追加・更新は `npm install` コマンドまたは `package.json` 経由で行う
  - `package-lock.json` の変更は慎重に確認し、不要な変更は避ける

### 共通ルール

**コミットメッセージ**:
- Conventional Commits形式推奨: `feat:`, `fix:`, `docs:`, `refactor:` 等
- 変更内容を簡潔かつ明確に記述

**ブランチ戦略**:
- `feature/` プレフィックスで機能開発ブランチ作成
- `main` ブランチへの直接コミット禁止
- Pull Request経由でのマージ必須

**ファイル管理**:
- 一時ファイル・ビルド成果物のコミット禁止
- `.gitignore` の適切な設定維持

