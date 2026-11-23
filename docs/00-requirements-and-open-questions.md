# 要件整理と未確定事項(Phase 0)

対象仕様: `AGENTS.md`。確定済みの設計方針(仕様2章)は変更しない。
ここでは「要件の整理」「仕様の矛盾・不明点」「実機で確認が必要な前提」をまとめる。
不明点は勝手に決めず、選択肢と推奨案を示す。**推奨案は未承認であり、確認が取れるまで実装しない。**

## 1. 要件整理

| 領域 | 主な要件 | 仕様章 | Phase |
|---|---|---|---|
| アーキテクチャ | Domain層とInfrastructure層の分離、Checker / Storage / Notifier の抽象化 | 7, 8.1, 22 | 1 |
| 設定 | monitors.json / settings.json、検証、schema_version、Migration | 14 | 2 |
| Gitストレージ | Git Data API で複数ファイルを1コミット、SHA競合の再試行、冪等、履歴保持 | 6, 15 | 3 |
| GitHub App | JWT → Installation → Installation Token、最小権限 | 5 | 4 |
| 監視エンジン | HTTP / TCP / DNS / Ping / Latency、SSRF対策 | 8, 16 | 5 |
| 障害管理 | 状態遷移、Incident、Issue作成・更新・クローズ、重複防止 | 9, 10 | 6 |
| Maintenance | 4種類、skip / suppress(既定 suppress)、稼働率からの除外 | 11 | 7 |
| 通知 | Discord Webhook、通知条件、429対応、重複防止 | 12 | 8 |
| レポート | Availability / Health / Coverage、Latency統計、日次・月次JSON | 13 | 9 |
| Actions | Reusable Workflow、schedule / dispatch、Secrets、concurrency | 4, 18 | 10 |
| 品質 | テスト、CI、セキュリティ検証 | 19, 20 | 11 |
| コマンド(将来) | Issueコメントによる /status など。監視エンジンから独立したパッケージ | 17 | 設計のみ |

## 2. 仕様の矛盾・不明点

「ブロック」は、その Phase に入る前に決める必要があることを示す。

### Q1. Secret名が `GITHUB_` で始まっている(ブロック: Phase 10)

仕様4.3・5.4の `GITHUB_APP_PRIVATE_KEY` は、GitHub の Secrets 名の制約(`GITHUB_` で始まる名前は使えない)に抵触する。
- 選択肢A: `INALY_APP_PRIVATE_KEY` にする(推奨)
- 選択肢B: 別の名前を指定する
- Webhook 用の `DISCORD_WEBHOOK_URL` は問題ない。

### Q2. `degraded` の定義と連続失敗の扱い(ブロック: Phase 6)

9.2の遷移図は「連続失敗 → degraded → 失敗条件成立 → outage」だが、同じ節で「接続成功だが性能が悪い場合は degraded」とも書かれている。
- 選択肢A: 失敗1〜2回は直前の状態を維持して失敗カウンタだけ増やし、3回で outage。degraded は Latency 超過専用(推奨)
- 選択肢B: 失敗1〜2回で degraded、3回で outage。degraded が「失敗の途中」と「遅い」の2つの意味を持つ
- どちらの場合も、`unsupported` と `skipped` は失敗カウンタに加算しない(`CheckStatus.CountsAsFailure` で実装済み)。

### Q3. 監視間隔(`interval`)とWorkflowの実行周期(ブロック: Phase 2, 5, 9)

monitors.json には監視対象ごとの `interval` があるが、起動は15分固定の cron で、GitHub Actions の schedule は遅延・欠落しうる。
Coverage の `ExpectedChecks` も interval に依存する。
- 選択肢A: interval は Workflow 周期の整数倍のみ許可。前回実行から `interval − 許容差` 以上経過した監視対象だけを実行する(推奨)
- 選択肢B: interval を廃止し、毎回すべて実行する
- 選択肢C: Workflow 周期より短い interval を許可する(cron を増やすため、Actions の実行時間と API 制限の面で非推奨)

### Q4. データの保存先ブランチ(ブロック: Phase 3, 10)

