# ConnectHTTPMitm 死锁修复方案审查与计划

## 问题描述

### 死锁场景

当客户端（如 Curl）使用 `Expect: 100-continue` 头上传大文件时，会发生短暂死锁：

```
时间线:
T0: Client 发送 Header (带 Expect: 100-continue)
T1: Proxy 收到 Header，调用 req.Write(connRemoteSite)
T2: Target 收到 Header，秒回 "100 Continue"
T3: "100 Continue" 到达 Proxy 的 TCP 接收缓冲区
T4: Client 等待 1 秒后放弃，开始发送 100MB Body
T5: Proxy 的 req.Write 正在搬运 Body，无法读取响应
    → "100 Continue" 积压在缓冲区中
T6: req.Write 完成后才调用 http.ReadResponse
    → 才能转发积压已久的 "100 Continue"
```

### 根本原因

`req.Write()` 是同步阻塞操作：

- 它按顺序发送：Header → Body（流式读取）
- 只有发送完整个请求后才会 return
- 在此期间无法读取 Target 的响应

**受影响代码位置**：`mproxy/https.go:422-431`

## 方案审查

### plan.md 中的修复方案分析

**核心思路**：

1. 启动 Goroutine 专门负责 `req.Write`
2. 主线程立即开始读取响应
3. 循环处理 1xx 响应，立即转发给客户端

### 审查结果

#### ✅ 正确的部分

1. **并发模型设计正确**
   - 读写分离避免死锁
   - 使用 channel 传递错误

2. **1xx 响应处理逻辑正确**
   - `http.ReadResponse` 会返回 1xx 响应（不会自动跳过）
   - 循环读取可以正确处理 1xx 响应
   - 手动构造并发送 1xx 响应符合 RFC 7231

3. **与现有代码风格一致**
   - 项目中已有类似的双向转发模式（隧道模式、WebSocket）

#### ⚠️ 需要改进的部分

**问题 1：`req.Body` 生命周期管理**

方案中注释掉了 `req.Body.Close()`：

```go
// line 23: defer req.Body.Close() // 注意：这里可能需要斟酌
```

**建议**：在 `req.Write` goroutine 中关闭 Body：

```go
go func() {
    defer req.Body.Close()  // 在写完成后关闭
    err := req.Write(connRemoteSite)
    writeErrCh <- err
    close(writeErrCh)
}()
```

**问题 2：错误处理不完整**

方案中的 `select` 只是非阻塞检查，如果 `req.Write` 失败了但响应已经成功读取，这个错误会被忽略。

**建议**：在返回最终响应前，等待 `writeErrCh` 确保 Body 发送完成：

```go
// 在读取到最终响应后
if writeErr := <-writeErrCh; writeErr != nil {
    // 记录警告但不中断响应处理（Early Response 情况）
    ctxt.WarnP("Request write failed after response received: %v", writeErr)
}
```

**问题 3：连接复用问题**

`ConnectHTTPMitm` 使用长连接隧道（`for !reqReader.IsEOF()`），`connRemoteSite` 会在多个请求之间复用。

需要确保：

1. `req.Write` 完成后，连接仍然可用
2. 如果 `req.Write` 失败，需要关闭连接并重新建立

**问题 4：缺少流量统计**

当前代码在 `actions.go` 中通过包装 `req.Body` 和 `resp.Body` 实现流量统计。但如果在 `req.Write` goroutine 中读取 Body，流量统计的 `reqBodyReader` 可能无法正常工作。

**建议**：确保流量统计在 `req.Write` 之前正确设置。

## 修复计划

### 修改文件

**主要文件**：`mproxy/https.go`

**修改位置**：`ConnectHTTPMitm` case 中的请求发送和响应读取逻辑（约 422-436 行）

### 修改步骤

1. **修改请求发送逻辑**（约 422-426 行）
   - 创建错误 channel
   - 启动 goroutine 处理 `req.Write`
   - 在 goroutine 中管理 `req.Body` 生命周期

2. **修改响应读取逻辑**（约 428-436 行）
   - 立即开始读取响应（不等待 `req.Write` 完成）
   - 添加循环处理 1xx 响应
   - 手动构造并发送 1xx 响应给客户端
   - 确保在读取最终响应后检查 `req.Write` 错误

