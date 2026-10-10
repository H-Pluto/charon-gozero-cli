# go-zero 用 goreman 同时拉起多个服务。
# 用法：goreman start
#
# 两行对应两个进程；日志分别带 api / rpc 前缀，便于区分。

api: go run ./cmd/api -f etc/charon-api.yaml
rpc: go run ./cmd/rpc -f etc/charon-rpc.yaml
