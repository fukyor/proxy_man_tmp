# BuildBodyReader 协程安全性基准测试方案审查

## 背景

审查 `test/gotest/plan_t.md` 中的基准测试方案，该方案用于测试 `myminio.BuildBodyReader` 函数的协程安全性。

**重要更新**：根据测试需求，上传文件和下载文件的大小都必须为 **2MB**。

## 一、原方案评估（`test/gotest/plan_t.md`）

### ✅ 核心设计正确

| 要点                            | 评价                                    |
| ------------------------------- | --------------------------------------- |
| Mock Transport 拦截             | ✅ 正确消除真实网络 IO                   |
| `io.Copy(io.Discard, req.Body)` | ✅ **最关键**：防止 `io.Pipe` 写入端死锁 |
| `RunParallel` 并发测试          | ✅ 正确的压测方法                        |
| 内存数据 `largeFileBytes`       | ✅ 避免磁盘 IO 噪音                      |

### 🔴 发现的测试覆盖缺口

| 缺口                 | 说明                                                | 原方案问题                     |
| -------------------- | --------------------------------------------------- | ------------------------------ |
| `contentLength = -1` | chunked 编码走 `uploadViaTempFile` 路径（临时文件） | 固定传入 `len(largeFileBytes)` |
| `skipUpload` 分支    | SSE/WebSocket 类型跳过捕获，无 Goroutine            | 未测试透传路径                 |
| 空请求体             | 0 字节边界条件                                      | 未测试                         |

---

## 二、最终测试用例方案

### 测试用例对照表

| 用例名称                        | 测试内容               | contentLength | contentType              |
| ------------------------------- | ---------------------- | ------------- | ------------------------ |
| `BenchmarkUpload_RoutineSafety` | **原方案**：常规并发   | len(data)     | application/octet-stream |
| `BenchmarkUpload_Chunked`       | **新增**：chunked 编码 | -1            | application/octet-stream |
| `BenchmarkUpload_EmptyBody`     | **新增**：空请求体     | 0             | application/octet-stream |
| `BenchmarkUpload_SkipUpload`    | **新增**：跳过捕获路径 | len(data)     | text/event-stream        |
| `BenchmarkUpload_GoroutineLeak` | **新增**：协程泄漏检测 | len(data)     | application/octet-stream |

> 注：`HighConcurrency` 用例已删除，使用 `-cpu` 标志运行 `RoutineSafety` 即可达到相同效果。

### 关键代码片段（新增用例）

#### 用例 1：Chunked 编码

  ```go
func BenchmarkUpload_Chunked(b *testing.B) {
    // ... setup code ...

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
            reader := myminio.BuildBodyReader(
                reqBody, 10086, "req", "application/octet-stream",
                -1, // 关键：未知长度，走 uploadViaTempFile 路径
            )
            io.Copy(io.Discard, reader)
            reader.Close()
        }
    })
}
  ```

  #### 用例 2：SkipUpload 路径

  ```go
func BenchmarkUpload_SkipUpload(b *testing.B) {
    // ... setup code ...

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
            reader := myminio.BuildBodyReader(
                reqBody, 10086, "req",
                "text/event-stream", // 关键：触发 shouldSkipCapture() 返回 true
                int64(len(largeFileBytes)),
            )
            io.Copy(io.Discard, reader)
            reader.Close()
        }
    })
}
  ```

  #### 用例 3：协程泄漏检测

  ```go
func BenchmarkUpload_GoroutineLeak(b *testing.B) {
    // ... setup code ...
    var ops int64

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // ... BuildBodyReader 调用 ...

            atomic.AddInt64(&ops, 1)
            if ops%1000 == 0 {
                numGoroutines := runtime.NumGoroutine()
                b.ReportMetric(float64(numGoroutines), "goroutines")
            }
        }
    })
}
  ```

---

  ## 三、关键代码位置参考

| 组件                             | 文件位置                         |
| -------------------------------- | -------------------------------- |
| 被测函数 `BuildBodyReader`       | `myminio/minioUpload.go:49-81`   |
| 上传协程 `uploadToMinIO`         | `myminio/minioUpload.go:83-111`  |
| Read 方法                        | `myminio/minioUpload.go:159-173` |
| Close 方法（含 `doneCh` 等待）   | `myminio/minioUpload.go:175-189` |
| 临时文件路径 `uploadViaTempFile` | `myminio/minioUpload.go:114-157` |

