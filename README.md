# primind-tasks

## API

### ヘルスチェック

GET `/health`

### タスク登録

POST `/tasks`
POST `/tasks/{queue}`

```json
{
  "task": {
    "httpRequest": {
      "body": "eyJtZXNzYWdlIjogIkhlbGxvIn0=",
      "headers": {
        "Content-Type": "application/json"
      }
    },
    "scheduleTime": "2025-12-17T10:00:00Z"
  }
}
```

`httpRequest.body`: base64エンコードのリクエストボディ
`httpRequest.headers`: 転送時に付与するHTTPヘッダー
`scheduleTime`: 実行時刻

response
```json
{
  "name": "tasks/abc123",
  "scheduleTime": "2025-12-17T10:00:00Z",
  "createTime": "2025-12-16T10:00:00Z"
}
```

```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": {
      "httpRequest": {
        "body": "eyJrZXkiOiAidmFsdWUifQ==",
        "headers": {"Content-Type": "application/json"}
      }
    }
  }'
```

```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": {
      "httpRequest": {
        "body": "eyJrZXkiOiAidmFsdWUifQ=="
      },
      "scheduleTime": "2025-12-17T10:00:00Z"
    }
  }'
```

## 環境変数

### 共通

| variable | desc | default |
|------|------|-----------|
| `REDIS_ADDR` | Redisアドレス | `localhost:6379` |
| `REDIS_PASSWORD` | Redisパスワード | `""` |
| `REDIS_DB` | Redis DB番号 | `0` |
| `QUEUE_NAME` | キュー名 | `default` |
| `RETRY_COUNT` | 最大リトライ回数 | `3` |

### APIサーバー

| variable | desc | default |
|------|------|-----------|
| `API_PORT` | 起動ポート | `8080` |

### ワーカー

| variable | desc | default |
|------|------|-----------|
| `TARGET_ENDPOINT` | 転送先HTTPエンドポイント |  |
| `WORKER_CONCURRENCY` | 並行処理数 | `10` |
| `REQUEST_TIMEOUT` | HTTPリクエストタイムアウト | `30s` |
