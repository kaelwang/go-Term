#!/usr/bin/env bash
# start.sh — 便捷启动脚本（等价于 deploy/go-Term.service 的手动启动方式）
# 用法: ./start.sh   （可选：同目录放置 go-Term.env 提供环境变量）
./go-term -port 5173 --auth --vault-key Eversec@sdjn506  --bootstrap-admin-user admin --bootstrap-admin-pass Eversec@sdjn506