仕様は `config/` と `data/` を同じツリーに置くとしているが、ブランチは未指定。15分ごとにコミットされ、ブランチ保護とも衝突しうる。
- 選択肢A: デフォルトブランチに直接コミット。ブランチ保護で GitHub App のバイパスが必要(推奨ではないが仕様に最も近い)
- 選択肢B: `data/` 専用ブランチ(例: `inaly-pulse-data`)にコミットし、`config/` はデフォルトブランチに置く。ツリー構成は仕様どおりだがブランチが分かれる

### Q5. 開発リポジトリの公開範囲と OWNER(ブロック: Phase 10)

Private の場合、Reusable Workflow の利用許可と Bot 本体の Checkout 権限が必要になる(仕様4.3, 18)。
- 必要な情報: リポジトリの Owner 名、Public / Private のどちらか
- `go.mod` のモジュール名は現在 `inaly_pulse`(Owner が決まれば `github.com/OWNER/inaly_pulse` に変更できる)

### Q6. Issue が手動削除された場合(ブロック: Phase 6)

GitHub REST API には Issue 削除がなく、削除済みの Issue は 404 / 410 で見える。
- 選択肢A: 同じ Incident ID のまま新しい Issue を作成して対応付けを更新し、本文に再作成した旨を記載する(推奨)
- 選択肢B: 対応付けを「切り離し」にして、Git 上の Incident だけで継続する(Issue は作らない)

### Q7. Issue が手動でクローズされた場合(ブロック: Phase 6)

- 選択肢A: 障害継続中なら再オープンして追記する
- 選択肢B: クローズのまま、コメントだけ追記し続ける(推奨。人の操作を覆さない)

### Q8. メンテナンス中の障害・復旧(ブロック: Phase 7)

suppress 中は「新規Issueを作らない」「既存Issueを自動クローズしない」と決まっているが、次が未定義。
- メンテナンス中に新規の障害が成立した場合: Git 上の Incident は記録するのか。メンテナンス終了後もまだ障害なら Issue を作るのか。
- メンテナンス中に復旧した場合: Git 上の Incident を resolved にして、Issue のクローズだけ終了後に行うのか。
- 推奨: Git 上の Incident は常に記録する。Issue の作成・クローズ・通知は終了後にまとめて反映する。

### Q9. 外部依存(ブロック: Phase 5)

「不要な外部サービス・依存を増やさない」と、DNS の TTL / RCODE の記録(8.5)が緊張関係にある。標準の `net.Resolver` は TTL も RCODE も返さないため、`system` Resolver では記録できない(null とする)。
- 選択肢A: 最小限の外部ライブラリを許可する。DNS は `github.com/miekg/dns`、Ping は `golang.org/x/net/icmp`、HTML 検証は `golang.org/x/net/html`(推奨)
- 選択肢B: 標準ライブラリのみ。DNS は wire format を自前実装、Ping は OS の `ping` を固定引数で実行、HTML は簡易検証にとどめる(実装・保守コストが大きい)

### Q10. DNS監視の判定ルール(ブロック: Phase 5)

仕様は「応答差異だけで障害と判定しない」とだけ書いている。何をもって失敗とするかが未定義。
- 選択肢A: 全 Resolver が失敗なら failure、一部だけ失敗なら success で details に警告、応答差異は情報のみ(推奨)
- 選択肢B: A に加えて、設定で期待値(`expect`)を指定した場合のみ不一致を failure にする(設定項目の追加が必要。opt-in)

### Q11. 監視対象の認証情報の渡し方(ブロック: Phase 5, 10)

HTTP監視は Basic / Bearer 認証に対応するが、monitors.json はリポジトリ内のファイルで、認証情報を直接書けない。仕様5.4が定める Secrets は2つだけ。
- 選択肢A: 1つの Secret(例: `INALY_MONITOR_SECRETS`、JSON のマップ)にまとめ、monitors.json は `"secret_ref": "NAME"` で参照する(推奨)
- 選択肢B: `secrets: inherit` を使い、名前で環境変数を引く(呼び出し側の全 Secret が見える)
- 選択肢C: 認証付き監視は初期実装では対象外にする

