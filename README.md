# Inaly Pulse

GitHub Actions で動作する Go 製の分散型サービス監視 Bot です。現在はローカルで設定を検証し、HTTP / TCP / DNS チェックを実行する初期実装です。

## ローカルで試す

```powershell
go run .
```

初回は `config/monitors.json` を作成して終了します。対象設定を編集してから、設定検証と実行を行います。

```powershell
go run .
go run . -run
```

`-run` を付けない場合は設定の検証だけを行います。定期実行では `data/state/monitor_states.json` の前回実行時刻を使い、期限到来した Monitor だけを実行します。手動の強制チェックには `-force` を付けます。

```powershell
go run . -run -force
go run . -config config/monitors.json
```

Windows PowerShell で1回だけ試す場合も同じです。結果の書き込み先は `-data-dir` で一時フォルダーに変更できます。

```powershell
go run . -run -force -data-dir .tmp-pulse-data
```

チェック結果は `data/state/current.json`、日ごとの履歴は `data/history/YYYY/MM/DD.json`、状態は `data/state/monitor_states.json` に保存します。監視対象の失敗はJSON結果として保存します。

## GitHub Actionsで使う

GitHub Appの作成とインストール後、対象リポジトリのRepository variableに `INALY_APP_ID`、Actions secretsに `INALY_APP_PRIVATE_KEY` を登録します。認証付きHTTP監視を使うときは `INALY_MONITOR_SECRETS` も登録します。AppにはMetadata read、Contents writeを設定し、対象リポジトリだけにインストールしてください。

対象リポジトリに [`docs/target-workflow.yml`](docs/target-workflow.yml) の内容を `.github/workflows/inaly-pulse.yml` として配置します。Reusable workflowは公開リポジトリ `LingXuan-Li/inaly_pulse` の `main` branchを参照します。独自forkやtagを使う場合はworkflowの参照先を変更してください。

認証したAppはデフォルトブランチへ監視JSONを直接コミットします。ブランチ保護を使う場合、Appに限定したbypassを設定するか、書き込みが拒否される構成を解消する必要があります。App ID・秘密鍵を設定する前のローカル実行はローカルJSONだけを更新します。

## 設定例

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
      "interval": "10m",
      "timeout": "10s",
      "http": {"method": "GET", "expected_status": [200]}
    },
    {
      "id": "example-tcp",
      "name": "Local TCP service",
      "type": "tcp",
      "enabled": false,
      "target": "127.0.0.1:8080",
      "interval": "20m",
      "timeout": "3s"
    },
    {
      "id": "example-dns",
      "name": "Example DNS",
      "type": "dns",
      "enabled": true,
      "target": "example.com",
      "interval": "30m",
      "dns": {"record_type": "A", "resolvers": ["system", "google", "cloudflare"]}
    }
  ],
  "settings": {
    "tolerance": "2m",
    "failure_threshold": 3,
    "recovery_threshold": 2
  }
}
```

Interval は10分以上の10分単位です。`ping` はこの初期実装では拒否します。DNSはOS標準、Google、Cloudflare Resolverに問い合わせますが、TTL/RCODEは取得しません。GitHub App認証とGit Data API保存は実装済みです。GitHub IssuesによるIncident管理、Maintenance、Outbox/Discord通知、レポート集計はまだ未実装です。本番運用に必要なこれらの仕様は後続実装が必要です。

## 開発

```bash
go fmt ./...
go vet ./...
go test ./...
go build ./...
```
