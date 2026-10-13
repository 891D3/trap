// Package service 定义经 mygo typed IPC 暴露给前端的服务。
//
// 约定（PRD §2/§5）：前端不直接访问 SQLite，所有数据操作经这些服务的
// 方法完成；mygo generate 据此生成 TS 客户端，两端类型同步演进。
package service

// AppService 提供应用级基础能力（T1 占位，T2/T3 充实设置与主题能力）。
type AppService struct{}

// Ping 连通性自检：验证 mygo typed IPC 端到端链路。
func (AppService) Ping() string { return "pong" }