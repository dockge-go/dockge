// Package server 定义可被 app 容器统一管理的服务生命周期接口。
package server

import "context"

// Server 是可被 app 容器统一编排的服务生命周期接口。
type Server interface {
	Start(context.Context) error
	Stop(context.Context) error
}
