package main_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ================= 环境配置 =================
const (
	// 代理服务器地址 (您手动启动的 proxy_man 监听地址)
	// 对应 main.go 中的 -addr 参数，默认为 :8080
	ProxyAddr = "127.0.0.1:8080"

	// 后端服务器地址 (您手动启动的 backend_server 地址)
	// 请根据实际情况修改端口，例如 8081, 8000 等
	HttpBackendBaseURL = "http://127.0.0.1:9001"
	HttpsBackendBaseURL = "https://127.0.0.1:9002"

	// 测试数据目录
	TestDataDir = `E:\D\zuoyewenjian\MyProject\proxy_man\test\data`
)

// ===========================================

func TestProxyRequests_Client(t *testing.T) {
	// 1. 配置 HTTP Client 使用您的代理服务器
	proxyURL, err := url.Parse("http://" + ProxyAddr)
	if err != nil {
		t.Fatalf("代理地址配置错误: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			// 禁用压缩以确保 Content-Length 准确匹配文件大小 (可选)
			DisableCompression: true,
		},
		// 设置较长的超时时间，以防传输大文件时中断
		Timeout: 30 * time.Minute,
	}

	// 2. 读取测试文件列表
	entries, err := os.ReadDir(TestDataDir)
	if err != nil {
		t.Fatalf("❌ 无法读取测试数据目录 '%s': %v", TestDataDir, err)
	}

	var testFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			testFiles = append(testFiles, entry.Name())
		}
	}

	// 3. 循环测试每个文件
	for _, fileName := range testFiles {
		filePath := filepath.Join(TestDataDir, fileName)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			t.Errorf("⚠️ 无法获取文件信息 %s: %v", fileName, err)
			continue
		}
		fileSize := fileInfo.Size()
		// === A. 上传测试 (Upload) ===
		t.Run("httpUpload", func(t *testing.T) {
			// 打开文件
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("无法打开文件: %v", err)
			}
			defer f.Close()

			uploadURL := fmt.Sprintf("%s/test/upload", HttpBackendBaseURL)
			t.Logf("⬆️ [Upload] %s (%d bytes) -> %s", fileName, fileSize, uploadURL)

			// 构造请求
			req, err := http.NewRequest("POST", uploadURL, f)
			if err != nil {
				t.Fatalf("创建请求失败: %v", err)
			}
			req.Header.Set("Content-Type", "application/octet-stream")
			// 显式设置 Content-Length (虽然 Go 通常会自动处理，但在代理测试中明确指定有助于排查问题)
			req.ContentLength = fileSize

			// 发送请求
			start := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("❌ 请求失败 (请检查代理或后端是否启动): %v", err)
			}
			defer resp.Body.Close()

			// 读取响应
			body, _ := io.ReadAll(resp.Body)
			duration := time.Since(start)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("❌ 上传失败: Status %d | Body: %s", resp.StatusCode, string(body))
			} else {
				t.Logf("✅ 上传成功: 耗时 %v | 响应: %s", duration, string(bytes.TrimSpace(body)))
			}
		})

		t.Run("httpsUpload", func(t *testing.T) {
			// 打开文件
			f, err := os.Open(filePath)
			if err != nil {
				t.Fatalf("无法打开文件: %v", err)
			}
			defer f.Close()

			uploadURL := fmt.Sprintf("%s/test/upload", HttpsBackendBaseURL)
			t.Logf("⬆️ [Upload] %s (%d bytes) -> %s", fileName, fileSize, uploadURL)

			// 构造请求
			req, err := http.NewRequest("POST", uploadURL, f)
			if err != nil {
				t.Fatalf("创建请求失败: %v", err)
			}
			req.Header.Set("Content-Type", "application/octet-stream")
			// 显式设置 Content-Length (虽然 Go 通常会自动处理，但在代理测试中明确指定有助于排查问题)
			req.ContentLength = fileSize

			// 发送请求
			start := time.Now()
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("❌ 请求失败 (请检查代理或后端是否启动): %v", err)
			}
			defer resp.Body.Close()

			// 读取响应
			body, _ := io.ReadAll(resp.Body)
			duration := time.Since(start)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("❌ 上传失败: Status %d | Body: %s", resp.StatusCode, string(body))
			} else {
				t.Logf("✅ 上传成功: 耗时 %v | 响应: %s", duration, string(bytes.TrimSpace(body)))
			}
		})


		// === B. 下载测试 (Download) ===
		t.Run("Download", func(t *testing.T) {
			httpDownloadURL := fmt.Sprintf("%s/test/download?file=%s", HttpBackendBaseURL, fileName)
			t.Logf("⬇️ [Download] %s <- %s", fileName, httpDownloadURL)

			start := time.Now()
			resp, err := client.Get(httpDownloadURL)
			if err != nil {
				t.Fatalf("❌ 请求失败: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("❌ 下载失败: Status %d | Body: %s", resp.StatusCode, string(body))
			}

			// 验证数据大小
			// 使用 io.Copy(io.Discard) 避免将大文件读入内存，只统计字节数
			receivedBytes, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				t.Fatalf("❌ 读取响应流失败: %v", err)
			}
			duration := time.Since(start)

			// 验证
			if receivedBytes != fileSize {
				t.Errorf("❌ 数据不完整: 期望 %d bytes, 实际接收 %d bytes", fileSize, receivedBytes)
			} else {
				// 计算吞吐量 (MB/s)
				throughput := float64(receivedBytes) / 1024 / 1024 / duration.Seconds()
				t.Logf("✅ 下载成功: %d bytes | 耗时 %v | 速度 %.2f MB/s", receivedBytes, duration, throughput)
			}
		})
		fmt.Println("---------------------------------------------------")

		// === B. https下载测试 (Download) ===
		t.Run("Download", func(t *testing.T) {
			httpsDownloadURL := fmt.Sprintf("%s/test/download?file=%s", HttpsBackendBaseURL, fileName)
			t.Logf("⬇️ [Download] %s <- %s", fileName, httpsDownloadURL)

			start := time.Now()
			resp, err := client.Get(httpsDownloadURL)
			if err != nil {
				t.Fatalf("❌ 请求失败: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("❌ 下载失败: Status %d | Body: %s", resp.StatusCode, string(body))
			}

			// 验证数据大小
			// 使用 io.Copy(io.Discard) 避免将大文件读入内存，只统计字节数
			receivedBytes, err := io.Copy(io.Discard, resp.Body)
			if err != nil {
				t.Fatalf("❌ 读取响应流失败: %v", err)
			}
			duration := time.Since(start)

			// 验证
			if receivedBytes != fileSize {
				t.Errorf("❌ 数据不完整: 期望 %d bytes, 实际接收 %d bytes", fileSize, receivedBytes)
			} else {
				// 计算吞吐量 (MB/s)
				throughput := float64(receivedBytes) / 1024 / 1024 / duration.Seconds()
				t.Logf("✅ 下载成功: %d bytes | 耗时 %v | 速度 %.2f MB/s", receivedBytes, duration, throughput)
			}
		})
		fmt.Println("---------------------------------------------------")
	}
}