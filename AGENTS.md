# Inaly Pulse 完全開発仕様・実装プロンプト

あなたは、Go言語、GitHub Actions、GitHub Apps、GitHub REST API、ネットワーク監視、分散システム、ソフトウェアアーキテクチャに精通したシニアソフトウェアエンジニアです。

以下の要件に従い、**Inaly Pulse** というGitHubベースの分散型監視Botを設計・開発してください。

単に動作するコードを作成するだけでなく、保守性、拡張性、セキュリティ、テスト容易性、データ整合性を重視してください。

仕様に矛盾や不明点がある場合は、勝手に決定せず、問題点と選択肢を提示してください。ただし、確定済みの設計方針は変更しないでください。

既存コードが存在する場合は、最初に構成と実装状況を調査し、既存実装を尊重してください。確認せずに全面的な書き換えを行ってはいけません。

---

# 1. プロジェクト概要

## 1.1 プロジェクト名

**Inaly Pulse**

## 1.2 目的

GitHub Actionsを利用して、Webサイトやネットワークサービスの死活監視を行うGitHubベースの監視Botを開発する。

監視結果、監視状態、障害履歴、メンテナンス情報、レポートなどはJSON形式で保存し、GitHubリポジトリを永続ストレージとして利用する。

GitHub Issuesを障害管理に、Discord Webhookを通知に利用する。

独自のWebサーバー、データベース、死活監視ダッシュボードは基本機能として実装しない。

## 1.3 基本コンセプト

- GitHub Actions上で動作する。
- Bot本体はGo言語で実装する。
- Gitを永続ストレージとして利用する。
- 監視対象リポジトリごとに完全独立して動作する。
- GitHub Appで認証する。
- 監視結果はJSONとして保存する。
- 障害管理にはGitHub Issuesを利用する。
- 通知にはDiscord Webhookを利用する。
- 将来的な監視方式や通知方式の追加を容易にする。
- Architecture Firstを徹底する。

---

# 2. 確定済みの設計方針

以下は確定事項であり、変更しないこと。

| 項目 | 採用仕様 |
|---|---|
| プロジェクト名 | Inaly Pulse |
| 開発リポジトリ | `LingXuan-Li/inaly_pulse` |
| 開発リポジトリ公開範囲 | Public |
| 開発言語 | Go |
| 実行環境 | GitHub-hosted GitHub Actionsのみ |
| ストレージ | Git |
| データ形式 | JSON |
| 対応範囲 | Multi-Repository |
| リポジトリ間の独立性 | 各リポジトリ完全独立 |
| 開発優先方針 | Architecture First |
| Workflow周期 | 10分 |
| Monitor Interval | Workflow周期の10分以上の整数倍 |
| 履歴保持期間 | ユーザー設定 |
| 履歴保持の初期値 | 30日 |
| アーカイブ保持期間 | 初期値180日 |
| レポート保持期間 | 初期値180日 |
| DNS Resolver | OS標準、Google Public DNS、Cloudflare DNS |
| Google Public DNS | 8.8.8.8 |
| Cloudflare DNS | 1.1.1.1 |
| 障害管理 | 障害発生ごとに新しいIssueを作成 |
| 継続中の障害 | 同じIssueに情報を追記 |
| 復旧後の再障害 | 新しいIssueを作成 |
| 通知 | Discord Webhook |
| Webダッシュボード | 実装しない |
| 独自データベース | 使用しない |
| 独自Webサーバー | 使用しない |
| Runner | GitHub-hostedのみ |

Self-hosted Runnerは対象外とする。

---

# 3. リポジトリアーキテクチャ

## 3.1 開発リポジトリ

開発リポジトリは以下とする。

`LingXuan-Li/inaly_pulse`

公開範囲はPublic。

このリポジトリにはBot本体と共通処理を配置する。

主な役割：

- 監視エンジン
- HTTP監視
- TCP監視
- DNS監視
- Ping監視
- Latency評価
- 障害判定
- メンテナンス管理
- GitHub App認証
- GitHub API操作
- Gitストレージ処理
- Discord通知
- Outbox管理
- 稼働率計算
- レポート生成
- 設定検証
- テストコード
- 共通GitHub Actions Workflow
- ドキュメント

開発リポジトリには、個々のユーザーや監視対象に固有の監視結果を保存しないこと。

## 3.2 監視対象リポジトリ

監視対象リポジトリには以下を配置する。

- 監視設定
- Bot起動用GitHub Actions Workflow
- 監視結果JSON
- 監視履歴JSON
- 障害情報JSON
- メンテナンス情報JSON
- レポートJSON
- 通知Outbox情報

監視対象リポジトリごとに独立した設定・状態・履歴・Issues・通知先を持たせる。

異なるリポジトリの監視情報を共有しないこと。

## 3.3 全体構成

```text
inaly_pulse/
├── cmd/
├── internal/
├── .github/
│   └── workflows/
├── config/
├── docs/
├── tests/
├── go.mod
└── README.md

target-repository/
├── .github/
│   └── workflows/
│       └── inaly-pulse.yml
├── config/
│   ├── monitors.json
│   └── settings.json
└── data/
    ├── state/
    ├── incidents/
    ├── history/
    ├── maintenance/
    ├── reports/
    └── outbox/
```

---

# 4. GitHub Actionsによる実行方式

## 4.1 基本方針

GitHub-hosted Runnerのみを使用する。

監視対象リポジトリ側に起動用Workflowを配置し、開発リポジトリ側にある共通Reusable Workflowを呼び出す。

Bot本体を各監視対象リポジトリに複製しない。

## 4.2 起動方式

最低限、以下の起動方式を実装する。

### 定期実行

GitHub Actionsの`schedule`を利用する。

**Workflowの実行周期は10分とする。**

```yaml
on:
  schedule:
    - cron: "2,12,22,32,42,52 * * * *"
```

GitHub ActionsのスケジュールはUTC基準であり、実行遅延や欠落が起こり得る。

そのため、実行時刻だけを根拠に監視間隔を計算してはいけない。

### 手動実行

`workflow_dispatch`に対応する。

GitHub Actionsの画面から管理者が監視を開始できるようにする。

手動実行では、通常の定期実行とは別に、強制実行または期限到来対象のみの実行を選択できる設計を検討する。

