package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProfileInfo 性能分析文件信息
type ProfileInfo struct {
	Name        string
	Description string
	Command     string
}

// 支持的性能分析类型
var profileTypes = []ProfileInfo{
	{
		Name:        "cpu",
		Description: "CPU 性能分析",
		Command:     "top",
	},
	{
		Name:        "mem",
		Description: "内存性能分析",
		Command:     "top",
	},
	{
		Name:        "heap",
		Description: "堆内存分配",
		Command:     "top",
	},
	{
		Name:        "block",
		Description: "阻塞分析",
		Command:     "top",
	},
	{
		Name:        "mutex",
		Description: "锁竞争分析",
		Command:     "top",
	},
}

// printBanner 打印横幅
func printBanner() {
	banner := `
╔════════════════════════════════════════════════════════════╗
║          🔍 pprof 性能分析工具                             ║
║                                                              ║
║  用于分析 Go 程序的 CPU 和内存性能                          ║
╚════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}

// checkGoTool 检查 go 工具是否可用
func checkGoTool() bool {
	_, err := exec.LookPath("go")
	if err != nil {
		return false
	}
	return true
}

// checkPprofFile 检查性能分析文件是否存在
func checkPprofFile(dir, profileType string) (string, bool) {
	// 尝试不同的文件扩展名
	extensions := []string{".prof", ".pprof"}

	for _, ext := range extensions {
		filename := filepath.Join(dir, profileType+ext)
		if info, err := os.Stat(filename); err == nil && info.Size() > 0 {
			return filename, true
		}
	}

	// 检查当前目录
	cwd, err := os.Getwd()
	if err == nil {
		for _, ext := range extensions {
			filename := filepath.Join(cwd, profileType+ext)
			if info, err := os.Stat(filename); err == nil && info.Size() > 0 {
				return filename, true
			}
		}
	}

	return "", false
}

// listAvailableProfiles 列出可用的性能分析文件
func listAvailableProfiles(dir string) []string {
	available := []string{}

	for _, profile := range profileTypes {
		if file, found := checkPprofFile(dir, profile.Name); found {
			available = append(available, file)
		}
	}

	return available
}

// runPprofTop 运行 pprof top 命令
func runPprofTop(profileFile string) error {
	fmt.Printf("\n🔍 分析 %s...\n", filepath.Base(profileFile))
	fmt.Println(strings.Repeat("─", 60))

	cmd := exec.Command("go", "tool", "pprof", "-top", profileFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runPprofList 列出特定函数的性能
func runPprofList(profileFile, functionName string) error {
	fmt.Printf("\n🔍 函数 %s 的详细性能...\n", functionName)
	fmt.Println(strings.Repeat("─", 60))

	cmd := exec.Command("go", "tool", "pprof", "-list="+functionName, profileFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runPprofWeb 启动 pprof Web 界面
func runPprofWeb(profileFile string, port string) error {
	fmt.Printf("\n🌐 启动 Web 界面在 http://localhost:%s\n", port)
	fmt.Println("按 Ctrl+C 停止服务器")

	cmd := exec.Command("go", "tool", "pprof", "-http=:"+port, profileFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runPprofDot 生成调用图（需要 graphviz）
func runPprofDot(profileFile, outputFile string) error {
	fmt.Printf("\n📊 生成调用图到 %s...\n", outputFile)
	fmt.Println(strings.Repeat("─", 60))

	cmd := exec.Command("go", "tool", "pprof", "-pdf", profileFile)
	outFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// printUsage 打印使用说明
func printUsage(scriptName string) {
	fmt.Printf(`
使用方法:
  go run %s [选项]

选项:
  -list <函数名>    列出指定函数的性能详情
  -web [端口]      启动 Web 界面（默认端口 8080）
  -dot <输出文件>  生成 PDF 调用图（需要安装 graphviz）
  -top             显示性能热点（默认）

示例:
  # 分析 CPU 性能热点
  go run %s cpu.prof

  # 查看 uploadToMinIO 函数的 CPU 使用
  go run %s cpu.prof -list uploadToMinIO

  # 启动 Web 界面查看火焰图
  go run %s cpu.prof -web 8080

  # 生成内存调用图
  go run %s mem.prof -dot callgraph.pdf

注意:
  - 需要 Go 工具链安装
  - Web 界面功能最强大，推荐使用
  - -dot 需要先安装 Graphviz（https://graphviz.org/）
`, scriptName, scriptName, scriptName, scriptName, scriptName)
}

// printProfileSummary 打印性能分析摘要
func printProfileSummary(dir string) {
	fmt.Println("\n📁 可用的性能分析文件:")
	fmt.Println(strings.Repeat("─", 60))

	available := listAvailableProfiles(dir)
	if len(available) == 0 {
		fmt.Println("  ⚠ 未找到性能分析文件")
		fmt.Println("\n💡 提示: 启动程序时使用以下参数生成性能分析文件:")
		fmt.Println("   proxy_man.exe -cpuprofile=cpu.prof -memprofile=mem.prof")
		return
	}

	for i, file := range available {
		info, _ := os.Stat(file)
		sizeMB := float64(info.Size()) / (1024 * 1024)
		fmt.Printf("  %d. %s (%.2f MB)\n", i+1, filepath.Base(file), sizeMB)
	}
}

func main() {
	printBanner()

	// 检查 go 工具
	if !checkGoTool() {
		log.Fatal("❌ 错误: 未找到 go 命令，请确保 Go 已安装")
	}

	// 解析命令行参数
	args := os.Args[1:]
	if len(args) == 0 {
		// 无参数，显示当前目录的可用文件
		cwd, _ := os.Getwd()
		printProfileSummary(cwd)
		printUsage(filepath.Base(os.Args[0]))
		return
	}

	// 获取性能分析文件路径
	profileFile := args[0]
	if _, err := os.Stat(profileFile); err != nil {
		// 尝试在当前目录查找
		cwd, _ := os.Getwd()
		if file, found := checkPprofFile(cwd, profileFile); found {
			profileFile = file
		} else {
			log.Fatalf("❌ 错误: 找不到性能分析文件: %s", profileFile)
		}
	}

	// 解析选项
	action := "top"
	webPort := "8080"
	targetFunc := ""
	dotOutput := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-list":
			if i+1 < len(args) {
				targetFunc = args[i+1]
				i++
			}
		case "-web":
			action = "web"
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				webPort = args[i+1]
				i++
			}
		case "-dot":
			action = "dot"
			if i+1 < len(args) {
				dotOutput = args[i+1]
				i++
			}
		case "-top":
			action = "top"
		case "-h", "-help", "--help":
			printUsage(filepath.Base(os.Args[0]))
			return
		}
	}

	// 执行对应的操作
	var err error
	switch action {
	case "top":
		err = runPprofTop(profileFile)
	case "web":
		err = runPprofWeb(profileFile, webPort)
	case "dot":
		if dotOutput == "" {
			dotOutput = strings.TrimSuffix(profileFile, filepath.Ext(profileFile)) + ".pdf"
		}
		err = runPprofDot(profileFile, dotOutput)
	}

	if err != nil {
		log.Fatalf("❌ 执行失败: %v", err)
	}

	// 如果指定了函数列表，额外执行
	if targetFunc != "" && action == "top" {
		if err := runPprofList(profileFile, targetFunc); err != nil {
			log.Printf("⚠ 列出函数失败: %v", err)
		}
	}

	// 打印提示
	if action == "top" {
		fmt.Println("\n💡 提示:")
		fmt.Println("   使用 -web 启动交互式 Web 界面查看火焰图")
		fmt.Println("   使用 -list <函数名> 查看特定函数的性能")
		fmt.Printf("   示例: go run %s %s -web\n", filepath.Base(os.Args[0]), filepath.Base(profileFile))
	}
}