---

  ## 四、完整测试代码（benchmark_test.go）

  ```go
  package main_test
  
  import (
     "bytes"
     "io"
     "net/http"
      "proxy_man/myminio"
      "runtime"
      "sync/atomic"
      "testing"
  
      "github.com/minio/minio-go/v7"
      "github.com/minio/minio-go/v7/pkg/credentials"
  )
  
  // ============================================================
  // 全局预加载数据（2MB 测试数据）
  // ============================================================
  var (
      largeFileBytes []byte // 2MB - 上传和下载都用这个大小
      emptyFileBytes []byte // 0 字节 - 边界条件测试
  )
  
  func init() {
      // 创建 2MB 测试数据（上传用）
      largeFileBytes = make([]byte, 2*1024*1024)
      // 填充一些数据防止被编译器过度优化
      copy(largeFileBytes, []byte("start...2MB_TEST_DATA...end"))
  
      // 空数据用于边界测试
      emptyFileBytes = []byte{}
  }
  
  // ============================================================
  // Mock Transport（核心组件）
  // ============================================================
  // 作用：拦截 MinIO SDK 的 HTTP 请求，消除真实网络 IO
  type MockMinioTransport struct {}
  
  func (m *MockMinioTransport) RoundTrip(req *http.Request) (*http.Response, error) {
      // 【关键】必须消费请求体，否则 Pipe 写入端会死锁
      // 这里模拟服务端接收上传的 2MB 数据
      io.Copy(io.Discard, req.Body)
      req.Body.Close()
  
      // 返回模拟数据（下载 2MB）
      // 注意：虽然 BuildBodyReader 只处理上传，但完整的 Mock 也需要返回数据
      return &http.Response{
          StatusCode: 200,
          Body:       io.NopCloser(bytes.NewReader(largeFileBytes)), // 2MB 下载
          Header:     make(http.Header),
          Request:    req,
      }, nil
  }
  
  // ============================================================
  // 测试初始化辅助函数
  // ============================================================
  
  func setupMinioClient(transport *MockMinioTransport) error {
      client, err := minio.New("mock.local", &minio.Options{
          Creds:     credentials.NewStaticV4("mock", "mock", ""),
          Secure:    false,
          Transport: transport,
      })
      if err != nil {
          return err
      }
  
      myminio.GlobalClient = &myminio.Client{
          Client: client,
          Config: myminio.Config{Bucket: "test-bucket"},
      }
      return nil
  }
  
  // ============================================================
  // 基准测试：常规并发（原方案）
  // ============================================================
  // 运行：go test -bench=BenchmarkUpload_RoutineSafety -benchmem -cpu=1,2,4,6
  func BenchmarkUpload_RoutineSafety(b *testing.B) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          b.Fatal(err)
      }
      b.SetParallelism(10)
      b.ResetTimer()

      b.RunParallel(func(pb *testing.PB) {
          for pb.Next() {
              reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
              reader := myminio.BuildBodyReader(
                  reqBody,
                  10086,
                  "req",
                  "application/octet-stream",
                  int64(len(largeFileBytes)), // 已知长度
              )

              io.Copy(io.Discard, reader)
              reader.Close()
          }
      })
  }

  // ============================================================
  // 基准测试：chunked 编码（contentLength = -1）
  // ============================================================
  // 运行：go test -bench=BenchmarkUpload_Chunked -benchmem
  func BenchmarkUpload_Chunked(b *testing.B) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          b.Fatal(err)
      }
      b.SetParallelism(10)
      b.ResetTimer()
  
      b.RunParallel(func(pb *testing.PB) {
          for pb.Next() {
              reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
              reader := myminio.BuildBodyReader(
                  reqBody,
                  10086,
                  "req",
                  "application/octet-stream",
                  -1, // 未知长度，走 uploadViaTempFile 路径
              )
  
              io.Copy(io.Discard, reader)
              reader.Close()
          }
      })
  }
  
  // ============================================================
  // 基准测试：空请求体
  // ============================================================
  func BenchmarkUpload_EmptyBody(b *testing.B) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          b.Fatal(err)
      }
  
      b.ResetTimer()
  
      b.RunParallel(func(pb *testing.PB) {
          for pb.Next() {
              reqBody := io.NopCloser(bytes.NewReader(emptyFileBytes))
              reader := myminio.BuildBodyReader(
                  reqBody,
                  10086,
                  "req",
                  "application/octet-stream",
                  0,
              )
  
              io.Copy(io.Discard, reader)
              reader.Close()
          }
      })
  }
  
  // ============================================================
  // 基准测试：skipUpload 路径
  // ============================================================
  func BenchmarkUpload_SkipUpload(b *testing.B) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          b.Fatal(err)
      }
      b.SetParallelism(10)
      b.ResetTimer()

      b.RunParallel(func(pb *testing.PB) {
          for pb.Next() {
              // 使用 text/event-stream 触发 skipUpload
              reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
              reader := myminio.BuildBodyReader(
                  reqBody,
                  10086,
                  "req",
                  "text/event-stream", // 这个类型会被跳过
                  int64(len(largeFileBytes)),
              )

              io.Copy(io.Discard, reader)
              reader.Close()
          }
      })
  }

  // ============================================================
  // 基准测试：协程泄漏检测
  // ============================================================
  // 运行：go test -bench=BenchmarkUpload_GoroutineLeak -benchmem
  func BenchmarkUpload_GoroutineLeak(b *testing.B) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          b.Fatal(err)
      }
  
      var ops int64
  
      b.ResetTimer()
  
      b.RunParallel(func(pb *testing.PB) {
          for pb.Next() {
              reqBody := io.NopCloser(bytes.NewReader(largeFileBytes))
              reader := myminio.BuildBodyReader(
                  reqBody,
                  10086,
                  "req",
                  "application/octet-stream",
                  int64(len(largeFileBytes)),
              )
  
              io.Copy(io.Discard, reader)
              reader.Close()
  
              atomic.AddInt64(&ops, 1)
  
              // 每 1000 次操作检查一次协程数
              if ops%1000 == 0 {
                  numGoroutines := runtime.NumGoroutine()
                  b.ReportMetric(float64(numGoroutines), "goroutines")
              }
          }
      })
  }
  
  // ============================================================
  // Close 并发调用测试
  // ============================================================
  func TestCloseConcurrency(t *testing.T) {
      if err := setupMinioClient(&MockMinioTransport{}); err != nil {
          t.Fatal(err)
      }

      reqBody := io.NopCloser(bytes.NewReader([]byte("test data")))
      reader := myminio.BuildBodyReader(
          reqBody,
          1,
          "req",
          "application/json",
          int64(len("test data")),
      )

      // 多个协程同时调用 Close
      var wg sync.WaitGroup
      for i := 0; i < 10; i++ {
          wg.Add(1)
          go func() {
              defer wg.Done()
              reader.Close()
          }()
      }

      wg.Wait()
  }
  ```

