package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"evalux-server/ent"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

// Connect 创建 ent 客户端并连接到 PostgreSQL
func Connect(databaseURL string) (*ent.Client, error) {
	client, err := ent.Open(dialect.Postgres, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("创建 ent 客户端失败: %w", err)
	}

	// 预迁移修复：将旧表中 permission_codes 为 NULL 的行填充为空 JSON 数组
	// ent 重新生成后该列为 NOT NULL，迁移前需消除 NULL 值
	if err := preMigrateFix(databaseURL); err != nil {
		log.Printf("预迁移修复警告（忽略）: %v", err)
	}

	// 自动迁移（创建/更新表结构）
	if err := client.Schema.Create(context.Background()); err != nil {
		client.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	log.Println("数据库迁移完成（ent auto-migration）")

	return client, nil
}

// preMigrateFix 在 ent 自动迁移前修复历史 NULL 数据
func preMigrateFix(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	fixes := []string{
		// sys_role：如果 permission_codes 列不存在则先加上（兼容旧库），再填充 NULL
		`ALTER TABLE sys_role ADD COLUMN IF NOT EXISTS permission_codes jsonb`,
		`UPDATE sys_role SET permission_codes = '[]'::jsonb WHERE permission_codes IS NULL`,
		// ux_org_role / ux_project_role 同理
		`ALTER TABLE ux_org_role ADD COLUMN IF NOT EXISTS permission_codes jsonb`,
		`UPDATE ux_org_role SET permission_codes = '[]'::jsonb WHERE permission_codes IS NULL`,
		`ALTER TABLE ux_project_role ADD COLUMN IF NOT EXISTS permission_codes jsonb`,
		`UPDATE ux_project_role SET permission_codes = '[]'::jsonb WHERE permission_codes IS NULL`,
	}

	for _, fix := range fixes {
		if _, err := db.Exec(fix); err != nil {
			// 表不存在时忽略（首次部署，ent 会建表）
			log.Printf("preMigrateFix skip: %v", err)
		}
	}
	return nil
}

