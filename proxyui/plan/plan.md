# 单一可执行文件跨平台部署方案 (Go + Vue)

本方案旨在解决将 `proxy_man` (Go 后端) 和 `proxyui` (Vue 前端) 打包为**单个可执行文件**的需求，使其无论是在 Windows 还是 Linux 下都能做到“一键运行”，无需提前部署 Nginx 或单独启动前端服务。

## 核心思路

自 Go 1.16 版本起，原生提供了 `//go:embed` 特性。它能够在 Go 语言编译期，将前端构建生成的静态 HTML、JS、CSS、图片等文件直接打包内嵌到编译后的二进制文件中。之后通过 Go 自身的 HTTP 服务直接在内存中对外提供前端文件访问。最后通过 Go 的交叉编译特性，分别打包出 `.exe` (Win) 和无后缀可执行程序 (Linux)。

---

## 实施步骤详情

### 1. 前端（proxyui）打包部署

在 Vue 项目 (`proxyui`) 中执行标准的生产环境构建：

```bash
cd e:\D\zuoyewenjian\MyProject\proxyui
npm run build
```

这会在 `proxyui` 目录下生成一个 `dist` 文件夹，里面包含了所有前端静态资源。

### 2. 将静态文件转移或映射至后端

我们需要将 `dist` 文件夹放置在 Go 项目能够读取并 embed 的目录下。
建议在 `proxy_man` 根目录下创建一个 `public` 文件夹，然后将上一步生成的 `dist` 文件夹复制进去（路径为 `proxy_man/public/dist/`）。
*提示：这一步后续都可以通过自动化脚本一键完成。*

### 3. 修改后端代码适配前端嵌入 (重点)

#### 3.1 引入 `//go:embed`