---

  ## 五、运行命令汇总

  ### 基准测试

  ```bash
  # 常规并发测试（原方案）- 使用 -cpu 控制并发度
  go test -bench=BenchmarkUpload_RoutineSafety -benchmem -cpu=2,6,12

  # Chunked 编码测试
  go test -bench=BenchmarkUpload_Chunked -benchmem -cpu=2,6,12

  # 空请求体测试
  go test -bench=BenchmarkUpload_EmptyBody -benchmem

  # SkipUpload 路径测试
  go test -bench=BenchmarkUpload_SkipUpload -benchmem

  # 协程泄漏检测 + 内存 profile
  go test -bench=BenchmarkUpload_GoroutineLeak -benchmem -memprofile=mem.prof
  go tool pprof mem.prof
  ```

  ### 数据竞争检测

  ```bash
# 运行所有基准测试并检查数据竞争
go test -race -bench=. -count=5 -v

  ```

  ## 六、预期结果

  ### 正常情况下的指标（2MB 文件）

| 测试          | ns/op      | B/op    | allocs/op        |
| ------------- | ---------- | ------- | ---------------- |
| RoutineSafety | ~1,000,000 | ~20,000 | ~100             |
| Chunked       | ~1,600,000 | ~40,000 | ~200（临时文件） |
| EmptyBody     | ~100,000   | ~5,000  | ~20              |
| SkipUpload    | ~100,000   | ~2,000  | ~10              |

  ### 需要关注的异常信号

    1. **协程数持续增长** - 表示 Goroutine 泄漏
    2. **B/op 随时间增加** - 表示内存泄漏
    3. **race detector 报告** - 表示数据竞争
    4. **测试卡死** - 表示死锁

---

  ## 七、关键文件位置

| 文件       | 路径                             |
| ---------- | -------------------------------- |
| 被测函数   | `myminio/minioUpload.go:49-81`   |
| Read 方法  | `myminio/minioUpload.go:159-173` |
| Close 方法 | `myminio/minioUpload.go:175-189` |
| 上传协程   | `myminio/minioUpload.go:83-111`  |
| 测试方案   | `test/gotest/plan_t.md`          |

---

  ## 八、补充说明

  ### 为什么文件大小改为 2MB？

    1. **更真实的压力测试**：2MB 文件更能暴露协程调度和内存分配问题
    2. **临时文件路径测试**：chunked 编码时会创建 2MB 临时文件，验证 `uploadViaTempFile` 的完整性
    3. **性能指标校准**：2MB 数据下的性能指标可作为生产环境的参考

  ### 上传 vs 下载

  - **上传**：`BuildBodyReader` 将请求/响应体通过 Pipe 传递给 MinIO SDK
  - **下载**：Mock Transport 的 `RoundTrip` 返回 2MB 响应体（虽然当前实现不读取，但保持完整性）