# 跨区域数据库模式演进兼容验证服务（task280-regevocompat）

面向数据库工程师的后端服务：在分区域逐步升级数据库 schema 期间，验证读写请求与
schema 版本组合是否保持语义一致。服务登记区域副本、schema 版本、迁移步骤与读写
语义，模拟跨版本读写路径，定位不兼容组合（如某区域仍用旧读路径、另一区域已停止
双写），支持声明兼容窗口（读路径适配器）并发布不可变兼容快照。

## 业务闭环

1. 登记 schema 版本（字段定义 + 内容哈希）与区域副本（当前运行的 schema 版本）。
2. 登记演进计划与迁移步骤（字段拆分、双写、回填、收尾、停止双写、撤销）。
3. 登记读写语义（每个 schema 版本对应读路径 / 写路径）。
4. 模拟跨版本读写：写路径按迁移阶段产出字段集合，读路径按区域当前版本解释记录。
5. 探索冲突路径：枚举「区域读版本 × 写阶段字段集合」组合，定位无法解释的记录字段。
6. 声明兼容窗口（读路径适配器，如 `customer_name = first_name + " " + last_name`）消除冲突。
7. 发布不可变兼容快照，封存计划版本。

## 状态机

- 演进计划：`draft → verifying → conflicted | publishable → sealed`（sealed 终态，不可改写）。
- 区域副本：`old → transitional → upgraded | lagging`（lagging 表示落后于目标版本）。
- 迁移步骤：`pending → dual_write → backfill → finalize | rollback`。
- 兼容快照：`draft → published → superseded`。

## 标准命令

```bash
export GOTOOLCHAIN=local CGO_ENABLED=0
go build ./...
go vet ./...
go test ./...
go run ./cmd/regevocompat --addr :8080 --db regevocompat.db
go run ./cmd/regevocompat --smoke-test   # 不启动长驻服务，自检后退出码 0
```

## API 入口

所有接口前缀 `/api`，详见 `BENZHI_README.md` 与 `internal/httpapi`。

## 持久化

`modernc.org/sqlite`（纯 Go，CGO 无关）。表：schema_versions、region_replicas、
migration_plans、migration_steps、read_write_semantics、compat_windows、
conflict_paths、compat_snapshots、sample_records、audit_events。关闭后重开同一
数据库可恢复全部状态，相同指纹导入幂等。
