# 1. 监控内存 (Heap)
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/heap

# 2. 监控 CPU
采集30s的cpu数据
go tool pprof -http=:8083 "http://localhost:6060/debug/pprof/profile?seconds=30"