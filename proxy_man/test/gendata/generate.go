package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// TestDataConfig 测试数据配置
type TestDataConfig struct {
	Name string
	Size int64
	Desc string
}

// 测试数据配置列表
var testDataConfigs = []TestDataConfig{
	{"small_1k.bin", 1024, "1KB 小文件"},
	{"medium_100k.bin", 100 * 1024, "100KB 中等文件"},
	{"large_1m.bin", 1024 * 1024, "1MB 大文件"},
	{"huge_5m.bin", 5 * 1024 * 1024, "5MB 超大文件"},
}

// generateRandomBytes 生成随机字节流（更真实的测试数据）
func generateRandomBytes(size int64) ([]byte, error) {
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return nil, fmt.Errorf("生成随机数据失败: %w", err)
	}
	return data, nil
}

// ensureDir 确保目录存在
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return nil
}

// writeTestFile 写入测试文件
func writeTestFile(dir string, config TestDataConfig) error {
	filePath := filepath.Join(dir, config.Name)

	// 检查文件是否已存在
	if info, err := os.Stat(filePath); err == nil {
		if info.Size() == config.Size {
			log.Printf("✓ %s 已存在，大小正确 (%d KB)", config.Name, config.Size/1024)
			return nil
		}
		log.Printf("⚠ %s 已存在但大小不匹配，重新生成", config.Name)
	}

	log.Printf("生成 %s (%d bytes)...", config.Desc, config.Size)

	data, err := generateRandomBytes(config.Size)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	log.Printf("✓ 已生成: %s (%d bytes)", config.Name, config.Size)
	return nil
}

// printBanner 打印横幅
func printBanner() {
	banner := `
╔════════════════════════════════════════════════════════════╗
║          🧪 AB 压力测试数据生成工具                          ║
║                                                              ║
║  生成随机二进制测试文件，用于 Apache Bench 压力测试           ║
╚════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}

// printSummary 打印摘要
func printSummary(dir string) {
	fmt.Println(`
════════════════════════════════════════════════════════════
                        📊 生成完成
════════════════════════════════════════════════════════════
`)

	totalSize := int64(0)
	for _, config := range testDataConfigs {
		filePath := filepath.Join(dir, config.Name)
		if info, err := os.Stat(filePath); err == nil {
			totalSize += info.Size()
			sizeKB := float64(info.Size()) / 1024
			sizeMB := sizeKB / 1024

			var sizeStr string
			if sizeMB >= 1 {
				sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
			} else {
				sizeStr = fmt.Sprintf("%.2f KB", sizeKB)
			}

			fmt.Printf("  ✓ %-20s %8s  %s\n", config.Name, sizeStr, config.Desc)
		}
	}

	totalMB := float64(totalSize) / (1024 * 1024)
	fmt.Printf("\n  📁 输出目录: %s\n", dir)
	fmt.Printf("  📦 总大小: %.2f MB\n", totalMB)
	fmt.Println(`
════════════════════════════════════════════════════════════
使用示例:
  ab -n 1000 -c 10 -p testdata/small_1k.bin -T application/octet-stream \\
     http://localhost:8080/test/upload
════════════════════════════════════════════════════════════
`)
}

func main() {
	printBanner()
	execDir, _ := os.Getwd()
	outputDir := filepath.Join(execDir, "test/data")
	fmt.Println(outputDir)

	// 确保输出目录存在
	if err := ensureDir(outputDir); err != nil {
		log.Fatal("❌ 错误:", err)
	}

	// 生成所有测试文件
	for _, config := range testDataConfigs {
		if err := writeTestFile(outputDir, config); err != nil {
			log.Fatal("❌ 生成文件失败:", err)
		}
	}

	// printSummary(outputDir)
}