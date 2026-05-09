package main

import (
	"evalux-server/internal/config"
	"evalux-server/internal/db"
	"evalux-server/internal/router"
	"evalux-server/internal/storage"
	"log"
	"net/http"
	"time"
)

func main() {
	cfg := config.Load()

	client, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("数据库连接/迁移失败: %v", err)
	}
	defer client.Close()

	if err := db.SeedBaseData(client); err != nil {
		log.Fatalf("基础数据初始化失败: %v", err)
	}

	if err := db.SeedAdminUser(client, cfg.AdminAccount, cfg.AdminPassword, cfg.AdminNickname); err != nil {
		log.Fatalf("管理员用户初始化失败: %v", err)
	}

	// 初始化 Cloudreve 对象存储
	cloudreveStorage, err := storage.NewCloudreveStorage(cfg)
	if err != nil {
		log.Printf("Cloudreve 初始化失败（截图/录屏上传功能不可用）: %v", err)
		// 不阻塞启动，Cloudreve 不可用时只影响文件上传功能
	} else {
		log.Println("Cloudreve 对象存储初始化成功")
	}

	r := router.Setup(client, cfg, cloudreveStorage)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Minute,  // 接收请求体（含截图上传）最长 5 分钟
		WriteTimeout: 15 * time.Minute, // 写响应（含 AI 推理等待）最长 15 分钟
		IdleTimeout:  2 * time.Minute,
	}

	log.Printf("服务启动在 :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