在 [proxy_man/proxysocket/proxy.go](file:///e:/D/zuoyewenjian/MyProject/proxy_man/proxysocket/proxy.go) 中引入 `embed` 并在包级别声明静态资源：

```go
package proxysocket

import (
    "embed"
    "io/fs"
    "net/http"
    "path"
    "strings"
)

//go:embed public/dist/*
var embeddedFiles embed.FS
```

#### 3.2 API、WebSocket 与静态文件的路由分发 (Multiplexing)

Go 语言的 `http.ServeMux` 具有**最长前缀匹配优先**的特性。由于控制台和前后端通讯都在 `8000` 端口下，我们需要处理好流量剥离的问题。

处理流向设计如下：

1. **WebSocket 连接 (`/start`)**：前端发起 WebSocket 连接时精确命中，执行原有的 [handleWebSocket](file:///e:/D/zuoyewenjian/MyProject/proxy_man/proxysocket/proxy.go#99-158) 逻辑。
2. **后端 API 请求 (`/api/...`)**：前端发起 `fetch` 或 `axios` 等 HTTP 请求（例如获取配置 `/api/config`、下载请求 `/api/storage/download`），将会精确或按前缀命中现有接口层。
3. **前端页面和静态资源 (`/`)**：当请求**不匹配以上任何一条规则时**（比如请求加载 `/assets/main.js`，或者是你刷新了 `/dashboard/connections`），才会被路由至最低级匹配路径 `/` 中，进入静态文件服务兜底逻辑：
   - 检查 `dist` 中是否存在这个物理文件（如下载 CSS/JS）。如果有则直接返回文件流。
   - 如果不存在（像刚才讨论的处理 Vue History 虚拟路由的情形），统一回退返回 [index.html](file:///e:/D/zuoyewenjian/MyProject/proxyui/index.html) 的内容，把路由控制权交还给 Vue Router。

只要注册路由的顺序不干扰，它们就能在同一个端口下和谐共处。我们在 [proxy.go](file:///e:/D/zuoyewenjian/MyProject/proxy_man/proxysocket/proxy.go) 中的 `http.NewServeMux()` 修改将如下所示：

```go
func (ws *WebsocketServer) StartControlServer(cm *mproxy.ConfigManager, router *mproxy.Router) bool {
    // ... 前置初始化
    mux := http.NewServeMux()
    
    // 1. 保留现有的 API 和 WS 高优先级路由
    mux.HandleFunc("/start", ws.loginHandler(ws.handleWebSocket))
    mux.HandleFunc("/api/storage/download", myminio.HandleDownload)
    mux.HandleFunc("/api/config", ws.handleConfig(cm, router))

    // 2. 提取内嵌静态文件系统的根目录
    distFS, err := fs.Sub(embeddedFiles, "public/dist")
    if err != nil {
        log.Fatal("无法加载前端静态资源")
    }
    fileServer := http.FileServer(http.FS(distFS))
    
    // 3. 注册根路径 `/` 的兜底拦截器（接管 Vue 路由及所有静态资源请求）
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // [拦截回退逻辑代码...]
        // -> 如果请求静态资源则 ServeFile
        // -> 如果没找到则强行输出 index.html
    })
    
    // ... 后续启动服务
}
```

### 4. 自动化构建与跨平台编译脚本

为了避免每次都要手动 `npm run build`、复制文件夹、针对不同平台编译，我们可以写一个跨平台的脚本。
更关键的是，我们要保证**目标服务器零依赖部署**，所以我们要采用关闭 CGO 绑定的纯静态编译方式。

#### 运行环境要求（零依赖）

- **开发机（打包环境）：** 需要完整的 Go 和 Node.js/npm 环境进行构建。
- **目标服务器（运行环境）：** **没有任何环境要求。** 不需要 Node.js，不需要 npm，不需要 Nginx，不需要预装 Go 运行环境，甚至不需要动态链接 C 语言标准库 (`libc`)。只要是一台普通装有 Windows 或 Linux 操作系统的裸机（或容器），把构建出来的单文件拖进去，就能直接运行。

#### 编译命令示例

**编译为 Windows 版本 (完全静态打包)**

```bash
cd proxy_man
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-w -s" -o proxy_man_win.exe main.go
```

**编译为 Linux 版本 (完全静态打包)**

```bash
cd proxy_man
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-w -s -extldflags '-static'" -o proxy_man_linux main.go
```

*(注：`-ldflags="-w -s"` 用于去除符号表和调试信息以减小可执行文件体积。对于 Linux `extldflags '-static'` 进一步确保完全静态链接没有任何 libc 依赖。)*

---

## 补充说明：端口角色分离

打包后无论是 Windows 还是 Linux 平台的单文件程序，在运行时会同时开启两个独立的端口，各自负责不同的流量，确保互不干扰：

1. **代理服务器端口（按 [config.json](file:///e:/D/zuoyewenjian/MyProject/proxy_man/config.json) 设定，如默认的 `8080`）：** 
   仅接受真实设备的 HTTP/HTTPS 代理请求，承担网络抓包、拦截及代理转发职责。没有任何前端页面的网络包会干扰此处。
2. **管理面板端口（目前的 `:8000` 端口）：**
   仅处理基于浏览器的面板访问请求（[index.html](file:///e:/D/zuoyewenjian/MyProject/proxyui/index.html)）、Vue 页面静态资源下发，以及面板控制通讯（WebSocket `/start` 和 `/api/config` 接口）。

即使物理上只剩下一个 `.exe` 可执行文件，它内部依然会分别启动两个不同的 `http.Server`，两者的网络流量各走各的门。

---

## 补充说明：开发模式与生产模式互不干扰（前后端分离开发依然可用）

打包方案**完全不会**影响你现有的“前后端分离”开发体验。引入 `//go:embed` 仅仅是利用 Go 在生产环境中额外托管了一份静态文件，它不会改变任何现有的 API 和 WebSocket 逻辑。

按照本方案修改后，在你的日常开发过程中：

1. **启动后端：** 你依然像以前一样运行 `go run main.go`。由于开启了跨域 (CORS) 支持并且 8000 端口完整保留了所有 API，后端就像一个独立的服务正常待命。
2. **启动前端热重载：** 你依然可以在 `proxyui` 目录下执行 `npm run dev`，启动类似于 `localhost:5173` 的开发服务器。
3. **联调工作：** 运行在 `5173` 端口的 Vue 开发服务器，依然可以向 `8000` 端口发送 API 抓取或 WebSocket 连接。我们保留了 `corsMiddleware` 允许跨域。

**结论：** 

- **开发时（Dev）**，你依旧进行着前后端完全分离的工作，享受前端极速热更新（HMR）。
- **发布时（Prod）**，你只需要执行打包脚本，生成一个单文件交给用户，用户无需启动前端服务也可以直接在 8000 端口上浏览并管理代理面板。

---

## User Review Required

> [!IMPORTANT]
> **我们需要对代码做以下更改以达成以上方案：**
>
> 1. 修改 [proxy_man/proxysocket/proxy.go](file:///e:/D/zuoyewenjian/MyProject/proxy_man/proxysocket/proxy.go)，注入 `embed.FS` 并重写 `http.NewServeMux()` 以支持 `history mode` 静态文件服务。
> 2. 为你提供一份自动化 `build.bat` 或 `build.ps1` 脚本，将 npm build 和 go build 集成到一起，实现真正的**一键跨平台打包**。
>
> **请确认：**
>
> - 是否允许我开始按照这个方案修改代码？
> - 端口方面，控制服务和现有前端 API 都在 `:8000` 端口下，我们将把前端网页也托管在 `http://localhost:8000` 下向用户提供访问，这样设计是否符合你的预期？