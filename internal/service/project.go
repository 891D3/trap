// project.go 定义项目 typed IPC 服务与变更事件（v2.0 新增）。
//
// 与 TodoService 同构：store 注入 + ready 守卫 + 变更广播。项目删除后其
// 关联待办 project_id 由 store 层置 NULL，但前端已有的待办数据需要感知此
// 变化——前端收到 project.deleted 后按约定重新拉取全部待办（见 web/src
// realtime 模块），避免逐条广播 todo.changed。
package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

// ProjectChangeAction 表示项目变更类型。
type ProjectChangeAction string

const (
	ProjectAdded   ProjectChangeAction = "added"
	ProjectUpdated ProjectChangeAction = "updated"
	ProjectDeleted ProjectChangeAction = "deleted"
)

// ProjectChangedPayload 是 project.changed 事件载荷。
type ProjectChangedPayload struct {
	Action    ProjectChangeAction `json:"action"`
	Project   *store.Project      `json:"project,omitempty"`
	ProjectID int64               `json:"projectId,omitempty"`
}

// ProjectChanged 在项目成功写入后广播到所有 WebView。
var ProjectChanged = mygo.NewEvent[ProjectChangedPayload]("project.changed")

// ProjectService 暴露项目 CRUD typed IPC。
type ProjectService struct {
	store *store.DB
}

// AttachProjectStore 注入数据层；由 main 在 App.WhenReady 中调用。
// 包级函数，不会被 mygo 绑定为 IPC 方法。
func AttachProjectStore(service *ProjectService, db *store.DB) {
	service.setStore(db)
}

func (s *ProjectService) setStore(db *store.DB) { s.store = db }

func (s *ProjectService) ready() (*store.DB, error) {
	if s.store == nil {
		return nil, errors.New("service: 项目数据层尚未初始化（应用尚未完成启动）")
	}
	return s.store, nil
}

// Add 新增项目（name 必填；color 为前端色令牌名，缺省主色 slate）。
func (s *ProjectService) Add(ctx context.Context, name, color string) (store.Project, error) {
	db, err := s.ready()
	if err != nil {
		return store.Project{}, err
	}
	project, err := db.AddProject(ctx, strings.TrimSpace(name), color)
	if err != nil {
		return store.Project{}, err
	}
	s.broadcast(ProjectChangedPayload{Action: ProjectAdded, Project: &project, ProjectID: project.ID})
	return project, nil
}

// List 列出全部项目（携带待办计数）。
func (s *ProjectService) List(ctx context.Context) ([]store.Project, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	return db.ListProjects(ctx)
}

// Update 更新项目（名称/颜色/排序）。
func (s *ProjectService) Update(ctx context.Context, id int64, name, color string, sortOrder int) (store.Project, error) {
	db, err := s.ready()
	if err != nil {
		return store.Project{}, err
	}
	project, err := db.UpdateProject(ctx, id, strings.TrimSpace(name), color, sortOrder)
	if err != nil {
		return store.Project{}, err
	}
	s.broadcast(ProjectChangedPayload{Action: ProjectUpdated, Project: &project, ProjectID: project.ID})
	return project, nil
}

// Delete 删除项目并广播 deleted；关联待办的关联由 store 层置 NULL。
func (s *ProjectService) Delete(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	if err := db.DeleteProject(ctx, id); err != nil {
		return err
	}
	s.broadcast(ProjectChangedPayload{Action: ProjectDeleted, ProjectID: id})
	return nil
}

// broadcast 记录广播失败但不回滚已落库的数据；前端可通过 List 重新同步。
func (s *ProjectService) broadcast(payload ProjectChangedPayload) {
	if err := ProjectChanged.Broadcast(payload); err != nil {
		log.Printf("service: 广播 project.changed 失败（数据已落库）: %v", err)
	}
}