### 将来の拡張

Issueコメントによるコマンド実行を追加できる設計にする。

初期実装では定期実行と手動実行を優先する。

## 4.3 Monitor IntervalとWorkflow周期

Monitorごとの`interval`は、Workflow周期の10分以上の整数倍のみ許可する。

有効な例：

- 10分
- 20分
- 30分
- 40分
- 60分
- 120分

無効な例：

- 5分
- 15分
- 25分
- 45分

Workflowは10分ごとに起動するが、すべてのMonitorを毎回実行するわけではない。

各Monitorについて、前回実行時刻からの経過時間を確認し、以下の条件を満たす場合だけ実行する。

```text
elapsed >= interval - tolerance
```

`tolerance`は設定可能にする。

実行時刻ではなく、実際の前回実行時刻を基準にする。

これにより、GitHub Actionsの遅延や欠落による監視間隔のずれを考慮する。

前回実行時刻はGit上の状態ファイルに保存する。

同一Monitorの二重実行を防止するため、実行状態や実行IDを管理する。

複数回分の監視予定時刻を一度のWorkflowで処理するかどうかは、監視間隔と遅延の状況に応じて設計する。

監視予定が欠落した場合に、過去の監視を実際に実行したかのように記録してはいけない。

## 4.4 Reusable Workflow

開発リポジトリ側に共通Workflowを配置する。

監視対象リポジトリ側のWorkflowは、共通Workflowを呼び出すだけの構成にする。

```yaml
name: Inaly Pulse

on:
  schedule:
    - cron: "2,12,22,32,42,52 * * * *"
  workflow_dispatch:

permissions:
  contents: read

jobs:
  monitor:
    uses: LingXuan-Li/inaly_pulse/.github/workflows/monitor.yml@v1
    secrets:
      INALY_APP_PRIVATE_KEY: ${{ secrets.INALY_APP_PRIVATE_KEY }}
      INALY_MONITOR_SECRETS: ${{ secrets.INALY_MONITOR_SECRETS }}
      DISCORD_WEBHOOK_URL: ${{ secrets.DISCORD_WEBHOOK_URL }}
```

上記は構成例であり、実際のWorkflowの入出力、権限、Secretsの受け渡しを正確に定義すること。

開発リポジトリはPublicとする。

Private RepositoryのReusable Workflowに関する制約を前提にした不要な設定を追加しないこと。

## 4.5 Runner上の処理

共通Workflowでは以下を実行する。

1. 監視対象リポジトリをCheckoutする。
2. Bot本体をCheckoutする。
3. Go環境をセットアップする。
4. GitHub Appの認証情報を受け取る。
5. 監視対象の設定ファイルを読み込む。
6. Botを実行する。
7. 監視結果を対象リポジトリへ保存する。
8. 必要に応じてGitHub Issuesを更新する。
9. 未送信Outboxイベントを処理する。
10. 必要に応じてDiscordへ通知する。
11. Outboxの送信状態を保存する。

実行環境は毎回新しいRunnerになることを前提とし、ローカルファイルシステムに永続データを依存させない。

---

# 5. GitHub App認証

## 5.1 認証方式

GitHub Appを使用する。

Personal Access Tokenを基本認証方式として使用しない。

App JWTを生成し、対象リポジトリのInstallationを取得してInstallation Access Tokenを発行する。

固定のInstallation IDを設定ファイルに埋め込まない。

## 5.2 認証フロー

1. GitHub App IDを取得する。
2. Private Keyを利用してApp JWTを生成する。
3. 対象リポジトリのInstallation情報を取得する。
4. Installation Access Tokenを発行する。
5. 必要なAPI操作を実行する。
6. Tokenの有効期限を考慮する。

Tokenは必要最小限のRepositoryに限定する。

## 5.3 権限

必要な権限を最小限にする。

想定する権限：

- Metadata：Read
- Contents：Read and write
- Issues：Read and write

必要な権限が増える場合は、理由を明示すること。

## 5.4 Secrets

以下をGitHub Actions Secretsで管理する。

| Secret名 | 用途 |
|---|---|
| `INALY_APP_PRIVATE_KEY` | GitHub App Private Key |
| `INALY_MONITOR_SECRETS` | 監視対象の認証情報をまとめたJSON |
| `DISCORD_WEBHOOK_URL` | Discord Webhook URL |

GitHub App IDは機密情報ではないため、必要に応じてRepository Variablesなどで管理してよい。

Private Key、Webhook URL、監視対象の認証情報をソースコード、通常のJSON設定、ログに出力してはならない。

---

# 6. Gitストレージ

## 6.1 基本方針

GitHubリポジトリを永続ストレージとして利用する。

監視結果はJSONファイルとして保存する。

ローカルファイルは実行時の一時データとして扱う。

## 6.2 保存先ブランチ

**デフォルトブランチに直接コミットする。**

設定ファイルと監視結果は同じブランチのツリーに保存する。

別のデータ専用ブランチは初期実装では使用しない。

デフォルトブランチにブランチ保護が設定されている場合は、GitHub Appによる更新が可能な権限設定が必要になる。

仕様上はGitHub Appのブランチ保護バイパスを許可する構成を想定する。

ただし、これはセキュリティ上のリスクを伴うため、以下を必ず検討する。

- バイパス対象をInaly PulseのGitHub Appに限定する。
- 必要最小限のRepositoryに限定する。
- 人間の通常の保護ルールを可能な限り維持する。
- バイパス権限を付与する理由をドキュメント化する。
- ブランチ保護設定の変更を監査可能にする。

バイパスを無条件に推奨しないこと。

## 6.3 GitHub APIによる保存

複数ファイルを更新する場合はGitHub Git Data APIを使用する。

基本的な処理：

1. 現在のRefを取得する。
2. 最新Commitを取得する。
3. 必要なBlobを作成する。
4. Treeを作成する。
5. Commitを作成する。
6. Refを更新する。

複数ファイル更新を可能な限り1回のCommitにまとめる。

Contents APIを複数回呼び出して、複数ファイルを順番に更新する方式は避ける。

## 6.4 同時更新対策

Gitの更新競合を考慮する。

