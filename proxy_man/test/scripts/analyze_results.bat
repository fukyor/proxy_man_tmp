@echo off
REM ═════════════════════════════════════════════════════════════
REM  测试结果分析脚本
REM
REM  功能：
REM    1. 解析 AB 测试日志提取关键指标
REM    2. 生成性能对比表格
REM    3. 输出 Markdown 格式报告
REM ═════════════════════════════════════════════════════════════

setlocal enabledelayedexpansion

set RESULTS_DIR=..\results
set REPORT_FILE=%RESULTS_DIR%\test_report.md
set SUMMARY_FILE=%RESULTS_DIR%\summary.txt

echo.
echo ════════════════════════════════════════════════════════════
echo   📊 测试结果分析
echo ════════════════════════════════════════════════════════════
echo.

REM 检查结果目录是否存在
if not exist "%RESULTS_DIR%" (
    echo ❌ 错误: 结果目录不存在: %RESULTS_DIR%
    echo 请先运行 run_ab.bat 执行测试
    exit /b 1
)

REM 创建 Markdown 报告头部
(
echo # MinIO 上传性能测试报告
echo.
echo **生成时间**: %date% %time%
echo.
echo ---
echo.
echo ## 测试环境
echo.
echo - 代理服务器: localhost:8080
echo - 后端服务器: localhost:9001
echo - MinIO 服务器: localhost:9000
echo.
echo ---
echo.
echo ## 测试结果汇总
echo.
echo ### 测试矩阵
echo.
echo | 测试ID | 文件大小 | 并发数 | 请求数 | QPS | 平均延迟 | P99延迟 | 传输量 |
echo |--------|----------|--------|--------|-----|----------|---------|--------|
) > "%REPORT_FILE%"

REM 解析日志文件
set /a test_count=0

for %%f in ("%RESULTS_DIR%\t*.log") do (
    set /a test_count+=1
    set "logfile=%%f"

    REM 提取测试信息
    for /f "tokens=1-4 delims=_" %%a in ("%%~nf") do (
        set "test_id=%%a"
        set "file_size=%%b"
        set "concurrency=%%c"
        set "requests=%%d"
    )

    REM 解析 QPS
    set "qps=N/A"
    for /f "tokens=2 delims=:" %%q in ('type "%%f" ^| findstr /C:"Requests per second"') do (
        set "qps_line=%%q"
        for /f "tokens=1" %%v in ("%%q") do set "qps=%%v"
    )

    REM 解析平均延迟
    set "mean_lat=N/A"
    for /f "tokens=2 delims=:" %%m in ('type "%%f" ^| findstr /C:"Time per request:.*mean"') do (
        set "mean_line=%%m"
        for /f "tokens=1" %%v in ("%%m") do set "mean_lat=%%v"
    )

    REM 解析 P99 延迟
    set "p99_lat=N/A"
    for /f "tokens=2 delims=:" %%p in ('type "%%f" ^| findstr /C:"99%%"') do (
        set "p99_line=%%p"
        for /f "tokens=1,2" %%v in ("%%p") do (
            set "p99_lat=%%v ms"
        )
    )

    REM 解析传输量
    set "transfer=0"
    for /f "tokens=2 delims=:" %%t in ('type "%%f" ^| findstr /C:"Transfer rate"') do (
        set "transfer_line=%%t"
        for /f "tokens=1" %%v in ("%%t") do set "transfer=%%v"
    )

    REM 确定文件大小描述
    set "size_desc=1KB"
    if "!file_size!"=="small" set "size_desc=1KB"
    if "!file_size!"=="medium" set "size_desc=100KB"
    if "!file_size!"=="large" set "size_desc=1MB"

    REM 写入表格行
    echo | !test_id! | !size_desc! | !concurrency! | !requests! | !qps! | !mean_lat! | !p99_lat! | !transfer! | >> "%REPORT_FILE%"
)

REM 添加性能分析章节
(
echo.
echo ---
echo.
echo ## 性能分析
echo.
echo ### QPS 对比
echo.
) >> "%REPORT_FILE%"

REM 统计不同文件大小的 QPS
echo #### 小文件 (1KB)
echo. >> "%REPORT_FILE%"
echo | 并发数 | QPS |
echo |--------|-----| >> "%REPORT_FILE%"

for %%f in ("%RESULTS_DIR%\t*_small_*.log") do (
    for /f "tokens=2,3 delims=_" %%a in ("%%~nf") do (
        set "size=%%a"
        set "conc_req=%%b"
    )
    for /f "tokens=2 delims=c" %%c in ("!conc_req!") do set "conc=%%c"

    set "qps=0"
    for /f "tokens=2 delims=:" %%q in ('type "%%f" ^| findstr /C:"Requests per second"') do (
        for /f "tokens=1" %%v in ("%%q") do set "qps=%%v"
    )
    echo | !conc! | !qps! | >> "%REPORT_FILE%"
)

echo. >> "%REPORT_FILE%"

REM 生成摘要文件
(
echo ════════════════════════════════════════════════════════════
echo  测试结果摘要
echo ════════════════════════════════════════════════════════════
echo.
echo 测试文件数量: %test_count%
echo.
echo 最高 QPS 测试:
) > "%SUMMARY_FILE%"

REM 查找最高 QPS
set "max_qps=0"
set "max_qps_file="

for %%f in ("%RESULTS_DIR%\t*.log") do (
    for /f "tokens=2 delims=:" %%q in ('type "%%f" ^| findstr /C:"Requests per second"') do (
        for /f "tokens=1" %%v in ("%%q") do (
            set "current_qps=%%v"
            set "current_qps=!current_qps:[=#!
            REM 简单的数值比较（Windows batch 限制）
            if "!max_qps!"=="0" (
                set "max_qps=!current_qps!"
                set "max_qps_file=%%~nxf"
            )
        )
    )
)

echo   文件: !max_qps_file! >> "%SUMMARY_FILE%"
echo   QPS: !max_qps! >> "%SUMMARY_FILE%"

echo.
echo ════════════════════════════════════════════════════════════
echo   ✅ 分析完成！
echo ════════════════════════════════════════════════════════════
echo.
echo 📄 报告文件:
echo   - Markdown: %REPORT_FILE%
echo   - 摘要: %SUMMARY_FILE%
echo.
echo 💡 查看报告:
echo   type "%REPORT_FILE%"
echo.

endlocal