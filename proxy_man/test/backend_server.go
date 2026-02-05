package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"bytes"
)

// TestData 测试数据结构
type TestData struct {
	Name   string
	Size   int64
	Data   []byte
}

// 内置测试数据
var testFiles = map[string]TestData{
	"small_1k":   {Name: "small_1k.bin", Size: 1024, Data: generateBytes(1024)},
	"medium_100k": {Name: "medium_100k.bin", Size: 102400, Data: generateBytes(102400)},
	"large_1m":    {Name: "large_1m.bin", Size: 1024 * 1024, Data: generateBytes(1024 * 1024)},
}

// generateBytes 生成指定长度的测试字节流
func generateBytes(size int64) []byte {
	data := make([]byte, size)
	for i := int64(0); i < size; i++ {
		data[i] = byte(i % 256)
	}
	return data
}

// handleTestDownload 处理测试下载请求
func handleTestDownload(w http.ResponseWriter, r *http.Request) {
	// 获取文件名参数
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "缺少 file 参数", http.StatusBadRequest)
		return
	}

	// 查找测试数据
	for _, testData := range testFiles {
		if testData.Name == filename {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", testData.Size))

			start := time.Now()
			written, _ := io.Copy(w, bytes.NewReader(testData.Data))
			duration := time.Since(start)

			log.Printf("[下载] 文件: %s | 大小: %d 字节 | 耗时: %v | 速度: %.2f MB/s",
				filename, written, duration, float64(written)/(1024*1024)/duration.Seconds())
			return
		}
	}

	http.Error(w, "文件不存在", http.StatusNotFound)
}

// handleTestUpload 处理测试上传请求
func handleTestUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	start := time.Now()
	size, err := io.Copy(io.Discard, r.Body)
	r.Body.Close()
	duration := time.Since(start)

	if err != nil {
		log.Printf("[上传] 错误: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	speed := float64(size) / (1024 * 1024) / duration.Seconds()
	log.Printf("[上传] 接收大小: %d 字节 | 耗时: %v | 速度: %.2f MB/s",
		size, duration, speed)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","size":%d,"duration_ms":%d,"speed_mb_s":%.2f}`,
		size, duration.Milliseconds(), speed)
}

// handleRoot 根路径，显示可用接口
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>测试后端服务器</title>
    <meta charset="utf-8">
    <style>
        body { font-family: monospace; margin: 40px; background: #1e1e1e; color: #d4d4d4; }
        h1 { color: #4ec9b0; }
        .endpoint { background: #252526; padding: 15px; margin: 10px 0; border-left: 3px solid #4ec9b0; }
        .method { color: #dcdcaa; font-weight: bold; }
        .path { color: #9cdcfe; }
        .desc { color: #6a9955; margin-top: 5px; }
    </style>
</head>
<body>
    <h1>🧪 测试后端服务器 (端口 9001)</h1>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/test/download?file=small_1k.bin</span></div>
        <div class="desc">返回 1KB 测试数据</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/test/download?file=medium_100k.bin</span></div>
        <div class="desc">返回 100KB 测试数据</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/test/download?file=large_1m.bin</span></div>
        <div class="desc">返回 1MB 测试数据</div>
    </div>
    <div class="endpoint">
        <div><span class="method">POST</span> <span class="path">/test/upload</span></div>
        <div class="desc">接收上传数据并返回统计信息</div>
    </div>
    <div class="endpoint">
        <div><span class="method">GET</span> <span class="path">/health</span></div>
        <div class="desc">健康检查</div>
    </div>
</body>
</html>
`
	w.Write([]byte(html))
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","time":"%s"}`, time.Now().Format(time.RFC3339))
}

func main() {
	mux := http.NewServeMux()

	// 注册路由
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/test/download", handleTestDownload)
	mux.HandleFunc("/test/upload", handleTestUpload)

	server := &http.Server{
		Addr:         ":9001",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Println("🚀 测试后端服务器启动在 :9001")
	log.Println("📄 访问 http://localhost:9001 查看可用接口")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}