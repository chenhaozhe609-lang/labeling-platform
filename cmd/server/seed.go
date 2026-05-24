package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/config"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/db"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// runSeed 生成 demo 数据：在 source-db 建一张源表并授权，在 meta-db 建 dataset + 任务。
// 用于在 C2（上传/恢复）就绪前打通标注端到端。
func runSeed(cfg config.Config) {
	ctx := context.Background()

	adminURL := os.Getenv("SOURCE_ADMIN_URL")
	if adminURL == "" {
		adminURL = "postgres://postgres:postgres@localhost:5433/sandbox_template?sslmode=disable"
	}
	spool, err := db.NewPool(ctx, adminURL)
	if err != nil {
		fatal("连接 source-db 失败", err)
	}
	defer spool.Close()
	for _, stmt := range sourceSeedStmts {
		if _, err := spool.Exec(ctx, stmt); err != nil {
			fatal("source seed 失败", err)
		}
	}

	mpool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("连接 meta-db 失败", err)
	}
	defer mpool.Close()
	st := store.New(mpool)

	id, err := st.CreateDataset(ctx, &domain.Dataset{
		Name:              "高考专业库（demo）",
		SourceSchema:      "ds_demo",
		SourceTable:       "majors",
		SourcePKColumn:    "id",
		FormSchema:        json.RawMessage(demoFormSchema),
		FormSchemaVersion: 3,
		Status:            domain.StatusReady,
		TotalRows:         4,
	})
	if err != nil {
		fatal("创建 dataset 失败", err)
	}
	n, err := st.CreateTasks(ctx, id, []string{"1", "2", "3", "4"})
	if err != nil {
		fatal("生成 tasks 失败", err)
	}
	fmt.Printf("seed 完成：dataset #%d «高考专业库（demo）»，生成 %d 个任务\n", id, n)
}

var sourceSeedStmts = []string{
	`CREATE SCHEMA IF NOT EXISTS ds_demo`,
	`DROP TABLE IF EXISTS ds_demo.majors`,
	`CREATE TABLE ds_demo.majors (id INT PRIMARY KEY, title TEXT, body TEXT, category TEXT, discipline VARCHAR(16), difficulty SMALLINT)`,
	`INSERT INTO ds_demo.majors (id, title, body, category) VALUES
	 (1, '计算机科学与技术', '本专业培养具备扎实的数学与计算机科学理论基础，系统掌握计算机硬件、软件与网络技术，能够在科研院所、企事业单位从事计算机教学、科学研究与应用开发的高级专门人才。主干课程包括数据结构、操作系统、计算机组成原理、编译原理、计算机网络、数据库系统与人工智能导论等。', '工学'),
	 (2, '哲学', '哲学专业旨在培养具有较高理论素养与思辨能力、系统掌握中外哲学史与哲学基本原理的研究型人才。课程涵盖马克思主义哲学、中国哲学史、西方哲学史、伦理学、逻辑学与科学技术哲学等，注重经典文本的精读与批判性思维训练。', '哲学'),
	 (3, '生物医学工程', '生物医学工程是综合生命科学、医学与工程技术的交叉学科，培养既懂医学又掌握工程方法、能够从事医疗仪器研发、医学影像处理与生物信息分析的复合型人才。主干课程包括电子技术、信号与系统、医学影像、生物力学与医学仪器设计。', '工学'),
	 (4, '会计学', '会计学专业培养具备管理、经济、法律和会计学知识与能力，能在企事业单位、会计师事务所及政府部门从事会计实务、审计与财务管理工作的应用型专门人才，强调实务操作与职业资格衔接。', '管理学')`,
	`GRANT USAGE ON SCHEMA ds_demo TO labeling_reader`,
	`GRANT SELECT ON ALL TABLES IN SCHEMA ds_demo TO labeling_reader`,
}

// v2 列角色 form_schema（PRD §24）：title/body/category 作上下文（兼 LLM 预填依据），
// discipline/difficulty 作补全列（源表中为 NULL → 整表都是待补全任务）。
const demoFormSchema = `{
  "version": 1,
  "primary_cols": ["title"],
  "columns": [
    { "code": "id", "type": "integer", "role": "id", "label": "ID", "pk": true },
    { "code": "title", "type": "text", "role": "context", "label": "专业名称" },
    { "code": "body", "type": "text", "role": "context", "label": "专业介绍" },
    { "code": "category", "type": "text", "role": "context", "label": "学科门类" },
    { "code": "discipline", "type": "varchar(16)", "role": "fill", "label": "学科归类",
      "field": { "kind": "single", "options": [
        { "value": "theory", "label": "理论型", "key": "Q" },
        { "value": "engineering", "label": "工程型", "key": "W" },
        { "value": "interdisciplinary", "label": "交叉型", "key": "E" },
        { "value": "applied", "label": "应用型", "key": "R" }
      ] } },
    { "code": "difficulty", "type": "smallint", "role": "fill", "label": "学习难度",
      "field": { "kind": "number", "min": 1, "max": 5 } }
  ]
}`
