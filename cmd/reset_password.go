package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"dockge/internal/repository"
	"dockge/internal/service"
	"dockge/pkg/log"

	"github.com/samber/do/v2"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
)

// runResetPassword 交互式重置指定用户的密码（遗忘密码时的唯一恢复入口）。
// 复用 repository 层，避免命令行工具绕过数据访问层直连 bucket。
func runResetPassword(conf *viper.Viper) {
	injector := do.New(
		func(i do.Injector) { do.ProvideValue(i, conf) },
		log.Package,
		repository.Package,
	)
	defer func() { _ = injector.Shutdown() }()
	// 数据库被运行中的服务占用时给出明确指引（单机约束：一次只允许一个进程）
	repo, err := do.Invoke[*repository.Repository](injector)
	if err != nil {
		fmt.Printf("无法打开数据库：%v\n提示：请先停止 dockge 服务，再执行 reset-password。\n", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter username to reset: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)
	if username == "" {
		fmt.Println("Error: username is required")
		os.Exit(1)
	}

	ctx := context.Background()
	user, err := repo.GetUserByUsername(ctx, username)
	if err != nil {
		fmt.Printf("User '%s' not found\n", username)
		os.Exit(1)
	}

	var newPassword string
	for {
		fmt.Print("Enter new password: ")
		pw, _ := reader.ReadString('\n')
		fmt.Print("Confirm new password: ")
		pw2, _ := reader.ReadString('\n')
		pw, pw2 = strings.TrimSpace(pw), strings.TrimSpace(pw2)
		if pw == pw2 && service.ValidatePasswordStrength(pw) {
			newPassword = pw
			break
		}
		fmt.Println("Passwords do not match, or too weak (min 6 chars, must contain letters and digits)")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	if err := repo.UpdatePassword(ctx, user.ID, string(hashed)); err != nil {
		panic(err)
	}
	fmt.Printf("Password for '%s' has been reset successfully\n", username)
}
