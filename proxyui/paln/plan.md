# 连接层级显示与活跃连接过滤实施方案

## 需求概述

1. **Connections 页面**：实现父子连接的层级折叠显示，显示所有连接（不过滤 status）
2. **Overview 页面**：仅显示活跃（status=Active）的子节点连接，不显示父节点

---

## 一、数据结构设计

### 1.1 Store 层（保持不变）

`src/stores/websocket.js` 中的 `connections` 继续存储后端推送的扁平化数据：

```javascript
connections: [
  { id: 1, parentId: 0, status: "Active", host: "...", up: 1024, down: 2048, ... },
  { id: 2, parentId: 1, status: "Closed", host: "...", up: 512, down: 1024, ... }
]
```

### 1.2 Connections.vue ViewModel

**扁平转树形结构**：

```javascript
{
  rootIds: [1, 3, 5],        // parentId === 0 的节点 id 列表
  childrenMap: {
    1: [2, 3],                // id: 1 的子节点 id 列表
    3: [4],                   // id: 3 的子节点 id 列表
    // ...
  }
}
```

**展开状态管理**：

```javascript
{
  expandedIds: Set([1, 3])   // 存储已展开的父节点 id（独立于数据源）
}
```

### 1.3 Overview.vue ViewModel

```javascript
{
  activeConnections: [       // 过滤后的活跃子节点连接
    { id: 2, parentId: 1, status: "Active", ... }
  ]
}
```

---

## 二、Connections.vue 修改方案

### 2.1 修改文件

`src/views/Connections.vue`

### 2.2 数据层修改

| 位置          | 修改内容                                                     |
| ------------- | ------------------------------------------------------------ |
| 第 119 行     | `const connections = ref([])` 改为 `const flatConnections = ref([])` |
| 第 176-178 行 | 删除 `updateConnections` 中的 `filter(conn => conn.parentId === 0)`，改为存储全量数据 |
| 新增 computed | `rootIds` - 根节点 id 数组                                   |
| 新增 computed | `childrenMap` - 子节点映射表                                 |
| 新增 computed | `flattenedConnections` - 用于渲染的扁平化列表（展开的子节点插入到父节点后） |
| 新增 state    | `expandedIds = ref(new Set())` - 展开状态管理                |

**核心数据转换逻辑**：

```javascript
// 构建树形结构
const rootIds = computed(() => {
  return flatConnections.value
    .filter(c => c.parentId === 0)
    .map(c => c.id)
})

const childrenMap = computed(() => {
  const map = {}
  flatConnections.value.forEach(conn => {
    if (conn.parentId !== 0) {
      if (!map[conn.parentId]) map[conn.parentId] = []
      map[conn.parentId].push(conn)
    }
  })
  return map
})

// 展开搜索匹配的父节点
const expandSearchResults = (query) => {
  if (!query) return
  const lowerQuery = query.toLowerCase()
  flatConnections.value.forEach(conn => {
    if ((conn.host?.toLowerCase().includes(lowerQuery) ||
         conn.url?.toLowerCase().includes(lowerQuery)) &&
        conn.parentId !== 0) {
      expandedIds.value.add(conn.parentId)
    }
  })
}
```

### 2.3 模板层修改

**表头**：在 ID 列添加展开/收起图标占位
**表格主体**：使用递归或两层 v-for 渲染

| 位置         | 修改内容                                                     |
| ------------ | ------------------------------------------------------------ |
| 第 88 行     | `v-for="conn in sortedConnections"` 改为 `v-for="item in flattenedConnections"` |
| 第 89-104 行 | 修改渲染逻辑，区分父行和子行                                 |

**渲染逻辑**：

```html
<!-- 父行 -->
<tr :class="{ 'parent-row': item.parentId === 0, 'child-row': item.parentId !== 0 }">
  <td>
    <span v-if="item.parentId === 0" @click="toggleExpand(item.id)" class="expand-icon">
      {{ expandedIds.has(item.id) ? '▼' : '▶' }}
    </span>
    {{ item.id }}
  </td>
  <!-- 其他列... -->
</tr>
```

### 2.4 样式层修改

**新增样式**：

```css
.expand-icon {
  cursor: pointer;
  margin-right: 8px;
  color: #cba376;
}

.child-row {
  background: #222;
}

.child-row td:first-child {
  padding-left: 40px;  /* 缩进 */
}
```

---

## 三、Overview.vue 修改方案

### 3.1 修改文件

`src/views/Overview.vue`

### 3.2 数据层修改

| 位置          | 修改内容                              |
| ------------- | ------------------------------------- |
| 第 170-172 行 | 修改 `updateConnections` 中的过滤逻辑 |

**新的过滤逻辑**：

```javascript
// 过滤活跃状态的子节点连接
const activeConnections = computed(() => {
  return flatConnections.value.filter(conn =>
    conn.parentId !== 0 && conn.status === 'Active'
  )
})
```

### 3.3 模板层修改

| 位置      | 修改内容                                                     |
| --------- | ------------------------------------------------------------ |
| 第 46 行  | `v-for="conn in connections"` 改为 `v-for="conn in activeConnections"` |
| 第 171 行 | 删除 `connections.value = data`，改为 `flatConnections.value = data` |

---

## 四、关键文件清单

| 文件路径                    | 修改类型 | 修改说明                              |
| --------------------------- | -------- | ------------------------------------- |
| `src/views/Connections.vue` | 重构     | 实现层级显示、展开/收起、搜索自动展开 |
| `src/views/Overview.vue`    | 简单修改 | 添加活跃状态过滤                      |

---

## 五、验证方案

### 5.1 功能验证

1. **Connections 页面**：
   - 父节点显示展开/收起图标
   - 点击图标切换子节点显示
   - 搜索子节点时自动展开父节点
   - 子行有缩进和背景色区分
   - 排序仅影响父节点顺序

2. **Overview 页面**：
   - 只显示 status 为 Active 的连接
   - 连接状态变为 Closed 时自动消失

### 5.2 测试步骤

1. 启动开发服务器：`npm run dev`
2. 登录后切换到 Connections 页面
3. 验证父子节点显示和折叠功能
4. 测试搜索功能，确认匹配子节点时父节点自动展开
5. 切换到 Overview 页面
6. 验证仅显示活跃连接
7. 等待连接状态变化，验证自动过滤

---

## 六、注意事项

1. **数据不污染**：所有 UI 状态（expandedIds）独立于数据源存储
2. **性能考虑**：computed 层做转换，避免在模板中重复计算
3. **后端兼容**：父节点流量已由后端聚合，前端无需额外处理