- Ref更新時のSHA競合を検出する。
- 最新状態を再取得する。
- 必要な変更を再適用する。
- 再試行回数を制限する。
- 冪等性を確保する。
- 変更がない場合はCommitを作成しない。
- 同一Monitorの状態更新を保護する。

Gitの更新が成功していないのに保存成功として扱わないこと。

## 6.5 履歴保持

監視履歴保持期間はユーザーが設定できるようにする。

初期値は30日とする。

保持期間を超えた履歴は削除対象とする。

アーカイブとレポートは監視履歴と異なる保持期間を持つ。

| 設定 | 初期値 |
|---|---:|
| `history_retention_days` | 30 |
| `archive_retention_days` | 180 |
| `report_retention_days` | 180 |

履歴削除は一度に大量のファイルを更新しないように設計する。

保持期間を0にした場合の意味を明確に定義すること。無期限保持を意味するのか、保存しないことを意味するのかを曖昧にしない。

---

# 7. ディレクトリ構成

以下を初期設計案とする。

```text
inaly_pulse/
├── cmd/
│   ├── monitor/
│   ├── command/
│   ├── report/
│   └── validate/
│
├── internal/
│   ├── app/
│   │
│   ├── domain/
│   │   ├── monitor/
│   │   ├── incident/
│   │   ├── maintenance/
│   │   ├── uptime/
│   │   └── notification/
│   │
│   ├── checker/
│   │   ├── http/
│   │   ├── tcp/
│   │   ├── dns/
│   │   ├── ping/
│   │   └── latency/
│   │
│   ├── validator/
│   │   ├── json/
│   │   ├── txt/
│   │   └── html/
│   │
│   ├── githubapp/
│   │   ├── auth/
│   │   ├── installation/
│   │   ├── client/
│   │   ├── issues/
│   │   ├── comments/
│   │   └── gitdata/
│   │
│   ├── storage/
│   │   ├── repository.go
│   │   ├── git/
│   │   ├── memory/
│   │   └── model/
│   │
│   ├── notifier/
│   │   └── discord/
│   │
│   ├── outbox/
│   ├── config/
│   ├── security/
│   ├── report/
│   └── logger/
│
├── .github/
│   └── workflows/
│       ├── monitor.yml
│       ├── test.yml
│       └── release.yml
│
├── config/
├── docs/
├── tests/
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

これは初期案であり、責務の重複や不自然な依存関係がある場合は、理由を説明した上で改善してよい。

Domain層がGitHub APIやDiscord APIに直接依存しない設計を維持すること。

---

# 8. 監視エンジン

## 8.1 共通インターフェース

各監視方式は共通インターフェースを実装する。

```go
type Checker interface {
    Type() string
    Check(ctx context.Context, monitor Monitor) Result
}
```

実際の型やパッケージ構成は、Domain層との依存関係を考慮して決定する。

## 8.2 共通Result

各Checkerの結果には最低限以下を含める。

- MonitorID
- Type
- Status
- StartedAt
- FinishedAt
- Duration
- Error
- Details

監視方式ごとの固有情報は、型安全性を考慮した構造で保持する。

共通結果にすべての方式固有情報を無理に詰め込まないこと。

## 8.3 HTTP監視

以下に対応する。

- GET
- POST
- HEAD
- HTTP Headers
- Basic Authentication
- Bearer Authentication
- Request Body
- Content-Type
- Timeout
- Redirect制御
- HTTP Status Code
- Response Time
- Response Size
- JSON検証
- TXT検証
- HTML検証
- SSL/TLS情報
- Latency評価

監視対象の期待値と実際の応答を比較できるようにする。

HTTP Statusだけでなく、必要に応じてレスポンス本文も検証する。

認証情報をログや監視結果に保存しないこと。

## 8.4 TCP監視

以下を監視する。

- DNS解決
- TCP接続成功・失敗
- 接続時間
- Timeout
- 接続エラー分類

TCP接続成功は、接続先アプリケーション全体が正常であることを保証しない。

その点を状態判定に反映する。

## 8.5 DNS監視

以下のResolverに対応する。

| ID | Resolver |
|---|---|
| `system` | OS標準Resolver |
| `google` | 8.8.8.8 |
| `cloudflare` | 1.1.1.1 |

以下のレコードタイプに対応する。

- A
- AAAA
- CNAME
- MX
- TXT
- NS

以下を記録する。

- Query
- Resolver
- Record Type
- Response
- TTL
- Response Time
- Error
- DNS Response Code

DNSエラーを分類する。

- NXDOMAIN
- SERVFAIL
- REFUSED
- Timeout
- その他のエラー

### DNS判定ルール

- 全Resolverが失敗した場合は`failure`。
- 一部Resolverだけが失敗した場合は`success`とし、Detailsに警告を記録する。
- Resolver間の応答差異は情報として記録し、それだけで障害判定しない。

DNSの地域差、キャッシュ、伝播状況などを考慮する。

Resolverごとの応答を識別可能な形式で保存する。

DNS監視機能と、DNS解決結果を利用したネットワーク接続処理は区別する。

## 8.6 Ping監視

ICMP Echo Request / Replyを利用する。

記録項目：

- 送信数
- 受信数
- パケットロス率
- RTT最小値
- RTT平均値
- RTT最大値
- Timeout
- Error

GitHub-hosted RunnerでICMPが利用できない場合は`unsupported`として扱う。

ICMP非対応をサービス障害と同一視しない。

Ping単独の失敗でHTTPサービスなどを停止判定しない。

必要なOS権限や実行環境の制約を事前に確認し、利用できない場合の代替手段を明示する。

## 8.7 Latency監視

Latencyは独立した監視方式というより、各Checkerの結果を評価する共通機能として設計する。

設定可能な項目：

- Warning Threshold
- Critical Threshold
- 連続超過回数
- 評価対象

監視結果の応答時間をもとに正常・劣化などの状態を判定する。

---

# 9. 監視状態管理

## 9.1 状態の分離

以下を混同しないこと。

### Check Result

個々の監視実行結果。

- success
- failure
- timeout
- unsupported
- skipped

### Monitor State

監視対象の現在状態。

- unknown
- operational
- degraded
- outage
- paused

### Incident State

障害の状態。

- open
- resolved

`investigating`は初期実装では使用しない。

将来的に`/ack`などのコマンドで状態遷移を実装する際に追加する。

### Maintenance State

メンテナンスの状態。

- inactive
- active

## 9.2 障害判定

初期設定：

- 連続失敗3回で障害判定
- 復旧成功2回で復旧判定
- Timeout初期値10秒

これらはユーザー設定で変更できるようにする。

### 連続失敗の扱い

失敗1回目・2回目ではMonitor Stateを変更しない。

直前の状態を維持し、失敗カウンタだけを増加させる。

3回連続失敗した時点で`outage`へ遷移する。

### degradedの定義

`degraded`はLatency超過専用とする。

- 接続自体は成功している。
- 応答時間がWarningまたはCriticalの閾値を超過している。
- 設定された連続超過条件を満たしている。

連続接続失敗を`degraded`として扱わない。

Latencyの評価条件と障害判定条件を独立させる。

### 基本的な遷移

```text
unknown
   |
   | 正常応答
   v