3. **添加 100 Continue 转发支持**
   - 循环读取响应
   - 检测 1xx 状态码
   - 手动写入 1xx 响应到客户端连接

### 关键代码结构

```go
// ---------------------------------------------------------
// 最终实施代码 (mproxy/https.go -> ConnectHTTPMitm case)
// ---------------------------------------------------------

// 1. 创建错误通道，缓冲区设为1防止阻塞
writeErrCh := make(chan error, 1)

// 2. 启动协程发送请求
go func() {
    // req.Write 负责把 Header 和 Body 发给 Target
    // 注意：不要在这里 Close req.Body，因为主线程还要用连接回写响应
    err := req.Write(connRemoteSite)
    writeErrCh <- err
    close(writeErrCh)
}()

// 3. 主线程循环读取响应
resp, err = func() (*http.Response, error) {
    // 【关键】这里绝对不能有 defer req.Body.Close()
    
    for {
        // 阻塞读取响应
        resp, readErr := http.ReadResponse(remote_res, req)

        // -----------------------
        // 分支 A: 读取失败
        // -----------------------
        if readErr != nil {
            // 只有读失败了，才检查是不是写错误导致的
            select {
            case writeErr := <-writeErrCh:
                if writeErr != nil {
                    return nil, fmt.Errorf("read err: %v, caused by write err: %v", readErr, writeErr)
                }
            default:
            }
            return nil, readErr
        }

        // -----------------------
        // 分支 B: 读取成功 (readErr == nil)
        // -----------------------
        
        // 处理 1xx 中间状态响应 (如 100 Continue)
        if resp.StatusCode >= 100 && resp.StatusCode < 200 {
            // 构造状态行
            statusCodeStr := strconv.Itoa(resp.StatusCode) + " "
            text := strings.TrimPrefix(resp.Status, statusCodeStr)
            statusLine := "HTTP/1.1 " + statusCodeStr + text + "\r\n"

            // 写入状态行
            if _, err := io.WriteString(connFromClinet, statusLine); err != nil {
                 return nil, err
            }
            // 写入 Header
            if err := resp.Header.Write(connFromClinet); err != nil {
                return nil, err
            }
            // 写入空行
            if _, err := io.WriteString(connFromClinet, "\r\n"); err != nil {
                return nil, err
            }

            // 清理临时 resp 的 Body (通常是空的，但为了规范)
            if resp.Body != nil {
                io.Copy(io.Discard, resp.Body)
                resp.Body.Close() // 这里的 Close 只是销毁 resp 对象，不影响连接
            }

            // 【关键】继续循环，等待最终的 200 OK
            continue
        }

        // 处理最终响应 (>= 200)，直接返回
        return resp, nil
    }
}()
```

### 验证步骤

1. **编译测试**

   ```bash
   go build -o proxy_man main.go
   ```

2. **功能测试 - 使用 Curl 上传大文件**

   ```bash
   # 测试 100 Continue 行为
   curl -v --proxy http://localhost:8080 \
     --upload-file large_file.bin \
     http://example.com/upload
   ```

3. **验证日志输出**

   - 检查是否正确转发 100 Continue
   - 检查流量统计是否正确
   - 检查连接是否正确关闭

4. **压力测试**

   - 多个并发上传请求
   - 验证连接复用是否正常

## 风险评估

### 低风险

- 修改范围有限（仅 `ConnectHTTPMitm` case）
- 不影响其他连接模式（`ConnectMitm`, `ConnectAccept`）
- 使用成熟的并发模式（项目已有类似实现）

### 中风险

- `req.Body` 生命周期管理需要仔细测试
- 错误处理路径增加，需要全面测试
- 1xx 响应转发逻辑需要验证边界情况

### 缓解措施

1. 添加详细的日志输出
2. 在测试环境充分验证后再部署
3. 保留原代码的回滚方案

## 总结

plan.md 中的修复方案**总体正确**，核心思路可以解决死锁问题。主要需要改进：

1. **`req.Body` 生命周期管理** - 在 goroutine 中关闭
2. **错误处理完善** - 确保所有错误路径都被处理
3. **连接复用验证** - 确保长连接隧道正常工作
4. **流量统计验证** - 确保 `reqBodyReader` 仍然有效

建议按上述计划实施修复，并进行充分测试。