# Overview.vue 和 Connections.vue 性能优化实施计划

## 上下文

**问题描述**: Overview.vue（活动连接部分）和 Connections.vue（详细连接）在高并发大流量场景下存在严重性能问题，导致浏览器卡顿。

**问题根因**:

1. 使用原生 `<table>` + `v-for` 全量渲染所有连接数据
2. 无最大数量限制，万级连接会全部挂载到 DOM
3. WebSocket Store 的 `connections` 数据没有设置上限（对比 logs 有 MAX_LOGS=500，mitm 有 MAX_MITM_EXCHANGES=2000）

**参考实现**: Logs.vue 和 MITM.vue 已完美使用 `@tanstack/vue-virtual` 实现虚拟滚动，且依赖已安装（package.json:17）。

---

## 修订后的实施计划

### 第一步：WebSocket Store 层面添加数据上限

**文件**: `src/stores/websocket.js`

**改动**:

1. 添加 `MAX_CONNECTIONS = 1000` 常量（与 MAX_LOGS、MAX_MITM_EXCHANGES 保持一致）
2. 在 `handleConnections` 方法中添加数量限制逻辑（参考 `handleLogBatch` 的实现）

**原因**: 即使前端使用虚拟滚动，数据层也应该有上限作为兜底保护。

---

### 第二步：Overview.vue 活动连接改造

**文件**: `src/views/Overview.vue`

**改动方案**:

1. 移除原生 `<table>` 结构（第38-64行）
2. 引入 `@tanstack/vue-virtual` 的 `useVirtualizer`
3. 使用 CSS Grid 模拟表格布局（参考 MITM.vue:544 的 grid-template-columns）
4. 限制显示数量为 50 条，提供"查看更多"按钮跳转到 Connections.vue

**关键实现细节**:

```js
// 虚拟滚动器配置
const virtualizer = useVirtualizer(
  computed(() => ({
    count: displayConnections.value.length,  // 最多 50 条
    getScrollElement: () => scrollerRef.value,
    estimateSize: () => 42,  // 固定行高
    overscan: 10,
    getItemKey: (index) => displayConnections.value[index].id,
  }))
)

// 限制显示数量
const displayConnections = computed(() =>
  connections.value.slice(0, 50)
)
```

**CSS Grid 定义**:

```css
.connections-grid-row {
  display: grid;
  grid-template-columns: 80px 80px 180px 1fr 100px 100px 100px;
  align-items: center;
  color: #cba376;
}
```

---

### 第三步：Connections.vue 详细连接改造

**文件**: `src/views/Connections.vue`

**改动方案**:

1. 移除原生 `<table>` 结构（第39-125行）
2. 引入 `@tanstack/vue-virtual`
3. 使用 CSS Grid 替代表格布局
4. 保持父子展开功能，需要处理动态高度

**关键实现细节**:

#### 3.1 虚拟滚动器配置（动态高度）

```js
const virtualizer = useVirtualizer(
  computed(() => ({
    count: flatConnections.value.length,
    getScrollElement: () => scrollerRef.value,
    estimateSize: () => 42,  // 基础行高
    overscan: 10,
    getItemKey: (index) => flatConnections.value[index].id,
  }))
)
```

#### 3.2 动态高度测量（参考 MITM.vue:82）

```html
<div
  v-for="virtualRow in virtualRows"
  :key="virtualRow.key"
  :ref="(el) => { if (el) virtualizer.measureElement(el) }"
  :data-index="virtualRow.index"
  ...
>
```

#### 3.3 扁平化数据处理

保持现有的 `sortedConnections` computed 逻辑，但其结果直接作为虚拟列表的数据源。

#### 3.4 展开状态维护

保持现有的 `expandedIds` Set 结构，每次展开/收起时触发虚拟列表重新计算高度。

**CSS Grid 定义**:

```css
.connections-grid-row {
  display: grid;
  grid-template-columns: 80px 80px 180px 1fr 100px 100px 100px;
  align-items: center;
  color: #cba376;
}
```

---

## 关键文件清单

| 文件                        | 改动类型 | 说明                            |
| --------------------------- | -------- | ------------------------------- |
| `src/stores/websocket.js`   | 修改     | 添加 MAX_CONNECTIONS 限制       |
| `src/views/Overview.vue`    | 重构     | table → 虚拟滚动 + 50条限制     |
| `src/views/Connections.vue` | 重构     | table → 虚拟滚动 + 保持展开功能 |

---

## 参考文件（复用实现）

| 参考文件                     | 可复用的实现                 |
| ---------------------------- | ---------------------------- |
| `src/views/Logs.vue:62-98`   | useVirtualizer 基础配置      |
| `src/views/MITM.vue:233-311` | 虚拟滚动 + CSS Grid 表格布局 |
| `src/views/MITM.vue:542-557` | CSS Grid 列定义              |
| `src/views/MITM.vue:82`      | 动态高度 measureElement 用法 |
| `src/views/MITM.vue:369-378` | 展开/收起状态维护            |

---

## 验证方法

1. **开发服务器测试**: `npm run dev`
2. **模拟高并发**: 使用后端推送 1000+ 连接数据
3. **性能指标**:
   - 打开 Chrome DevTools → Performance 录制
   - 滚动列表，FPS 应保持 60
   - Memory 面板，DOM 节点数量应稳定在 ~100（而非全量）

4. **功能验证**:
   - Overview.vue 显示最多 50 条，点击"查看更多"跳转
   - Connections.vue 父子展开/收起正常工作
   - 搜索、排序功能保持正常

---

## 与原计划的差异说明

| 原计划建议                        | 修订方案                       | 原因                                 |
| --------------------------------- | ------------------------------ | ------------------------------------ |
| Overview 使用虚拟滚动或 50 条限制 | 虚拟滚动 + 50 条限制（双保险） | 50 条原生 table 仍有性能开销         |
| shallowRef 替代 ref               | 不需要                         | 虚拟滚动已解决渲染性能问题           |
| 未提及 Store 层优化               | 添加 MAX_CONNECTIONS           | 作为兜底保护，与其他数据类型保持一致 |
| 未详细说明动态高度处理            | 增加 measureElement 说明       | Connections 有展开功能，需要动态高度 |