operational
   |
   | Latency超過条件成立
   v
degraded
   |
   | Latency正常化
   v
operational
```

接続失敗時：

```text
operational
   |
   | 失敗1回目
   v
operational（失敗カウンタ=1）
   |
   | 失敗2回目
   v
operational（失敗カウンタ=2）
   |
   | 失敗3回目
   v
outage
   |
   | 復旧成功1回目
   v
outage（復旧カウンタ=1）
   |
   | 復旧成功2回目
   v
operational
```

`degraded`状態から接続失敗が発生した場合も、失敗カウンタを管理し、障害条件成立時に`outage`へ遷移させる。

状態遷移の条件はDomain層で一元管理する。

## 9.3 Monitor ID再利用防止

削除されたMonitor IDは、`data/state/metadata.json`の退役リストに記録する。

設定検証時に、退役済みIDの再利用を拒否する。

IDの生成方式としてUUID v7を利用することも可能とする。

UUID v7を使用する場合は、必要なUUIDライブラリの利用を許可する。

ただし、IDの退役管理とUUID生成は別の責務として設計する。

過去のIncidentと新しいMonitorが混同されないことを保証する。

---

# 10. 障害管理とGitHub Issues

## 10.1 基本方針

障害が発生するたびに新しいGitHub Issueを作成する。

同一障害が継続している間は、既存Issueに情報を追記する。

## 10.2 Issueのライフサイクル

1. 障害判定が成立する。
2. 新しいIncident IDを発行する。
3. Git上にIncidentを保存する。
4. GitHub Issueを作成する。
5. 障害の発生時刻、監視対象、エラー情報を記録する。
6. 障害継続中は既存Issueに経過を追記する。
7. 復旧判定が成立する。
8. 同じIssueに復旧情報を追記する。
9. Issueをクローズする。

復旧後に再び障害が発生した場合は、新しいIssueを作成する。

## 10.3 Issueの手動削除

GitHub Issueが手動削除された場合、同じIncident IDのまま新しいIssueを作成する。

新しいIssueの番号をGit上のIncident情報に再度対応付ける。

新しいIssueの本文には、Issueが再作成されたことを明記する。

Incident IDは変更しない。

過去のIssue番号を履歴として保持し、再作成前後の対応関係を追跡できるようにする。

## 10.4 Issueの手動クローズ

ユーザーがIssueを手動でクローズした場合は、クローズ状態を維持する。

BotはIssueを自動的に再オープンしない。

障害が継続していても、必要な情報は既存Issueへのコメントとして追記する。

人間による操作をBotが覆さないこと。

Git上のIncident状態とIssue状態の不一致は記録し、必要に応じて管理情報に反映する。

## 10.5 重複防止

GitHub APIの失敗やWorkflowの再実行によって、同じ障害のIssueが重複作成されないようにする。

Incident IDを利用して、作成済みIssueとの対応関係を管理する。

Git上のIncident情報を正本とし、GitHub Issuesはその管理・表示手段として扱う。

---

# 11. Maintenance機能

メンテナンス期間を設定できるようにする。

## 11.1 種類

- Global Maintenance
- Monitor Maintenance
- Scheduled Maintenance
- Manual Maintenance

## 11.2 Scheduled Maintenance

初期実装では固定期間のみ対応する。

- 開始日時
- 終了日時
- UTC基準

繰り返しスケジュールは初期実装に含めない。

将来的にcron、継続時間、タイムゾーンに対応する場合はschema_versionを上げて拡張する。

## 11.3 動作モード

### skip

監視処理自体を実行しない。

### suppress

監視は継続するが、通知や障害Issueの新規作成を抑制する。

デフォルトは`suppress`とする。

## 11.4 メンテナンス中の障害・復旧

Git上のIncident情報は、メンテナンス中でも常に記録する。

### メンテナンス中に新規障害が発生した場合

- Git上にIncidentを作成する。
- Incident Stateを`open`にする。
- GitHub Issueはメンテナンス中には作成しない。
- Discord通知もメンテナンス中には送信しない。
- メンテナンス終了時に障害が継続していればIssueを作成する。

### メンテナンス中に復旧した場合

- Git上のIncidentを`resolved`にする。
- メンテナンス中にはIssueのクローズ処理を行わない。
- メンテナンス終了後にIssueのクローズを行う。
- Discord通知もメンテナンス終了後に反映する。

### メンテナンス終了時

Git上のIncidentとIssueの状態を照合し、必要なIssue操作と通知をまとめて反映する。

Git上のIncidentは常に正本とする。

メンテナンス終了後に実際の状態とIssueの状態が一致するようにする。

## 11.5 メンテナンス中の処理

- 監視結果を保存する。
- Git上のIncidentを更新する。
- Discord通知を抑制する。
- 新規障害Issueの作成を抑制する。
- 既存Issueを自動クローズしない。
- 稼働率の計算対象から除外する。
- メンテナンス終了後に通常の監視へ戻す。

pauseとmaintenanceは別機能として扱う。

pauseは監視自体の無効化、maintenanceは監視を維持しながら外部への反映を抑制する機能とする。

## 11.6 pause状態の保存

pause状態は`data/state/monitor_states.json`の各Monitorの`paused`フラグで管理する。

```json
{
  "example-http": {
    "paused": true
  }
}
```

pause中は監視を実行しない。

pauseとmaintenanceの状態を混同しないこと。

手動メンテナンスは`schedules.json`に`origin: "manual"`として保存する。

---

# 12. Discord通知

## 12.1 基本方針

Discord Webhook URLはユーザーが各監視対象リポジトリで設定できるようにする。

開発リポジトリ側でWebhook URLを固定しない。

Webhook URLはGitHub Actions Secretsで管理する。

Secret名：

`DISCORD_WEBHOOK_URL`

## 12.2 通知設定

`settings.json`で通知の有効・無効と通知条件を管理する。

```json
{
  "notifications": {
    "discord": {
      "enabled": true,
      "notify_on": {
        "incident_created": true,
        "incident_updated": true,
        "incident_resolved": true,
        "maintenance": false
      }
    }
  }
}
```

## 12.3 通知内容

最低限、以下を通知できるようにする。

- 障害発生
- 障害継続・更新
- 復旧
- メンテナンス開始
- メンテナンス終了

通知には以下を含める。

- 監視対象名
- 障害状態
- 発生時刻
- 復旧時刻
- Incident ID
- Issue URL
- 必要なエラー情報

## 12.4 エラー処理

- Webhook未設定なら通知をスキップする。
- 通知失敗によって監視全体を失敗させない。
- HTTP 429ではRetry-Afterなどのレート制限情報に従う。
- Webhook URLをログに出さない。
- 無効なWebhook URLは警告として記録する。
- 同じイベントの重複通知を防ぐ。

## 12.5 通知方式

Discord通知は監視エンジンから独立させる。

```text
internal/notifier/
└── discord/
```

将来的にSlackやメール通知を追加できる設計にする。

---

# 13. Outboxパターンによる通知配送管理

## 13.1 基本方針

通知の欠落と重複を抑制するため、Outboxパターンを採用する。

監視イベントをGitに先に保存し、その後通知を送信する。

通知送信後に送信済み状態をGitに保存する。

## 13.2 基本フロー

1. IncidentやMaintenanceなどのイベントを生成する。
2. 未送信イベントをOutboxに記録する。
3. OutboxをGitにコミットする。
4. Discord通知を送信する。
5. 成功したイベントを送信済みに更新する。
6. 更新したOutboxをGitにコミットする。

## 13.3 Outboxデータ

イベントには最低限以下を持たせる。

- Event ID
- Event Type
- Aggregate ID
- Created At
- Payload
- Delivery Status
- Attempt Count
- Last Attempt At
- Last Error
- Delivered At

状態：

- pending
- delivered
- failed

## 13.4 重複送信対策

Discord Webhookは一般的にアプリケーション側の冪等性キーを提供しないため、送信成功後にGitへの保存が失敗した場合、重複送信が起こる可能性がある。

以下を実装する。

- Event IDによる識別
- 送信試行履歴
- 再試行回数制限
- 送信状態の永続化
- 失敗理由の記録
- 重複送信の可能性を考慮した通知内容

Outboxを採用したからといって、外部通知の完全なExactly-once配信を保証できると説明してはいけない。

実際に保証できる配送セマンティクスを明示する。

## 13.5 Maintenanceとの連携

メンテナンス中に発生した通知イベントは、すぐに送信せず保留する。

メンテナンス終了時にIncident状態と照合し、必要な通知を生成・送信する。

メンテナンス中に発生して終了前に復旧した障害について、通知するかどうかは通知設定で制御できる設計にする。

---

# 14. 稼働率・統計・レポート

## 14.1 Availability

```text
Availability =
(Operational + Degraded) / EligibleObservations
```

## 14.2 Health

```text
Health =
Operational / EligibleObservations
```

## 14.3 Coverage

```text
Coverage =
ObservedChecks / ExpectedChecks
```

以下を考慮する。

- Maintenance期間は稼働率の分母から除外する。
- unknownを正常扱いしない。
- unsupportedは別集計にする。
- 未実行はCoverageに反映する。
- 未実行を自動的にdowntime扱いしない。
- `interval`に応じてExpectedChecksを計算する。
- Workflowの実行遅延や欠落を考慮する。
- pause中の期間は監視対象外として扱う。
- Monitorが有効化された時刻と無効化された時刻を考慮する。

ExpectedChecksは単純なWorkflow実行回数から算出しない。

Monitorごとのinterval、監視有効期間、Maintenance、pauseをもとに計算する。

## 14.4 Latency統計

以下を計算する。

- Average
- Minimum
- Maximum
- P50
- P95
- P99

## 14.5 レポート

日次・月次レポートをJSONとして保存する。

保持期間は`report_retention_days`で管理する。

Webダッシュボードやグラフ表示ページは基本機能に含めない。

---

# 15. JSON設定仕様

## 15.1 monitors.json

監視対象の設定を保存する。

例：

```json
{
  "schema_version": 1,
  "monitors": [
    {
      "id": "example-http",
      "name": "Example Website",
      "type": "http",
      "enabled": true,
      "target": "https://example.com",
      "interval": "20m",
      "timeout": "10s",
      "latency": {
        "warning_ms": 500,
        "critical_ms": 1500
      },
      "authentication": {
        "secret_ref": "EXAMPLE_API_AUTH"
      }
    },
    {
      "id": "example-dns",
      "name": "Example DNS",
      "type": "dns",
      "enabled": true,
      "target": "example.com",
      "interval": "10m",
      "resolvers": [
        "system",
        "google",
        "cloudflare"
      ],
      "record_types": [
        "A",
        "AAAA"
      ]
    }
  ]
}
```

## 15.2 settings.json

全体設定を保存する。

設定項目：

- schema_version
- workflow_interval
- interval_tolerance
- timeout
- max_concurrency
- failure_threshold
- recovery_threshold
- history_retention_days
- archive_retention_days
- report_retention_days
- notifications
- maintenance
- その他の共通設定

初期値：

| 設定 | 初期値 |
|---|---:|
| Workflow周期 | 10分 |
| Monitor interval最小値 | 10分 |
| Monitor interval単位 | 10分の整数倍 |
| 連続失敗閾値 | 3回 |
| 復旧成功閾値 | 2回 |
| Timeout | 10秒 |
| 履歴保持 | 30日 |
| アーカイブ保持 | 180日 |
| レポート保持 | 180日 |

## 15.3 監視対象の認証情報

HTTP監視などで使用する認証情報は、`monitors.json`に直接記述しない。

1つのSecretにJSON形式のマップとしてまとめる。

Secret名：

`INALY_MONITOR_SECRETS`

例：

```json
{
  "EXAMPLE_API_AUTH": {
    "type": "bearer",
    "token": "SECRET_TOKEN"
  },
  "INTERNAL_BASIC_AUTH": {
    "type": "basic",
    "username": "example",
    "password": "SECRET_PASSWORD"
  }
}
```

`monitors.json`では以下のように参照する。

```json
{
  "authentication": {
    "secret_ref": "EXAMPLE_API_AUTH"
  }
}
```

Secretの値はGoアプリケーション内で安全に解析し、参照名によって対応する認証情報を取得する。

認証情報を以下に出力しない。

- ログ
- JSON監視結果
- Git Commit
- エラー詳細
- Issue本文
- Discord通知

SecretのJSON形式が不正な場合は、監視開始前に設定エラーとして検出する。

## 15.4 設定検証

以下を検証する。

- JSON形式が正しいこと。
- 必須項目が存在すること。
- Monitor IDが一意であること。
- Checker Typeが登録済みであること。
- IntervalとTimeoutが有効であること。
- Intervalが10分以上の10分単位であること。
- 数値が許容範囲内であること。
- 未知の設定項目を適切に扱うこと。
- 不正なResolver指定を拒否すること。
- Secret参照先が存在すること。
- 退役済みMonitor IDが使用されていないこと。

**既知のschema_versionにおいて未知の設定項目が存在する場合はエラーとする。**

未知の項目を無視して処理を継続してはいけない。

## 15.5 設定変更への対応

- 設定ファイルのバージョンを管理する。
- schema_versionを使用する。
- 将来の変更に備えてMigration機構を設計する。
- 削除された監視対象の過去履歴を即時削除しない。
- Monitor IDの再利用を禁止する。
- 退役IDをmetadata.jsonで管理する。

---

# 16. データ保存構造

監視対象リポジトリには以下の構成でデータを保存する。

```text
data/
├── state/
│   ├── current.json
│   ├── monitor_states.json
│   └── metadata.json
│
├── incidents/
│   ├── active.json
│   └── archive/
│
├── history/
│   └── YYYY/
│       └── MM/
│           └── DD.json
│
├── maintenance/
│   └── schedules.json
│
├── reports/
│   ├── daily/
│   └── monthly/
│
└── outbox/
    ├── pending.json
    └── delivered/
