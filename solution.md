# 方案说明

## 系统总体设计
- 应用入口位于 `cmd/server/main.go`，负责加载配置、初始化存储与审计组件，并启动 Gin HTTP 服务。
- 核心业务封装在 `internal/app/service.go`，提供统一接口供 API、定时任务与合并逻辑复用。
- `internal/api/handlers.go` 注册 `/hotels` 查询接口，只从本地存储读取已合并的数据并返回。
- `internal/domain/` 定义 `Hotel` 及其子结构；`internal/suppliers/` 为不同供应商的适配层，统一输出标准化酒店对象；`internal/store/` 提供可替换的 KV 实现与审计记录。

## 数据抓取与留痕
- 定义 `Supplier` 接口：`Name() string`、`Fetch(ctx) ([]RawHotel, error)`；各供应商文件负责把特殊字段映射为统一结构（如 `Latitude`→`Location.Lat` 等）。
- 抓取结果中保留原始 JSON，写入审计日志 `AuditStore`，便于追溯。
- `AuditRecord` 结构含 `Supplier`、`RawPayload`、`Merged`（仅在合并后填写）与 `Timestamp`。

## KV 存储模型
- `Store` 以 `destinationID -> hotelID -> Record` 组织，内部使用 `sync.RWMutex` 保证并发安全。
- `Record` 字段：
  - `Raw map[string]json.RawMessage` 保存供应商原始数据；
  - `Canonical *domain.Hotel` 存放最新合并结果；
  - `NeedsMerge bool` 标记是否待合并；
  - `LastFetched map[string]time.Time` 与 `LastMerged time.Time` 记录时间戳。

## 定时任务流程
1. **供应商拉取任务**（例如每 5 分钟）：
   - 并发调用各 `Supplier.Fetch`；
   - 按 `destination_id` 聚合后与本地 `Raw` 比较（可用字节比对或哈希）；
   - 发现新增或更新则覆盖原始数据、刷新 `LastFetched`、标记 `NeedsMerge=true`，并写入审计日志；无变化则跳过。
2. **待合并扫描任务**（例如每 1 分钟）：
   - 遍历 `NeedsMerge=true` 的记录，使用供应商标准化输出作为输入；
   - 执行合并逻辑，生成统一的 `Hotel`；
   - 成功后写回 `Canonical`、`LastMerged`、`NeedsMerge=false`，并追加一条 `merged` 审计记录。

## 合并与去重策略
- **字段补全**：按供应商优先级（注册顺序）填充 `Name`、`Location` 等单值字段，优先取非空且内容更长的值。
- **Amenities**：字符串去空格并转小写去重，保留首次出现的原始格式。
- **Images**：按类别分组，对 `link` 去重，描述保留较长且非空版本，保持供应商顺序。
- **BookingConditions**：标准化空白后去重，输出原文。
- 合并异常时保留 `NeedsMerge=true` 并记录错误，待人工或下一轮修复。

## 查询接口
- `/hotels` 接受 `destination`（支持逗号列表）与 `hotels`（逗号列表），可同时使用取交集；缺少参数返回 400。
- 仅返回 `NeedsMerge=false` 且 `Canonical` 存在的记录，若数据尚未合并可返回空数组或特定提示。
- 通过内存 Store 的索引查询，避免实时访问供应商。

## 测试计划
- Supplier 适配层：使用 `httptest.Server` 模拟返回，验证字段映射与错误处理。
- 合并逻辑：针对不同冲突场景编写单元测试，覆盖字段补全与去重行为。
- Store 与定时任务：构造伪数据，验证更新判定、状态切换、时间戳与审计记录。
- API 层：利用 Gin 测试工具检查参数解析与响应结构。

## 调试
- 增加一个webUI, 可以查看store的内容, 方便调试用, 有列表总览和针对id进行单独查询展示的实用功能, 极简,够用就行, 非必要不要引入过多依赖.


## 可扩展方向

- KV 与审计组件可替换为 Redis、数据库等持久化实现。
- 定时任务周期、并发策略可通过配置调整；必要时可加队列提升吞吐。
- 查询层后续可缓存热门组合或引入分页，以应对数据量增长。