### Q12. アーカイブ・レポートの保持期間(ブロック: Phase 3, 9)

仕様6.4は「監視履歴とは別の保存ポリシーを定義する」とだけ書いている。
- 推奨: 既定は無期限。任意の設定 `archive_retention_days` / `report_retention_days` を追加する

### Q13. Monitor ID 再利用の検出(ブロック: Phase 2)

仕様14.4は「ID再利用で過去の障害と混同しない」としているが方法は未定。
- 選択肢A: 削除した ID を `data/state/metadata.json` の退役リストに記録し、検証で再利用を拒否する(推奨)
- 選択肢B: Incident ID に設定の世代番号を含める
- 選択肢C: 警告のみ

### Q14. pause と手動メンテナンスの保存先(ブロック: Phase 7 / コマンド機能)

仕様15章のデータ構造に、pause 状態の置き場所がない。手動メンテナンスは `schedules.json` に `origin: manual` で保存できる。
- 選択肢A: pause は `monitor_states.json` の `paused` フラグで管理する(新規ファイル不要。推奨)
- 選択肢B: `data/state/overrides.json` を新設する

### Q15. `investigating` への遷移(ブロック: Phase 6)

Incident State に `investigating` があるが、誰がいつ遷移させるか未定義。
- 推奨: 初期実装は `open → resolved` のみ。`investigating` は将来のコマンド(例: `/ack`)で遷移させる。

### Q16. 未知の設定項目の扱い(ブロック: Phase 2)

- 選択肢A: 既知の schema_version では未知の項目をエラーにする(誤字を検出できる。推奨)
- 選択肢B: 警告にして無視する(将来互換性は高いが誤字に気づきにくい)

### Q17. Scheduled Maintenance の形式(ブロック: Phase 7)

固定期間(開始・終了)だけか、繰り返し(cron + 継続時間 + タイムゾーン)も含むか。
- 推奨: 初期は固定期間(UTC)のみ。繰り返しは schema_version を上げて拡張する。

### Q18. 通知の配送保証(ブロック: Phase 8)

コミット後に通知する方式では、クラッシュ時に通知が欠落する。通知後にコミットする方式では重複する。
- 選択肢A: コミット後に通知(at-least-once 寄りだが欠落しうる)。`DedupeKey` を通知に埋め込む
- 選択肢B: 未送信イベントを Git に先に記録(outbox)し、送信後に送信済みへ更新する。イベント発生時のみ追加コミットが必要(推奨)

### Q19. 禁止ポート・禁止IP範囲(ブロック: Phase 5)

仕様16章は localhost / Private / Link-local の拒否を必須としている。
- ポート: 範囲外(0, 65536以上)のみ拒否するか、危険なポートの denylist も設けるか。推奨は範囲外のみ拒否し、任意の `allowed_ports` 設定を追加。
- IP: 次も同趣旨で拒否することを提案する(仕様の範囲拡張にあたるため確認が必要): 未指定アドレス(0.0.0.0, ::)、マルチキャスト、CGNAT(100.64.0.0/10)、IPv6 ULA、IPv4-mapped IPv6 の正規化。

## 3. 実機で確認が必要な前提

- **V1: ICMP の可否**。GitHub-hosted Runner で ICMP Echo が使えるかは、権限(raw socket / unprivileged ping socket)とネットワーク経路の両方に依存し、事前には断定できない。Phase 5 の冒頭に実機で確認する probe を実行し、結果で実装を確定する。使えない場合は仕様8.6どおり `unsupported` とする。
- **V2: schedule の遅延・欠落**。仕様どおり Coverage で可視化する。実測値は運用後に確認する。

## 4. 検出した矛盾のうち、仕様の記述だけでは解消できないもの

Q1(Secret名)は GitHub 側の制約によるため、確認が取れるまで Phase 10 のWorkflowは確定できない。
Q2(degraded)は、状態遷移を Domain 層に一元化する(仕様9.2)ための前提で、Phase 6 の実装内容そのものが変わる。
