# Launch Options 缓存与 Shape/Image 绑定实施文档

更新日期：2026-06-16

本文定义创建实例页与创建模板页的选项加载改造方案。目标是把现在“每次打开页面、每次切换 Shape 都实时请求 OCI”的模式，改成“后端维护 OCI 选项目录，前端优先读本地缓存，后台只探查更新”的模式，尤其解决 Shape 与 Image 联动太慢、手感差的问题。

## 1. 背景问题

当前流程中，`/create` 与 `/templates/new` 都调用：

```http
GET /api/launch-options?profileId=...&region=...&compartmentId=...&availabilityDomain=...&vcnId=...&shape=...
```

OCI 模式下后端会实时调用 Identity、Compute、VCN、Block Volume 等 API。切换 Shape 时，前端再次请求 `/api/launch-options`，后端再用 Shape 去过滤兼容 Image。

这个模式正确但体验差：

- 打开创建页慢，因为要等多个 OCI API。
- 切换 Shape 慢，因为 Image 列表实时刷新。
- 模板页也被迫等待实时选项，但模板本身只是预输入，不应该让用户一直等。
- 当前为了避免 Shape 丢失而做的前端“保留选择”只是兜底，不是长期方案。

## 2. 核心目标

1. Shape 通过真实 OCI API 获取后写入本地选项目录。
2. Image 必须按 Shape 绑定缓存，不能只缓存一个全局 Image 列表。
3. 下次打开创建实例或创建模板页面时，优先读取本地 Shape/Image 目录。
4. 页面切换 Shape 时不再实时请求 OCI，而是从本地 `shape -> images` 绑定表中立即切换 Image 选项。
5. 后台只探查 Shape 是否更新；如果 Shape 目录未变化，继续使用上次完整缓存。
6. 如果 Shape 有新增、删除或规格范围变化，只增量刷新受影响 Shape 的 Image 绑定。
7. 用户点击“刷新选项”时触发后端刷新任务，而不是让前端同步等待 OCI 全量完成。
8. 缓存结果必须带来源、时间、状态和错误，不允许把旧数据伪装成实时验证结果。

## 3. 非目标

- 不在模板保存时调用 OCI 创建资源。
- 不把缓存结果当成容量保证。Shape 存在和 Image 兼容不代表一定有库存。
- 不缓存敏感密钥。目录表只保存 OCI 资源元数据和 request id。
- 不取消最终创建任务的真实 OCI 校验。提交创建实例后仍由 executor 做权限、容量、网络、配额和兼容检查。

## 4. 数据边界

Launch Options 拆成三类：

| 类型 | 缓存策略 | 说明 |
| --- | --- | --- |
| Profile / Region | 长缓存 | 变化少，可随 Profile 更新。 |
| Shape / Image | 目录缓存 + 后台刷新 | 本文重点。Image 必须绑定 Shape。 |
| VCN / Subnet / Reserved IP / Boot Volume 用量 | 短缓存或实时 | 网络和 IP 更容易被用户改动，可先保留现有实时或短 TTL。 |

第一阶段只改 Shape/Image，避免一次性重构所有 Launch Options。

## 5. 缓存键设计

Shape/Image 目录必须按 OCI 上下文隔离：

```text
profileId
region
compartmentId
availabilityDomain
```

建议缓存键：

```text
catalogKey = sha256(profileId + region + compartmentId + availabilityDomain)
```

说明：

- `profileId`：不同租户/用户权限不同。
- `region`：Shape 和 Image 都是区域相关。
- `compartmentId`：影响自定义镜像和可见资源。
- `availabilityDomain`：Shape 可用性可能随 AD 变化。

`vcnId` 不进入 Shape/Image 缓存键。网络选项单独处理。

## 6. 数据模型

### 6.1 Shape 目录表

PostgreSQL 表建议：

