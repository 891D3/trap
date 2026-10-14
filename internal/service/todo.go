// todo.go 定义待办 typed IPC 服务与变更事件。
//
// 服务层只做 IPC 边界、参数校验与事件广播；SQL 和持久化细节全部复用
// internal/store，避免前端或协议层绕过数据访问边界。
package service

import (
	"context"
	"errors"
	"log"

	"github.com/891D3/trap/internal/store"
	"github.com/egoist/mygo"
)

// TodoChangeAction 表示待办变更类型。
type TodoChangeAction string

const (
	TodoAdded   TodoChangeAction = "added"
	TodoUpdated TodoChangeAction = "updated"
	TodoDeleted TodoChangeAction = "deleted"
)

// TodoChangedPayload 是 todo.changed 事件载荷。
// 删除时 Todo 为空，由 TodoID 标识受影响记录；其他动作携带完整 Todo。
type TodoChangedPayload struct {
	Action TodoChangeAction `json:"action"`
	Todo   *store.Todo      `json:"todo,omitempty"`
	TodoID int64            `json:"todoId,omitempty"`
}

// TodoChanged 在待办成功写入后广播到所有 WebView。
var TodoChanged = mygo.NewEvent[TodoChangedPayload]("todo.changed")

// TodoService 暴露待办 CRUD typed IPC。
//
// store 通过包级 AttachTodoStore 注入，避免把内部数据库句柄作为导出方法
// 暴露给前端（同 T3.1 的 AttachStore 设计）。
type TodoService struct {
	store *store.DB
}

// AttachTodoStore 在 main 的 App.WhenReady 中注入与 AppService 相同的数据库。
// 这是包级函数，不会被 mygo 自动绑定为 IPC 方法。
func AttachTodoStore(service *TodoService, db *store.DB) {
	service.setStore(db)
}

// setStore 是 TodoService 的内部注入方法。
func (s *TodoService) setStore(db *store.DB) {
	s.store = db
}

// ready 返回已注入的数据层。
func (s *TodoService) ready() (*store.DB, error) {
	if s.store == nil {
		return nil, errors.New("service: 待办数据层尚未初始化（应用尚未完成启动）")
	}
	return s.store, nil
}

// Add 新增待办并广播 added 事件。
func (s *TodoService) Add(ctx context.Context, title string) (store.Todo, error) {
	db, err := s.ready()
	if err != nil {
		return store.Todo{}, err
	}
	todo, err := db.AddTodo(ctx, title)
	if err != nil {
		return store.Todo{}, err
	}
	s.broadcast(TodoChangedPayload{Action: TodoAdded, Todo: &todo, TodoID: todo.ID})
	return todo, nil
}

// List 列出全部待办。
func (s *TodoService) List(ctx context.Context) ([]store.Todo, error) {
	db, err := s.ready()
	if err != nil {
		return nil, err
	}
	return db.ListTodos(ctx)
}

// SetDone 设置完成/未完成状态并广播 updated 事件。
func (s *TodoService) SetDone(ctx context.Context, id int64, done bool) (store.Todo, error) {
	db, err := s.ready()
	if err != nil {
		return store.Todo{}, err
	}
	todo, err := db.SetTodoDone(ctx, id, done)
	if err != nil {
		return store.Todo{}, err
	}
	s.broadcast(TodoChangedPayload{Action: TodoUpdated, Todo: &todo, TodoID: todo.ID})
	return todo, nil
}

// Delete 删除待办并广播 deleted 事件。
func (s *TodoService) Delete(ctx context.Context, id int64) error {
	db, err := s.ready()
	if err != nil {
		return err
	}
	if err := db.DeleteTodo(ctx, id); err != nil {
		return err
	}
	s.broadcast(TodoChangedPayload{Action: TodoDeleted, TodoID: id})
	return nil
}

// broadcast 记录广播失败但不回滚已经落库的数据；前端可通过 List 重新同步。
func (s *TodoService) broadcast(payload TodoChangedPayload) {
	if err := TodoChanged.Broadcast(payload); err != nil {
		log.Printf("service: 广播 todo.changed 失败（数据已落库）: %v", err)
	}
}
