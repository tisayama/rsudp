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

## 重要な開発方針

### WaveformとSpectrogramの表示に関する方針
- **Python実装の表示が正しい基準**: Go実装はPython rsudp実装の表示結果に合わせること
- **勝手な軸ラベルや単位の変更禁止**: Velocity軸、Amplitude軸などのラベルはPython実装に従う
- **単位変換の正確性**: Python実装の計算結果をそのまま再現すること
- **異常な値のスケーリング**: 10^16や10^18といった異常なスケーリングは避け、Python実装の値範囲に合わせる
- **STA/LTA値への影響**: 誤った単位変換がSTA/LTA計算やアラート誤発報の原因となるため、Python実装との整合性を最優先とする

### データ処理の基本方針
- **Deconvolution**: Python実装はデフォルトで`deconvolve=False`（raw counts使用）
- **Alert STA/LTA**: raw countsで処理（Python準拠、デコンボリューション前の値を使用）
- **Plot表示**: デコンボリューション後の物理単位(m/s, m/s²)で表示
- **フィルタリング**: Python実装はデフォルトでwaveform filtering無効、Go実装は設定に依存

### Gitコミットに関する方針
- **勝手なコミット禁止**: ユーザーが明示的に指示するまでgit commitを実行してはならない
- **変更前の確認**: 軸ラベルや単位の変更は必ずユーザーに確認を取ること

## 重要な技術仕様 - Python実装との整合性

### 1. Waveform表示の技術仕様

**データ処理フロー**:
1. Go Backend: UDP packet → デコンボリューション (`/1.6e8` for geophone) → m/s単位
2. Frontend: エンジニアリング記法フォーマッター適用（スケーリングなし）
3. 軸表示: Python準拠のエンジニアリング記法（例: `2.5μ`, `100n`）

**重要な注意点**:
- **ダブルスケーリング禁止**: Backend（デコンボリューション）とFrontend（表示用スケーリング）の両方を適用してはならない
- **単位の自動切り替え**: データ範囲に応じてnm/s、μm/s、m/sを自動選択（ラベルのみ、データはm/s固定）
- **軸目盛り**: 単位なしの数値のみ表示、単位は軸ラベルに記載

### 2. Spectrogram表示の技術仕様

**Python matplotlib準拠パラメータ**:
- **FFTサイズ**: 128（Python: `nearest_pow_2(100)`）
- **オーバーラップ**: 97.5%（Python: `per_lap = 0.975`）
- **パワースケーリング**: 10乗根（Python: `sg**(1/10)`）
- **ゼロパディング**: FFTサイズの4倍（Python: `pad_to=nfft*4`）
- **カラーマップ**: 'inferno'（Python準拠）

**重要な注意点**:
- FFTサイズが大きすぎると時間解像度が悪化
- パワースケーリングが異なると視覚的コントラストが大きく変わる
- オーバーラップ率が低いと時間軸の変化が粗くなる

### 3. STA/LTA処理の技術仕様

**Python ObsPy準拠の実装**:
- **入力データ**: raw counts（デコンボリューション前、Python: `deconvolve=False`）
- **アルゴリズム**: recursive STA/LTA with energy-based calculation
- **値の範囲**: 0.09-0.5（正常）、>3.95（アラート）
- **設定値**: STA=6s, LTA=30s, threshold=3.95, reset=0.9

**重要な注意点**:
- デコンボリューション後の値をSTA/LTAに入力すると10倍程度の差が生じる
- Python実装は`deconvolve=False`がデフォルトのため、raw countsで処理している
- 感度の異なるチャンネル（ジオフォン vs 加速度計）でも同じraw countsを使用

### 4. データフロー設計

```
UDP Packet (raw counts)
├─ Alert Consumer: raw counts → STA/LTA → アラート判定
└─ Plot Consumer: raw counts → デコンボリューション → 物理単位 → 表示
```

**この設計により**:
- STA/LTA値がPython実装と一致
- 表示は適切な物理単位
- アラートの感度がPython実装と同等

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

## 今後の開発における注意事項

### Python実装との互換性維持
- 新機能追加時は必ずPython rsudp実装の動作を調査・比較する
- 表示系の変更は特に慎重に行い、Python実装の見た目・数値と一致させる
- 設定パラメータの変更時はPython実装のデフォルト値を確認する

### パフォーマンス考慮事項
- FFT処理はWeb Workerで実行し、メインスレッドをブロックしない
- Spectrogramの時間解像度とFFTサイズはトレードオフ関係にある
- リアルタイム性を重視し、過度な精度向上は避ける

### セキュリティ考慮事項
- UDP受信は信頼できるRaspberry Shakeデバイスからのみ
- Web インターフェースは内部ネットワーク用途を想定
- 機密情報（設定、ログ）の外部流出防止

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

## トラブルシューティング

### よくある問題と解決方法

#### 1. Waveform軸の数値が異常に大きい（10^18倍等）
**原因**: ダブルスケーリング（Backend + Frontend両方でスケーリング適用）
**解決方法**: 
- Backendでデコンボリューション適用済みの場合、Frontendでの追加スケーリングを削除
- `determineOptimalUnit`で`factor=1`を設定し、表示用スケーリングを無効化

#### 2. STA/LTA値がPython実装と10倍異なる
**原因**: デコンボリューション有無の違い
**解決方法**:
- Alert ConsumerではSTA/LTA処理前にデコンボリューションを削除
- raw countsを直接STA/LTA処理に入力（Python `deconvolve=False`準拠）

#### 3. Spectrogramが真っ白または視覚的に大きく異なる
**原因**: FFTパラメータまたはパワースケーリングの違い
**解決方法**:
- FFTサイズを128に設定（Python準拠）
- パワースケーリングを10乗根に変更（`Math.pow(psd, 1/10)`）
- オーバーラップ率を97.5%に設定

#### 4. 軸ラベルや目盛りが読みにくい
**原因**: エンジニアリング記法の未適用またはカラーテーマの問題
**解決方法**:
- `seismicTheme`を適用して適切な色設定
- エンジニアリング記法フォーマッター（`engineeringFormat`）を使用
- 軸ラベルは中央配置、目盛りは数値のみ表示

### デバッグ時の確認ポイント

1. **データの値範囲確認**:
   - raw counts: 10,000-50,000程度
   - deconvolved: 1e-7 to 1e-4 m/s程度
   - STA/LTA: 0.09-0.5（正常）、>3.95（アラート）

2. **設定ファイルの確認**:
   - `alert.deconv`と`plot.deconv`の設定値
   - Filter設定（`filter_waveform`, `filter_spectrogram`）
   - STA/LTA閾値設定

3. **ログ出力の確認**:
   - デバッグログでraw counts値とdeconvolved値を確認
   - STA/LTA比率の値をPython実装と比較
   - FFT処理エラーやWeb Worker例外をチェック