```sql
CREATE TABLE oci_shape_catalogs (
  id TEXT PRIMARY KEY,
  profile_id TEXT NOT NULL,
  region TEXT NOT NULL,
  compartment_id TEXT NOT NULL,
  availability_domain TEXT NOT NULL DEFAULT '',
  shape_fingerprint TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'EMPTY',
  last_checked_at TIMESTAMPTZ,
  last_changed_at TIMESTAMPTZ,
  last_error_code TEXT,
  last_error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`shape_fingerprint` 由排序后的 Shape 关键字段生成：

```text
shapeName
processorDescription / arch
minOcpus
maxOcpus
minMemoryGb
maxMemoryGb
gpuDescription
networkingBandwidth
localDiskDescription
```

只要 fingerprint 不变，就认为 Shape 目录未更新。

### 6.2 Shape 明细表

```sql
CREATE TABLE oci_shape_options (
  catalog_id TEXT NOT NULL REFERENCES oci_shape_catalogs(id) ON DELETE CASCADE,
  shape_name TEXT NOT NULL,
  arch TEXT NOT NULL DEFAULT 'unknown',
  min_ocpus NUMERIC NOT NULL DEFAULT 0,
  max_ocpus NUMERIC NOT NULL DEFAULT 0,
  min_memory_gb NUMERIC NOT NULL DEFAULT 0,
  max_memory_gb NUMERIC NOT NULL DEFAULT 0,
  is_flexible BOOLEAN NOT NULL DEFAULT false,
  is_always_free BOOLEAN NOT NULL DEFAULT false,
  raw JSONB,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (catalog_id, shape_name)
);
```

### 6.3 Shape/Image 绑定表

```sql
CREATE TABLE oci_shape_image_options (
  catalog_id TEXT NOT NULL REFERENCES oci_shape_catalogs(id) ON DELETE CASCADE,
  shape_name TEXT NOT NULL,
  image_id TEXT NOT NULL,
  image_name TEXT NOT NULL,
  operating_system TEXT,
  operating_system_version TEXT,
  lifecycle_state TEXT,
  time_created TIMESTAMPTZ,
  image_fingerprint TEXT NOT NULL DEFAULT '',
  raw JSONB,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (catalog_id, shape_name, image_id)
);
```

要求：

- 同一个 Image 可以绑定多个 Shape。
- 前端选择 Shape 后，只展示该 Shape 对应的 Image。
- 没有绑定 Image 的 Shape 不应隐藏，但要显示“该 Shape 暂无已缓存兼容镜像，可刷新补齐”。

### 6.4 File Store 兼容

无 PostgreSQL 时，在现有 file profile store 中增加：

```json
{
  "launchCatalogs": {
    "<catalogKey>": {
      "profileId": "",
      "region": "",
      "compartmentId": "",
      "availabilityDomain": "",
      "shapeFingerprint": "",
      "status": "READY",
      "shapes": [],
      "shapeImages": {
        "VM.Standard.E3.Flex": []
      }
    }
  }
}
```

## 7. 后端 API 设计

### 7.1 读取缓存目录

```http
GET /api/launch-catalog?profileId=...&region=...&compartmentId=...&availabilityDomain=...
```

返回：

```json
{
  "profileId": "profile-default",
  "region": "ap-chuncheon-1",
  "compartmentId": "ocid1...",
  "availabilityDomain": "xxx",
  "status": "READY",
  "cacheState": "HIT",
  "shapeFingerprint": "sha256...",
  "lastCheckedAt": "2026-06-16T00:00:00Z",
  "lastChangedAt": "2026-06-16T00:00:00Z",
  "shapes": [],
  "shapeImages": {
    "VM.Standard.E3.Flex": []
  },
  "missingImageShapes": [],
  "errorCode": "",
  "errorMessage": ""
}
```

`cacheState`：

| 状态 | 含义 |
| --- | --- |
| `MISS` | 从未同步过，需要初始化。 |
| `HIT` | 有缓存，可直接使用。 |
| `STALE` | 有缓存但超过建议刷新时间，仍可使用并提示后台刷新。 |
| `REFRESHING` | 后台正在刷新，前端继续使用旧缓存。 |
| `FAILED` | 刷新失败；如果有旧缓存则继续展示旧缓存并显示错误。 |

### 7.2 探查 Shape 是否更新

```http
POST /api/launch-catalog/refresh-shapes
```

请求：

```json
{
  "profileId": "profile-default",
  "region": "ap-chuncheon-1",
  "compartmentId": "ocid1...",
  "availabilityDomain": "xxx",
  "mode": "check"
}
```

行为：

1. 调用 Compute `ListShapes`。
2. 计算新的 `shapeFingerprint`。
3. 如果 fingerprint 未变化：
   - 更新 `lastCheckedAt`。
   - 不刷新 Image。
   - 返回 `changed=false`。
4. 如果 fingerprint 变化：
   - 写入新的 Shape 目录。
   - 找出新增、删除、规格变化的 Shape。
   - 为新增/变化 Shape 创建 Image 补齐任务。
   - 返回 `changed=true` 和受影响 Shape 列表。

### 7.3 初始化或补齐 Shape/Image 绑定

```http
POST /api/launch-catalog/refresh-images
```

请求：

```json
{
  "profileId": "profile-default",
  "region": "ap-chuncheon-1",
  "compartmentId": "ocid1...",
  "availabilityDomain": "xxx",
  "shapes": ["VM.Standard.E3.Flex"],
  "mode": "missing-only"
}
```

`mode`：

| 模式 | 行为 |
| --- | --- |
| `missing-only` | 只为没有 Image 绑定的 Shape 拉取 Image。 |
| `changed-only` | 只刷新本次 Shape fingerprint 变化涉及的 Shape。 |
| `force` | 全量刷新所有 Shape 的 Image 绑定。 |

每个 Shape 调用 Compute `ListImages` 时必须传入 `shape` 参数，确保返回兼容镜像。

### 7.4 兼容现有 `/api/launch-options`

现有 API 不立即删除，改为支持缓存模式：

```http
GET /api/launch-options?profileId=...&region=...&compartmentId=...&availabilityDomain=...&shape=...&cacheMode=prefer
```

`cacheMode`：

| 模式 | 行为 |
| --- | --- |
| `prefer` | 优先返回缓存；缺失时触发后台初始化并返回可用的旧数据或空状态。 |
| `refresh-shapes` | 同步探查 Shape fingerprint，但不全量刷新 Image。 |
| `force` | 管理员手动触发全量刷新任务，接口返回 Job，不同步等待。 |

第一阶段前端可以继续调用 `/api/launch-options`，后端内部改为读取目录缓存，降低前端改造量。

## 8. 后台任务设计

新增 Job 类型：

```text
LAUNCH_CATALOG_REFRESH_SHAPES
LAUNCH_CATALOG_REFRESH_IMAGES
LAUNCH_CATALOG_FULL_WARMUP
```

任务状态沿用现有：

```text
PENDING -> RUNNING -> VERIFYING -> SUCCESS/FAILED
```

Image 预热要限制并发：

- 每个 catalog 同时最多 1 个 refresh job。
- ListImages 并发建议 2。
- 遇到 429、限流、网络错误时指数退避。
- 每次全量预热最多刷新 50 个 Shape；超过则分页/分批继续。
- 每个 Shape 的失败要单独记录，不应导致整个目录不可用。

## 9. 前端交互改造

### 9.1 页面打开

创建实例页和创建模板页加载流程：

1. 根据全局 Profile / Region / Compartment / AD 请求缓存目录。
2. 如果 `cacheState=HIT/STALE`，立即渲染 Shape 和默认 Shape 的 Image。
3. 如果 `cacheState=STALE`，显示轻提示：“选项目录较旧，正在后台探查 Shape 更新。”
4. 如果 `cacheState=MISS`，显示初始化状态和“初始化选项目录”按钮。
5. 初始化完成前，不能再展示“保留 Shape”这种临时伪选项作为常规体验。

### 9.2 切换 Shape

切换 Shape 时：

1. 不请求 OCI。
2. 从 `shapeImages[shapeName]` 读取 Image。
3. 如果当前 `imageId` 不在该 Shape 的绑定列表中，自动切换到该 Shape 的第一个兼容镜像。
4. 如果该 Shape 没有缓存 Image：
   - Image 下拉显示“该 Shape 暂无已缓存兼容镜像”。
   - 显示“补齐该 Shape 镜像”按钮。
   - 点击后创建 `LAUNCH_CATALOG_REFRESH_IMAGES` 任务。

### 9.3 刷新选项按钮

原来的“刷新选项”改成两级：

- “探查 Shape 更新”：轻量，只调用 ListShapes，默认按钮。
- “强制刷新全部镜像”：高级操作，需要二次确认，因为会对每个 Shape 调用 ListImages。

模板页文案：

```text
模板只保存预输入。Shape/Image 选项来自本地 OCI 目录；目录由后台任务通过真实 OCI API 同步。
```

### 9.4 状态展示

在“镜像与规格”区右上角显示目录状态：

```text
选项目录：已缓存 / 12 分钟前探查 / Shape 未变化
```

异常时：

```text
选项目录：刷新失败，正在使用 2026-06-16 20:30 的缓存
错误：NotAuthorizedOrNotFound...
```

## 10. Image 绑定规则

必须满足：

- 每个 Shape 的 Image 列表只来自带 `shape` 参数的 `ListImages`。
- 不能把未绑定 Shape 的通用 Image 列表塞给所有 Shape。
- 不能在 Shape 切换失败时沿用上一个 Shape 的 Image。
- 如果绑定表为空，必须明确提示“未缓存/刷新失败”，不能静默展示旧 Image。
- 保存模板时保存的是 `shape + imageId + imageName`，同时记录选项目录的 `shapeFingerprint`，便于后续提示模板是否可能过旧。

模板扩展字段建议：

```json
{
  "catalog": {
    "shapeFingerprint": "sha256...",
    "catalogCheckedAt": "2026-06-16T00:00:00Z"
  }
}
```

## 11. 缓存刷新策略

建议默认策略：

| 事件 | 行为 |
| --- | --- |
| 首次进入创建页且无缓存 | 创建全量 warmup job。 |
| 再次进入创建页且有缓存 | 立即展示缓存，后台探查 Shape。 |
| Shape fingerprint 未变化 | 不刷新 Image。 |
| Shape fingerprint 变化 | 只刷新新增/变化 Shape 的 Image。 |
| 用户手动强制刷新 | 分批刷新所有 Shape/Image。 |
| 缓存超过 24 小时 | 标记 `STALE`，但仍可使用。 |
| 缓存超过 7 天 | 创建实例提交前提示目录过旧，建议刷新。 |

Image 本身可能独立更新。为了兼顾性能，第一阶段不在页面打开时实时检查 Image；后续可增加每日定时任务刷新常用 Shape 的 Image。

## 12. 与预算和免费标记的关系

Shape 目录写入后，可为预算估算提供更稳定的 Shape 元数据：

- `is_flexible`
- `min_ocpus`
- `max_ocpus`
- `min_memory_gb`
- `max_memory_gb`
- `is_always_free`
- `arch`

前端不再依赖临时推断 Shape 能否调节 OCPU/内存，而是使用目录字段冻结或开放输入。

## 13. 安全与审计

- 缓存目录不保存 PEM、SMTP 密码、面板密码、root 密码。
- 每次刷新记录 actor、profileId、region、compartmentId、AD、OCI request id、错误码。
- 手动“强制刷新全部镜像”写入 Audit。
- 后台任务失败进入通知中心，不阻塞页面使用旧缓存。

## 14. 迁移计划

### 阶段 1：后端目录与缓存读取

- 新增 Shape/Image catalog 数据模型。
- 新增 PostgreSQL migration。
- File store 增加 launchCatalogs。
- 新增 catalog repository。
- 新增 `/api/launch-catalog` 读取接口。
- `/api/launch-options` 在 `cacheMode=prefer` 下优先读缓存。

### 阶段 2：后台刷新任务

- 实现 `LAUNCH_CATALOG_REFRESH_SHAPES`。
- 实现 `LAUNCH_CATALOG_REFRESH_IMAGES`。
- 实现全量 warmup 分批任务。
- 加入并发、退避、错误记录。

### 阶段 3：前端切换到缓存目录

- 创建实例页打开时读取缓存目录。
- 创建模板页打开时读取缓存目录。
- Shape 切换改为本地切换 Image。
- 移除常规路径上的 `selection-preserved` 兜底展示。
- 刷新按钮改成“探查 Shape 更新 / 强制刷新全部镜像”。

### 阶段 4：验收与真实 OCI 专项验证

- 使用真实 Profile 初始化目录。
- 验证 E3.Flex、A1.Flex、E2.1.Micro、至少一个固定 Shape。
- 验证每个 Shape 的 Image 列表不同且兼容。
- 验证 Shape fingerprint 未变化时不刷新 Image。
- 验证新增 Shape 或模拟 fingerprint 变化时只刷新受影响 Shape。
- 验证无 OCI 密钥时模板 CRUD 仍可用，但选项目录显示未初始化。

## 15. 验收标准

- 第二次打开创建实例页时，Shape/Image 区域不等待 OCI API 即可展示。
- 切换 Shape 时不发起 `/api/launch-options?shape=...` 实时请求。
- 切换 Shape 后 Image 下拉立即变为该 Shape 绑定的缓存 Image。
- Shape 目录未变化时，后台只更新 `lastCheckedAt`，不刷新所有 Image。
- 强制刷新全部镜像会创建后台 Job，不让页面同步卡住。
- 缓存刷新失败时，页面继续展示旧缓存并明确显示错误状态。
- 模板页和创建实例页使用同一套 Shape/Image 目录，不再各自实时抓取。
- 后端测试覆盖：缓存命中、缓存缺失、Shape fingerprint 未变化、Shape fingerprint 变化、单 Shape Image 补齐、刷新失败保留旧缓存。
- 前端构建通过，创建实例页和模板页在无缓存、有缓存、缓存失败三种状态下都有明确 UI。

