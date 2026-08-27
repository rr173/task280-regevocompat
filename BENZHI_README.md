# BENZHI 评测说明

基于 Go 实现的跨区域数据库模式演进兼容验证后端服务，一款后端服务，完成区域副本与 schema 版本登记、跨版本读写语义模拟与冲突路径定位、兼容窗口裁决与不可变兼容快照发布。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/regevocompat --addr :8080 --db regevocompat.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/regevocompat --smoke-test
```

`--smoke-test` 会真实登记 schema 版本与区域副本、执行字段拆分双写与停止双写、定位滞后区域冲突、声明兼容窗口消解、发布不可变兼容快照，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/regevocompat --smoke-test
```

## HTTP API（前缀 /api）

schema：`POST /api/schema-versions`、`GET /api/schema-versions`、`GET /api/schema-versions/{id}`
区域：`POST /api/regions`、`GET /api/regions`、`GET /api/regions/{id}`、`POST /api/regions/{id}/upgrade`、`POST /api/regions/{id}/set-version`
计划：`POST /api/plans`、`GET /api/plans`、`GET /api/plans/{id}`、`POST /api/plans/{id}/verify`、`POST /api/plans/{id}/explore`、`POST /api/plans/{id}/seal`、`GET /api/plans/{id}/conflicts`
步骤：`POST /api/plans/{id}/steps`、`GET /api/plans/{id}/steps`、`POST /api/steps/{id}/advance`
语义：`POST /api/semantics`、`GET /api/semantics`
窗口：`POST /api/plans/{id}/windows`、`GET /api/plans/{id}/windows`、`POST /api/windows/{id}/revoke`
快照：`POST /api/plans/{id}/snapshots`、`GET /api/plans/{id}/snapshots`、`POST /api/snapshots/{id}/supersede`
自检：`GET /api/health`、`POST /api/example`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：schema_versions、region_replicas、migration_plans、migration_steps、read_write_semantics、compat_windows、conflict_paths、compat_snapshots、sample_records、audit_events。同计划验证串行；已发布快照不可改写，仅允许 supersede。
