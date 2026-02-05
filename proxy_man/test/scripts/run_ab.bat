@echo off
REM ═════════════════════════════════════════════════════════════
REM  MinIO 上传 AB 压力测试脚本
REM
REM  前提条件：
REM    1. 代理服务器运行在 localhost:8080
REM    2. 后端测试服务器运行在 localhost:9001
REM    3. 测试数据文件已生成
REM ═════════════════════════════════════════════════════════════

setlocal enabledelayedexpansion

REM 配置参数
set PROXY_URL=http://localhost:8080
set BACKEND_URL=http://localhost:9001
set TESTDATA_DIR=..\testdata
set RESULTS_DIR=..\results

REM 颜色代码（Windows 10+）
set GREEN=[92m
set RED=[91m
set YELLOW=[93m
set BLUE=[94m
set RESET=[0m

echo.
echo ════════════════════════════════════════════════════════════
echo   %GREEN%🧪 AB 压力测试 - MinIO 上传性能测试%RESET%
echo ════════════════════════════════════════════════════════════
echo.

REM 检查 ab 命令是否可用
where ab >nul 2>&1
if %errorlevel% neq 0 (
    echo %RED%❌ 错误: 未找到 Apache Bench (ab) 命令%RESET%
    echo.
    echo 请安装 Apache HTTP Server 或将 ab.exe 添加到 PATH
    echo 下载地址: https://www.apachelounge.com/download/
    exit /b 1
)

REM 创建结果目录
if not exist "%RESULTS_DIR%" mkdir "%RESULTS_DIR%"

REM 检查测试文件是否存在
if not exist "%TESTDATA_DIR%\small_1k.bin" (
    echo %RED%❌ 错误: 测试数据文件不存在%RESET%
    echo 请先运行: go run testdata\generate.go
    exit /b 1
)

echo %BLUE%ℹ️ 测试配置:%RESET%
echo   代理服务器: %PROXY_URL%
echo   后端服务器: %BACKEND_URL%
echo   测试数据目录: %TESTDATA_DIR%
echo   结果输出目录: %RESULTS_DIR%
echo.

REM ════════════════════════════════════════════════════════════
REM   测试矩阵
REM ════════════════════════════════════════════════════════════

echo.
echo ════════════════════════════════════════════════════════════
echo   %YELLOW%阶段 1: 小文件高并发测试 (1KB)%RESET%
echo ════════════════════════════════════════════════════════════
echo.

REM T1: 1 并发, 1000 请求
echo [%GREEN%T1%RESET%] 小文件 | 并发: 1 | 请求: 1000 | 预期 QPS: ^>500
ab -n 1000 -c 1 -p "%TESTDATA_DIR%\small_1k.bin" -T application/octet-stream ^
   -g "%RESULTS_DIR%\t1_small_1c_1k.tsv" ^
   "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t1_small_1c_1k.log" 2>&1
findstr /C:"Requests per second" "%RESULTS_DIR%\t1_small_1c_1k.log"
echo.

REM T2: 10 并发, 5000 请求
echo [%GREEN%T2%RESET%] 小文件 | 并发: 10 | 请求: 5000 | 预期 QPS: ^>1000
ab -n 5000 -c 10 -p "%TESTDATA_DIR%\small_1k.bin" -T application/octet-stream ^
   -g "%RESULTS_DIR%\t2_small_10c_5k.tsv" ^
   "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t2_small_10c_5k.log" 2>&1
findstr /C:"Requests per second" "%RESULTS_DIR%\t2_small_10c_5k.log"
echo.

REM T3: 50 并发, 10000 请求
echo [%GREEN%T3%RESET%] 小文件 | 并发: 50 | 请求: 10000 | 预期 QPS: ^>2000
ab -n 10000 -c 50 -p "%TESTDATA_DIR%\small_1k.bin" -T application/octet-stream ^
   -g "%RESULTS_DIR%\t3_small_50c_10k.tsv" ^
   "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t3_small_50c_10k.log" 2>&1
findstr /C:"Requests per second" "%RESULTS_DIR%\t3_small_50c_10k.log"
echo.

timeout /t 3 >nul

REM T4: 100 并发, 10000 请求
echo [%GREEN%T4%RESET%] 小文件 | 并发: 100 | 请求: 10000 | 预期 QPS: ^>3000
ab -n 10000 -c 100 -p "%TESTDATA_DIR%\small_1k.bin" -T application/octet-stream ^
   -g "%RESULTS_DIR%\t4_small_100c_10k.tsv" ^
   "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t4_small_100c_10k.log" 2>&1
findstr /C:"Requests per second" "%RESULTS_DIR%\t4_small_100c_10k.log"
echo.

timeout /t 3 >nul

REM T5: 200 并发, 10000 请求
echo [%GREEN%T5%RESET%] 小文件 | 并发: 200 | 请求: 10000 | 预期 QPS: ^>4000
ab -n 10000 -c 200 -p "%TESTDATA_DIR%\small_1k.bin" -T application/octet-stream ^
   -g "%RESULTS_DIR%\t5_small_200c_10k.tsv" ^
   "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t5_small_200c_10k.log" 2>&1
findstr /C:"Requests per second" "%RESULTS_DIR%\t5_small_200c_10k.log"
echo.

echo.
echo ════════════════════════════════════════════════════════════
echo   %YELLOW%阶段 2: 中等文件测试 (100KB)%RESET%
echo ════════════════════════════════════════════════════════════
echo.

if exist "%TESTDATA_DIR%\medium_100k.bin" (
    REM T6: 10 并发, 1000 请求
    echo [%GREEN%T6%RESET%] 中文件 | 并发: 10 | 请求: 1000 | 预期 QPS: ^>500
    ab -n 1000 -c 10 -p "%TESTDATA_DIR%\medium_100k.bin" -T application/octet-stream ^
       -g "%RESULTS_DIR%\t6_medium_10c_1k.tsv" ^
       "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t6_medium_10c_1k.log" 2>&1
    findstr /C:"Requests per second" "%RESULTS_DIR%\t6_medium_10c_1k.log"
    echo.

    REM T7: 50 并发, 2000 请求
    echo [%GREEN%T7%RESET%] 中文件 | 并发: 50 | 请求: 2000 | 预期 QPS: ^>800
    ab -n 2000 -c 50 -p "%TESTDATA_DIR%\medium_100k.bin" -T application/octet-stream ^
       -g "%RESULTS_DIR%\t7_medium_50c_2k.tsv" ^
       "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t7_medium_50c_2k.log" 2>&1
    findstr /C:"Requests per second" "%RESULTS_DIR%\t7_medium_50c_2k.log"
    echo.
) else (
    echo %RED%⚠ 跳过: medium_100k.bin 文件不存在%RESET%
)

echo.
echo ════════════════════════════════════════════════════════════
echo   %YELLOW%阶段 3: 大文件测试 (1MB)%RESET%
echo ════════════════════════════════════════════════════════════
echo.

if exist "%TESTDATA_DIR%\large_1m.bin" (
    REM T8: 5 并发, 500 请求
    echo [%GREEN%T8%RESET%] 大文件 | 并发: 5 | 请求: 500 | 预期 QPS: ^>100
    ab -n 500 -c 5 -p "%TESTDATA_DIR%\large_1m.bin" -T application/octet-stream ^
       -g "%RESULTS_DIR%\t8_large_5c_500.tsv" ^
       "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t8_large_5c_500.log" 2>&1
    findstr /C:"Requests per second" "%RESULTS_DIR%\t8_large_5c_500.log"
    echo.

    REM T9: 10 并发, 1000 请求
    echo [%GREEN%T9%RESET%] 大文件 | 并发: 10 | 请求: 1000 | 预期 QPS: ^>150
    ab -n 1000 -c 10 -p "%TESTDATA_DIR%\large_1m.bin" -T application/octet-stream ^
       -g "%RESULTS_DIR%\t9_large_10c_1k.tsv" ^
       "%PROXY_URL%/test/upload" > "%RESULTS_DIR%\t9_large_10c_1k.log" 2>&1
    findstr /C:"Requests per second" "%RESULTS_DIR%\t9_large_10c_1k.log"
    echo.
) else (
    echo %RED%⚠ 跳过: large_1m.bin 文件不存在%RESET%
)

echo.
echo ════════════════════════════════════════════════════════════
echo   %GREEN%✅ 所有测试完成！%RESET%
echo ════════════════════════════════════════════════════════════
echo.
echo 📁 结果文件保存在: %RESULTS_DIR%
echo.
echo ℹ️ 查看完整日志: type %RESULTS_DIR%\*.log
echo ℹ️ 生成性能报告: 运行 analyze_results.bat
echo.
endlocal