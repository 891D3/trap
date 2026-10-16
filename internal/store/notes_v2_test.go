// notes_v2_test.go：信息记录 v2 store 层测试（PRD v2.1 §3.5 验收 2/6）。
//
// sweep 边界需要回填 updated_at/archived_at——service 层无该入口，故在
// store 包内直连事务（同 migrate/crud 测试手法）。
package store

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// backfillNote 用直连事务回填 updated_at（测试专用；等价数据迁移语义）。
func backfillNote(t *testing.T, db *DB, id int64, updated time.Time) {
	t.Helper()
	_, err := db.conn.Transaction(context.Background(), []sqlite.Statement{
		{SQL: "UPDATE notes SET updated_at = ? WHERE id = ?", Args: []any{updated.UTC().Format(timeRFC3339), id}},
	})
	if err != nil {
		t.Fatalf("回填 updated_at: %v", err)
	}
}

func backfillArchivedAt(t *testing.T, db *DB, id int64, at time.Time) {
	t.Helper()
	_, err := db.conn.Transaction(context.Background(), []sqlite.Statement{
		{SQL: "UPDATE notes SET archived_at = ?, archived = 1 WHERE id = ?", Args: []any{at.UTC().Format(timeRFC3339), id}},
	})
	if err != nil {
		t.Fatalf("回填 archived_at: %v", err)
	}
}

