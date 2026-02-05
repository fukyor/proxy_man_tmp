package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"
)

// 配置区
const (
	TargetURL = "http://localhost:8080/test/upload"
	TestData  = "test_1mb.bin"
)

func main() {
	// 1. 准备阶段：生成测试文件
	createTestFile(TestData, 1024*1024)
	defer os.Remove(TestData) // 跑完自动清理

	// 2. 运行阶段：定义测试矩阵
	cases := []struct {
		Concurrency int
		Requests    int
	}{
		{10, 1000},
		{50, 2000},
		{100, 5000},
	}

	fmt.Println("🚀 开始 MinIO 代理压力测试...")
	fmt.Printf("%-10s %-10s %-15s %-15s\n", "并发数", "请求数", "QPS", "平均耗时(ms)")
	fmt.Println("-------------------------------------------------------")

	for _, c := range cases {
		// 调用系统安装的 ab
		// 注意：Windows下 ab 需要在 PATH 中
		cmd := exec.Command("ab",
			"-k",                                     // Keep-Alive
			"-n", fmt.Sprintf("%d", c.Requests),      // 总请求数
			"-c", fmt.Sprintf("%d", c.Concurrency),   // 并发数
			"-p", TestData,                           // POST 文件
			"-T", "application/octet-stream",         // Content-Type
			"-q",                                     // 静默模式
			TargetURL,
		)

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("测试失败 [C:%d]: %v", c.Concurrency, err)
			continue
		}

		// 3. 分析阶段：用正则提取结果 (比 Batch 脚本好写一万倍)
		qps := parseMetric(output, `Requests per second:\s+([\d\.]+)`)
		latency := parseMetric(output, `Time per request:\s+([\d\.]+)\s+\[ms\]\s+\(mean\)`)

		fmt.Printf("%-10d %-10d %-15s %-15s\n", c.Concurrency, c.Requests, qps, latency)
		
		// 休息一下让端口回收
		time.Sleep(2 * time.Second)
	}
}

func createTestFile(name string, size int64) {
	f, _ := os.Create(name)
	f.Truncate(size) // 快速生成空洞文件，用于测试足够了
	f.Close()
}

func parseMetric(output []byte, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindSubmatch(output)
	if len(matches) > 1 {
		return string(matches[1])
	}
	return "N/A"
}