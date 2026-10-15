// projects_test.go：项目 CRUD、计数聚合与删除置空关联的测试（v2.0 R1）。
package store

import (
	"context"
	"errors"
	"testing"
)

// mustAddTodo 是 TodoInput 的便捷构造（默认仅标题）。
func mustAddTodo(t *testing.T, db *DB, title string, projectID *int64) Todo {
	t.Helper()
	todo, err := db.AddTodo(context.Background(), TodoInput{
		Title:     title,
		ProjectID: projectID,
	})
	if err != nil {
		t.Fatalf("新增待办 %q: %v", title, err)
	}
	return todo
}

func int64Ptr(v int64) *int64 { return &v }

func TestProjectCRUDAndCounts(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	// 新增：空颜色缺省 slate、sort_order 递增。
	p1, err := db.AddProject(ctx, " WorkDesktop 开发 ", "")
	if err != nil {
		t.Fatalf("新增项目: %v", err)
	}
	if p1.Name != "WorkDesktop 开发" || p1.Color != ColorSlate || p1.SortOrder != 1 {
		t.Errorf("新增结果 = %+v", p1)
	}
	p2, err := db.AddProject(ctx, "跨境电商选品", ColorAmber)
	if err != nil {
		t.Fatalf("新增项目2: %v", err)
	}
	if p2.SortOrder != 2 {
		t.Errorf("sort_order 应递增, 实际 %d", p2.SortOrder)
	}

	// 非法颜色拒绝。
	if _, err := db.AddProject(ctx, "x", "blue"); err == nil {
		t.Error("非法颜色应被拒绝")
	}
	if _, err := db.AddProject(ctx, "   ", ""); err == nil {
		t.Error("空名称应被拒绝")
	}

	// 计数：p1 挂 2 条（1 未完成 1 已完成），p2 挂 1 条未完成。
	t1 := mustAddTodo(t, db, "任务A", int64Ptr(p1.ID))
	mustAddTodo(t, db, "任务B", int64Ptr(p1.ID))
	mustAddTodo(t, db, "任务C", int64Ptr(p2.ID))
	if _, err := db.SetTodoDone(ctx, t1.ID, true); err != nil {
		t.Fatalf("完成任务A: %v", err)
	}

	got, err := db.GetProject(ctx, p1.ID)
	if err != nil {
		t.Fatalf("查询项目: %v", err)
	}
	if got.TotalTodos != 2 || got.OpenTodos != 1 {
		t.Errorf("聚合计数 = %+v, 期望 total 2 open 1", got)
	}
	projects, err := db.ListProjects(ctx)
	if err != nil {
		t.Fatalf("列出项目: %v", err)
	}
	if len(projects) != 2 || projects[0].ID != p1.ID || projects[1].ID != p2.ID {
		t.Errorf("列表 = %+v", projects)
	}

	// 更新。
	up, err := db.UpdateProject(ctx, p2.ID, "新名称", ColorPurple, 0)
	if err != nil {
		t.Fatalf("更新项目: %v", err)
	}
	if up.Name != "新名称" || up.Color != ColorPurple || up.SortOrder != 0 {
		t.Errorf("更新结果 = %+v", up)
	}
	if _, err := db.UpdateProject(ctx, p2.ID, "", ColorGreen, 0); err == nil {
		t.Error("更新空名称应被拒绝")
	}
	if _, err := db.UpdateProject(ctx, 99999, "x", ColorGreen, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("更新不存在项目应 ErrNotFound, 实际 %v", err)
	}

	// 删除置空关联（PRD v2.0 验收 4）：待办保留、project_id 置 NULL。
	if err := db.DeleteProject(ctx, p1.ID); err != nil {
		t.Fatalf("删除项目: %v", err)
	}
	left, err := db.ListTodos(ctx, TodoFilter{})
	if err != nil {
		t.Fatalf("删除后列待办: %v", err)
	}
	if len(left) != 3 {
		t.Fatalf("删除项目后待办应全部保留, 实际 %d 条", len(left))
	}
	// 只验证 p1 的两条被置 NULL；p2 的「任务C」保留关联。
	for _, td := range left {
		if td.Title == "任务A" || td.Title == "任务B" {
			if td.ProjectID != nil {
				t.Errorf("待办 %q 的 project_id 应为 NULL, 实际 %d", td.Title, *td.ProjectID)
			}
		}
		if td.Title == "任务C" && (td.ProjectID == nil || *td.ProjectID != p2.ID) {
			t.Errorf("待办「任务C」应保留 p2 关联, 实际 %+v", td.ProjectID)
		}
	}
	if _, err := db.GetProject(ctx, p1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("读已删除项目应 ErrNotFound, 实际 %v", err)
	}
}
