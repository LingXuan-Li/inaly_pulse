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

チェック結果は `data/state/current.json`、日ごとの履歴は `data/history/YYYY/MM/DD.json`、状態は `data/state/monitor_states.json` に保存します。監視対象の失敗は JSON 結果として出力し、設定・内部エラーは終了コード 1 になります。

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

Interval は10分以上の10分単位です。`ping` は GitHub-hosted runner 上での実行可否が未確定のため、この初期実装では拒否します。現段階の DNS チェックでは OS 標準 Resolver を使います。TTL / RCODEを取得するDNS wire実装、GitHub App / Git Data API、Issues、Maintenance、Outbox、Discord、レポート集計、SSRF防御は未実装であり、Actionsへ本番接続する前に必要です。

## 開発

```bash
go fmt ./...
go vet ./...
go test ./...
go build ./...
```