```

それぞれのファイルの責務を明確にする。

- `current.json`：最新の監視結果
- `monitor_states.json`：監視対象の現在状態、失敗カウンタ、復旧カウンタ、前回実行時刻、pausedフラグ
- `metadata.json`：データ管理情報、退役Monitor ID、schema情報
- `active.json`：継続中の障害
- `archive/`：解決済み障害の記録
- `history/`：時系列監視結果
- `schedules.json`：メンテナンス予定
- `reports/`：集計結果
- `outbox/`：未送信・送信済み通知イベント

JSONファイルの肥大化を避け、日付単位で履歴を分割する。

---

# 17. セキュリティ

ネットワーク監視では、監視対象を利用した意図しない外部アクセスを防止する。

以下を必須要件とする。

- localhostへのアクセス拒否
- `0.0.0.0`へのアクセス拒否
- Private IPへのアクセス制限
- Link-local IPへのアクセス制限
- 不正なURLの拒否
- DNS解決後のIP再検証
- DNS Rebinding対策
- HTTP Redirect先の再検証
- Timeout設定
- 最大並列数制限
- Request Bodyのサイズ制限
- 不正なポート指定の拒否
- 危険なポートのdenylist
- Secretsのログ出力禁止
- Discord Webhook URLの保護
- GitHub App Private Keyの保護
- 外部入力による任意コマンド実行の禁止

## 17.1 ポート制限

ポート番号が以下の場合は拒否する。

- 0
- 65536以上
- 許可しない範囲外の値

加えて、危険なポートのdenylistを用意する。

denylistの内容は設定として管理し、理由と対象をドキュメント化する。

単に数値だけで拒否するのではなく、監視方式とポートの用途も考慮する。

## 17.2 IP制限

以下を拒否する。

- localhost
- unspecified address
- Private IP
- Link-local IP
- 不正なIP
- 不許可の特殊用途IP

加えて、直接IPアドレスを指定する場合の許可リストを設ける。

ユーザー指定の許可候補：

- `1.1.1.1`
- `8.8.8.8`
- `9.9.9.9`

IPv4/IPv6の直接指定については、許可リストを適用する。

## 17.3 ドメイン監視

ドメイン名の指定は通常許可する。

必要に応じてブラックリストを追加できる設計にする。

ただし、ドメインを許可した場合でも、DNS解決後のIPアドレスに対する安全性検証を省略してはならない。

DNS Rebinding対策として、名前解決結果を接続直前にも検証する。

HTTP Redirectが発生した場合は、遷移先のドメインとIPアドレスも再検証する。

**重要：直接指定IPの許可リストと、ドメイン名から解決された接続先IPの制限は別のポリシーとして実装する。**

指定された3つ以外のIPを全面的に拒否すると、通常のWebサイト監視が成立しない可能性がある。

そのため、実装前に以下の2方式のどちらを採用するか確認する。

A. 直接IP指定だけを3つに制限し、ドメイン名については安全な公開IPへの接続を許可する。

B. DNS解決後の接続先IPも3つに制限する。この場合、通常のWebサイト監視は大きく制限される。

この選択を勝手に決定しないこと。

## 17.4 DNS Resolverと許可IP

DNS監視で使用するResolverは以下とする。

- OS標準
- `8.8.8.8`
- `1.1.1.1`

`9.9.9.9`はIP許可候補として挙げられているが、DNS Resolverとしての採用は別途決定する。

---

# 18. コマンド機能

将来的にGitHub Issuesのコメントから以下のコマンドを実行できる設計にする。

```text
/status
/check
/check ID
/uptime
/uptime ID
/incidents
/history ID
/maintenance
/maintenance list
/maintenance start ID 2h
/maintenance stop ID
/maintenance global 1h
/pause ID
/resume ID
/help
```

将来の拡張：

```text
/ack ID
```

`/ack`はIncidentを`investigating`に遷移させる用途を想定する。

初期実装ではIncident Stateを`open`と`resolved`のみとし、`investigating`は実装しない。

コマンド実行時には以下を確認する。

- コメント投稿者の権限
- 対象リポジトリ
- コマンドの形式
- 引数の妥当性
- 実行可能な操作

初期実装では、コマンド機能を監視エンジンから独立したパッケージとして設計する。

---

# 19. 外部ライブラリ方針

## 19.1 基本方針

外部ライブラリは最小限の使用を許可する。

Go標準ライブラリで実装可能な機能は、原則として標準ライブラリを利用する。

ただし、保守性、安全性、実装品質が向上する場合は外部ライブラリを利用してよい。

## 19.2 使用を許可するライブラリ

以下は初期設計で使用を認める。

| 用途 | ライブラリ |
|---|---|
| DNS | `github.com/miekg/dns` |
| ICMP Ping | `golang.org/x/net/icmp` |
| HTML解析 | `golang.org/x/net/html` |

## 19.3 準標準ライブラリ

以下の名前空間で始まるライブラリは準標準ライブラリとして扱う。

- `golang.org/x/...`
- `github.com/google/...`

これらは事前確認なしで使用してよい。

ただし、利用目的と必要性はコードレビューで確認すること。

その他の外部ライブラリを追加する場合は、以下を説明する。

- ライブラリ名
- 利用目的
- 標準ライブラリで代替できない理由
- メンテナンス状況
- セキュリティ上の懸念
- ライセンス

---

# 20. GitHub Actionsの注意点

以下を考慮する。

- `schedule`はUTC基準。
- GitHub-hosted Runnerを使用する。
- スケジュール実行は遅延や欠落が発生する可能性がある。
- `concurrency`は完全なFIFOキューではない。
- 同時実行によるGit更新競合を防止する。
- Workflowの権限を最小化する。
- Actionは可能な限りCommit SHAで固定する。
- Public RepositoryのReusable Workflowを利用する。
- Secretsの受け渡しを明示する。
- Workflow実行時間とAPIレート制限を考慮する。
- 10分周期を前提にMonitor Intervalを評価する。
- 実際の前回実行時刻を保存する。
- Git更新とOutbox処理を冪等にする。

複数の監視要求を確実に処理する必要がある場合は、Gitベースのキューを検討する。

---

# 21. テスト方針

実装した機能にはテストを作成する。

## 21.1 Checker

- HTTP正常応答
- HTTP異常応答
- HTTP Timeout
- HTTP Redirect
- HTTP認証
- TCP接続成功
- TCP接続失敗
- TCP Timeout
- DNS正常応答
- DNS NXDOMAIN
- DNS SERVFAIL
- DNS Timeout
- DNS複数Resolver比較
- 一部Resolver失敗
- 全Resolver失敗
- Ping成功
- Ping非対応
- Latency閾値判定

## 21.2 Monitor Interval

- 10分Interval
- 20分Interval
- 30分Interval
- 10分未満のInterval
- 10分の整数倍でないInterval
- 前回実行時刻の取得
- 実行許容差
- Workflow遅延
- Workflow欠落
- 手動実行
- 二重実行防止
- 監視予定時刻の欠落

## 21.3 Incident

- 障害発生
- 失敗1回目
- 失敗2回目
- 失敗3回目
- 障害継続
- 障害復旧
- 復旧後の再障害
- Issue作成失敗
- Issue更新失敗
- 重複Issue防止
- 手動Issueクローズ
- Issue削除
- Issue再作成
- Incident IDの維持

## 21.4 Maintenance

- Global Maintenance
- Monitor Maintenance
- Scheduled Maintenance
- Manual Maintenance
- 固定期間
- UTC日時
- skip
- suppress
- メンテナンス中の障害発生
- メンテナンス中の復旧
- メンテナンス終了後のIssue作成
- メンテナンス終了後のIssueクローズ
- メンテナンス終了後の通知

## 21.5 Storage

- JSON読み込み
- JSON保存
- Git Commit作成
- Git Ref更新
- SHA競合
- 同時更新
- 再試行
- 変更なし
- 履歴削除
- アーカイブ削除
- レポート削除
- Outbox保存
- Outbox更新
- 不正データ

## 21.6 GitHub App

- JWT生成
- Installation取得
- Installation Token発行
- Token期限切れ
- 認証失敗
- 権限不足
- APIレート制限

## 21.7 Discord

- 正常通知
- Webhook未設定
- 無効なWebhook
- HTTP 429
- HTTP 500
- 通知失敗時の監視継続
- Secretsのログ漏洩防止
- Outbox再試行
- 重複送信の可能性
- 送信成功後のGit更新失敗

## 21.8 Security

- Private IP拒否
- localhost拒否
- unspecified address拒否
- Link-local拒否
- 許可IP検証
- DNS Rebinding対策
- Redirect先検証
- 不正URL拒否
- 不正ポート拒否
- denylist検証
- Timeout
- 並列数制限
- Secret参照検証
- Secret漏洩防止

---

# 22. CI/CD

GitHub Actionsを利用して品質を確認する。

最低限、以下を実行する。

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

加えて以下を検討する。

- JSON Schema検証
- Workflow YAML検証
- 静的解析
- 脆弱性検査
- テストカバレッジ
- 依存関係検査

CIでテストが失敗した場合は、原因を明確に報告する。

テストを通すためだけにテスト内容を弱めたり、失敗を無視したりしてはならない。

---

# 23. 開発フェーズ

以下の順序で実装する。

## Phase 1：Architecture

- 全体アーキテクチャ
- Domain Model
- パッケージ構成
- Checker Interface
- Storage Interface
- 設定仕様
- データ構造
- 依存関係の整理
- Outbox設計
- セキュリティ設計

## Phase 2：Configuration

- JSON読み込み
- JSON検証
- JSON Schema
- 設定エラー処理
- Monitor ID退役管理
- Interval検証
- Secret参照管理
- Migration設計

## Phase 3：Storage

- Git Storage Interface
- GitHub Git Data API
- Blob / Tree / Commit / Ref
- SHA競合対策
- 履歴保持処理
- アーカイブ保持処理
- レポート保持処理

## Phase 4：GitHub App

- JWT
- Installation取得
- Installation Token
- GitHub API Client
- Issues操作
- Issue再作成処理
- Issue手動クローズ対応

## Phase 5：Monitoring Engine

- Checker共通基盤
- HTTP
- TCP
- DNS
- Ping
- Latency
- Network Security
- Monitor Interval管理
- 実行スケジューリング

## Phase 6：Incident Management

- 状態遷移
- 連続失敗カウンタ
- 復旧カウンタ
- Incident生成
- Issue作成
- Issue更新
- Issue再作成
- 復旧処理
- 重複防止

## Phase 7：Maintenance

- メンテナンス管理
- 固定期間スケジュール
- skip
- suppress
- pause
- 稼働率計算との連携
- メンテナンス中のIncident管理
- 終了後のIssue・通知反映

## Phase 8：Notifications

- Discord Webhook
- 通知条件
- エラー処理
- Outbox
- 再試行
- 重複送信対策

## Phase 9：Reports

- Availability
- Health
- Coverage
- Latency統計
- 日次レポート
- 月次レポート
- 保持期間管理

## Phase 10：GitHub Actions

- Reusable Workflow
- 定期実行
- 手動実行
- Secrets受け渡し
- Checkout
- 実行環境
- 権限設定
- 10分周期とMonitor Intervalの整合性

## Phase 11：Quality Assurance

- テスト
- セキュリティ検証
- CI
- ドキュメント
- リリース準備

---

# 24. 実装時の重要なルール

以下を厳守すること。

1. Architecture Firstを徹底する。
2. Domain層とInfrastructure層を分離する。
3. GitHub APIへの依存を適切に隔離する。
4. Checkerを追加しやすい設計にする。
5. Notificationを追加しやすい設計にする。
6. JSONのスキーマとバージョンを管理する。
7. Git更新の競合を考慮する。
8. 冪等性を確保する。
9. Secretsを安全に扱う。
10. エラーを握りつぶさない。
11. テストしやすい構造にする。
12. 既存の仕様を無断で変更しない。
13. 不明な仕様を勝手に補完しない。
14. 不要なWebサーバーやデータベースを追加しない。
15. 不要な外部サービスへの依存を増やさない。
16. GitHub Actionsの実行時間とAPI制限を考慮する。
17. 監視結果と障害状態の正本はGit上のJSONとする。
18. GitHub Issuesは障害の表示・管理手段とする。
19. Discord通知の失敗で監視処理全体を停止させない。
20. 各監視対象リポジトリの独立性を維持する。
21. Workflowの遅延や欠落を前提に設計する。
22. Monitor Intervalは前回実行時刻を基準に判定する。
23. Outboxを使って通知イベントを永続化する。
24. Issueの手動操作を無条件に上書きしない。
25. Maintenance中でもGit上のIncident情報を記録する。
26. 既知のschema_versionで未知の項目をエラーにする。
27. 外部通知のExactly-once配信を安易に保証しない。
28. 直接IP指定とドメイン解決先のセキュリティポリシーを分離する。

コードを作成する際は、ファイル単位で責務を明確にし、必要なテストも同時に作成する。

---

# 25. 最初に実施する作業

いきなり全機能を実装しないこと。

まず、以下を実施する。

1. 全要件を整理する。
2. 要件間の矛盾を検証する。
3. 未確定事項を抽出する。
4. 全体アーキテクチャを提示する。
5. パッケージ間の依存関係を提示する。
6. Domain Modelを定義する。
7. JSONスキーマを設計する。
8. Git Storage Interfaceを設計する。
9. Checker Interfaceを設計する。
10. GitHub Actionsの実行構造を設計する。
11. Monitor Intervalの実行モデルを設計する。
12. Incidentの状態遷移を定義する。
13. MaintenanceとIncidentの連携を設計する。
14. Outboxのデータ構造と配送モデルを設計する。
15. GitHub Appの権限を整理する。
16. セキュリティ上の問題点を整理する。
17. 外部ライブラリの必要性を確認する。
18. 実装順序を確定する。

設計段階では、必要に応じてMermaidの図を使用すること。

設計を提示した後、未確定事項についてユーザーの確認を得てから実装する。

その後、各Phaseを順番に実装する。

各Phaseの終了時には以下を報告する。

- 実装した機能
- 変更したファイル
- 設計上の判断
- 実行したテスト
- テスト結果
- 残っている課題
- 次のPhase

**最終目標は、GitHub Actions上で安定して動作し、各GitHubリポジトリに監視結果を独立して蓄積できる、拡張性と保守性に優れたGo製監視Bot「Inaly Pulse」を完成させることです。**

---

# 26. 実装開始前に確認が必要な事項

以下は確定事項ではなく、実装前に確認すること。

## 26.1 IP制限ポリシー

直接IP指定の許可候補は以下。

- 1.1.1.1
- 8.8.8.8
- 9.9.9.9

ただし、ドメイン名を通常許可する方針との整合性が必要。

以下から選択する。

- A：直接IP指定だけを上記3つに制限し、ドメイン名は安全な公開IPへの接続を許可する。
- B：DNS解決後の接続先IPも上記3つに限定する。

この選択によって監視可能な対象が大きく変わるため、必ず確認すること。

## 26.2 DNS Resolver

`9.9.9.9`はIP許可候補として挙げられているが、DNS Resolverとしての採用は未確定。

初期Resolverは以下の3つを維持する。

- OS標準
- Google Public DNS
- Cloudflare DNS

## 26.3 Outboxの保持期間

Outboxの送信済みイベントをどの程度保持するかは、履歴・アーカイブの保持期間と合わせて検討すること。

## 26.4 GitHub Appのブランチ保護バイパス

デフォルトブランチへの直接コミットを実現するために、GitHub Appへバイパス権限を付与する場合は、必要な権限範囲と監査方法を明確にすること。

---

**以上をInaly Pulseの開発仕様とする。**