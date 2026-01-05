# Primind Tasks

AsynqベースのCloud Tasks風タスクキュー

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
`name`: タスクID（オプション、重複排除用）  

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

`task.name` 指定で重複排除ができる

```bash
curl -X POST http://localhost:8080/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": {
      "name": "my-unique-task-id",
      "httpRequest": {
        "body": "eyJrZXkiOiAidmFsdWUifQ=="
      }
    }
  }'
```

同じ `name` で再度リクエストした場合、409 Conflictエラー

```json
{
  "error": {
    "code": 409,
    "message": "task with name \"my-unique-task-id\" already exists",
    "status": "ALREADY_EXISTS"
  }
}
```

`name` を指定しない場合は、IDは自動生成

### タスク削除

DELETE `/tasks/{taskId}`
DELETE `/tasks/{queue}/{taskId}`

キューに登録されたタスクを削除する  
タスクがpending/scheduled/retry状態の場合は即座に削除  
active状態の場合はキャンセルを試みる（ベストエフォート）

response (成功時)
```json
{}
```

```bash
# デフォルトキューから削除
curl -X DELETE http://localhost:8080/tasks/my-task-id

# 指定キューから削除
curl -X DELETE http://localhost:8080/tasks/my-queue/my-task-id
```

タスクが見つからない場合、404 Not Foundエラー

```json
{
  "error": {
    "code": 404,
    "message": "task \"my-task-id\" not found in queue \"default\"",
    "status": "NOT_FOUND"
  }
}
```

### Proto定義

- `proto/taskqueue/v1/taskqueue.proto`

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

## 依存

- Redis v8
- Asynq

## モニタリング

- **asynqmon**: タスクキューのWeb UIモニタリング（ポート8081）