func TestSweepSparksBoundaries(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	mk := func(title string, updated time.Time) int64 {
		t.Helper()
		sp, err := db.CreateNoteV2(ctx, NoteKindSpark, title, title, nil)
		if err != nil {
			t.Fatalf("新建灵感: %v", err)
		}
		backfillNote(t, db, sp.ID, updated)
		return sp.ID
	}

	fresh := mk("新鲜29天", now.AddDate(0, 0, -29))
	exact := mk("刚好30天", now.AddDate(0, 0, -30))
	over := mk("超30天", now.AddDate(0, 0, -31))

	// 归档 90+ 天（→ 清除）与归档 30 天（→ 保留）与归档 89 天（→ 保留）。
	oldPurge := mk("归档91天", now.AddDate(0, 0, -SparkAutoPurgeDays-1))
	oldKeep := mk("归档89天", now.AddDate(0, 0, -1))
	backfillArchivedAt(t, db, oldPurge, now.AddDate(0, 0, -SparkAutoPurgeDays-1))
	backfillArchivedAt(t, db, oldKeep, now.AddDate(0, 0, -1))

	archived, purged, err := db.SweepSparks(ctx, now)
	if err != nil {
		t.Fatalf("SweepSparks: %v", err)
	}
	if archived != 2 {
		t.Errorf("自动归档数 = %d, 期望 2（30 天整与 31 天；29 天不触发）", archived)
	}
	if purged != 1 {
		t.Errorf("清除数 = %d, 期望 1（91 天前归档）", purged)
	}

	// 结果核对：保留者状态正确、被清除者消失。
	rows, err := db.ListNotesV2(ctx, NoteFilterV2{Kind: NoteKindSpark, IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	states := map[int64]bool{}
	for _, n := range rows {
		states[n.ID] = n.Archived
	}
	if got, ok := states[fresh]; !ok || got {
		t.Errorf("29 天灵感应保留且未归档（存在=%v 归档=%v）", ok, got)
	}
	if got, ok := states[exact]; !ok || !got {
		t.Errorf("30 天整灵感应已自动归档")
	}
	if got, ok := states[over]; !ok || !got {
		t.Errorf("31 天灵感应已自动归档")
	}
	if got, ok := states[oldKeep]; !ok || !got {
		t.Errorf("归档 1 天的灵感应保留")
	}
	if _, ok := states[oldPurge]; ok {
		t.Errorf("91 天前归档的灵感应被清除")
	}
}

func TestNoteV2CRUDAndFilters(t *testing.T) {
	db := openTempFile(t)
	ctx := context.Background()

	proj, err := db.AddProject(ctx, "项目A", ColorSlate)
	if err != nil {
		t.Fatal(err)
	}

	// 三种种子：笔记无项目 / 笔记带项目 / 灵感带项目 / 灵感归档。
	n1, err := db.CreateNoteV2(ctx, NoteKindNote, "笔记甲", "内容", nil)
	if err != nil {
		t.Fatal(err)
	}
	n2id := int64(0)
	{
		n2, err := db.CreateNoteV2(ctx, NoteKindNote, "笔记乙", "内容", &proj.ID)
		if err != nil {
			t.Fatal(err)
		}
		n2id = n2.ID
	}
	spId := int64(0)
	{
		sp, err := db.CreateNoteV2(ctx, NoteKindSpark, "灵感甲", "内容", &proj.ID)
		if err != nil {
			t.Fatal(err)
		}
		spId = sp.ID
	}
	sp2, err := db.CreateNoteV2(ctx, NoteKindSpark, "灵感乙", "内容", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ArchiveNote(ctx, sp2.ID); err != nil {
		t.Fatal(err)
	}

	// kind 过滤。
	sparks, err := db.ListNotesV2(ctx, NoteFilterV2{Kind: NoteKindSpark, IncludeArchived: true})
	if err != nil || len(sparks) != 2 {
		t.Fatalf("spark 过滤 = %+v err=%v, 期望 2", sparks, err)
	}
	// 项目过滤。
	byProj, err := db.ListNotesV2(ctx, NoteFilterV2{HasProject: true, ProjectID: &proj.ID})
	if err != nil || len(byProj) != 2 {
		t.Fatalf("项目过滤 = %+v err=%v, 期望 2（笔记乙+灵感甲）", byProj, err)
	}
	// 未处理灵感（不含归档）。
	open, err := db.ListNotesV2(ctx, NoteFilterV2{Kind: NoteKindSpark})
	if err != nil || len(open) != 1 || open[0].ID != spId {
		t.Fatalf("未处理灵感过滤 = %+v err=%v, 期望仅灵感甲", open, err)
	}
	// SaveNoteV2 更新项目关联。
	if _, err := db.SaveNoteV2(ctx, n1.ID, "笔记甲", "内容", &proj.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetNoteV2(ctx, n1.ID)
	if got.ProjectID == nil || *got.ProjectID != proj.ID {
		t.Errorf("SaveNoteV2 未更新项目关联: %+v", got)
	}
	// GetNote（v1）读 v2 数据不缺列。
	if _, err := db.GetNote(ctx, n2id); err != nil {
		t.Fatalf("v1 GetNote 读 v2 行失败: %v", err)
	}
	// PromoteSparkToNote：保留原灵感 + 关联沿用。
	np, err := db.PromoteSparkToNote(ctx, spId, "灵感甲", "> 内容", nil)
	if err != nil {
		t.Fatal(err)
	}
	if np.Kind != NoteKindNote || np.ProjectID == nil {
		t.Errorf("转笔记结果 = %+v 判断异常", np)
	}
	sparksAfter, _ := db.ListNotesV2(ctx, NoteFilterV2{Kind: NoteKindSpark, IncludeArchived: false})
	if len(sparksAfter) != 1 {
		t.Errorf("转笔记后原灵感应保留（未处理仍 1 条）")
	}
	// 非法 kind 拒绝。
	if _, err := db.CreateNoteV2(ctx, "diary", "x", "", nil); err == nil {
		t.Error("非法 kind 应被拒绝")
	}
	// DeleteProject 联动 notes.project_id 置空。
	if err := db.DeleteProject(ctx, proj.ID); err != nil {
		t.Fatal(err)
	}
	check, _ := db.GetNoteV2(ctx, n1.ID)
	if check.ProjectID != nil {
		t.Errorf("删除项目后笔记关联应为 NULL: %+v", check)
	}
}
