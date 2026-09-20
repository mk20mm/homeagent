package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/task"
)

type stubTaskService struct {
	tasks            []task.Task
	lastCmd          task.AssignTaskCmd
	assignID         string
	assignDuplicated bool
	assigned         bool
	completeCalled   bool
	failComplete     bool
}

func (s *stubTaskService) ListMyTasks(_ context.Context, _ string) ([]task.Task, error) {
	return s.tasks, nil
}

func (s *stubTaskService) AssignTask(_ context.Context, _ string, cmd task.AssignTaskCmd) (string, bool, error) {
	s.assigned = true
	s.lastCmd = cmd
	return s.assignID, s.assignDuplicated, nil
}

func (s *stubTaskService) CompleteTask(_ context.Context, _, _ string) error {
	s.completeCalled = true
	if s.failComplete {
		return apperr.New(apperr.CodeConflict, "任务已完成，勿重复打卡", nil)
	}
	return nil
}

func (s *stubTaskService) GetTask(_ context.Context, taskID string) (task.Task, error) {
	return task.Task{ID: taskID, Title: "洗碗", Risk: "medium", Status: task.StatusDone, Points: 1}, nil
}

type stubMemberNamer struct {
	name string
	fail bool
}

func (n *stubMemberNamer) MemberName(_ context.Context, _ string) (string, error) {
	if n.fail {
		return "", apperr.New(apperr.CodeNotFound, "成员不存在", nil)
	}
	return n.name, nil
}

type stubUndoWriter struct {
	saved map[string]string // toolName → undoData
}

func (w *stubUndoWriter) SaveUndo(_ context.Context, _ string, toolName string, undoData json.RawMessage) (string, error) {
	if w.saved == nil {
		w.saved = map[string]string{}
	}
	w.saved[toolName] = string(undoData)
	return "undo-1", nil
}

type stubPermLookup struct {
	perms map[string]bool
}

func (p *stubPermLookup) Permissions(_ context.Context, _ string) (map[string]bool, error) {
	return p.perms, nil
}

// stubAuditLogger 捕获审计条目（含越权尝试，T17）。
type stubAuditLogger struct {
	entries []tool.AuditEntry
}

func (a *stubAuditLogger) Log(_ context.Context, e tool.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func setupTaskRouter(perms map[string]bool, ts TaskService, uw UndoWriter, namer MemberNamer) (*gin.Engine, *stubAuditLogger) {
	gin.SetMode(gin.TestMode)
	audit := &stubAuditLogger{}
	r := gin.New()
	g := r.Group("/api/v1")
	g.Use(func(c *gin.Context) {
		c.Set("member_id", "member-1")
		c.Next()
	})
	pl := &stubPermLookup{perms: perms}
	g.GET("/tasks", ListTasks(ts, pl, audit))
	g.POST("/tasks", CreateTask(ts, namer, uw, pl, audit))
	g.POST("/tasks/:taskId/complete", CompleteTask(ts, uw, pl, audit))
	return r, audit
}

func TestListTasksFiltersByStatus(t *testing.T) {
	ts := &stubTaskService{tasks: []task.Task{
		{ID: "t1", Title: "洗碗", Status: task.StatusPending},
		{ID: "t2", Title: "扫地", Status: task.StatusInProgress},
	}}
	r, _ := setupTaskRouter(map[string]bool{"task.read": true}, ts, nil, nil)

	w := adminDo(r, http.MethodGet, "/api/v1/tasks?status=in_progress", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var out struct {
		Items []TaskItem `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Items) != 1 || out.Items[0].ID != "t2" {
		t.Fatalf("status 过滤应只剩 in_progress，got %+v", out.Items)
	}
}

func TestListTasksRequiresReadPerm(t *testing.T) {
	ts := &stubTaskService{}
	r, audit := setupTaskRouter(map[string]bool{}, ts, nil, nil) // 无 task.read

	w := adminDo(r, http.MethodGet, "/api/v1/tasks", "")
	if w.Code != 403 {
		t.Fatalf("无权限应 403，got %d", w.Code)
	}
	if ts.assigned {
		t.Error("无权限时不应调用服务")
	}
	// 越权尝试必须留审计（T17，与工具层 registry 同语义）
	if len(audit.entries) != 1 {
		t.Fatalf("应写 1 条越权审计，got %d", len(audit.entries))
	}
	e := audit.entries[0]
	if e.ToolName != "list_my_tasks" || !e.PermissionDenied || e.MemberID != "member-1" {
		t.Fatalf("审计条目不对: %+v", e)
	}
}

func TestCreateTaskWritesUndoAndMapsAssignee(t *testing.T) {
	ts := &stubTaskService{assignID: "task-9"}
	uw := &stubUndoWriter{}
	r, _ := setupTaskRouter(map[string]bool{"task.write": true}, ts, uw, &stubMemberNamer{name: "妈妈"})

	w := adminDo(r, http.MethodPost, "/api/v1/tasks",
		`{"title":"洗碗","assignee_id":"member-mom"}`)
	if w.Code != 201 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	// assignee_id 在边界处被解析成名字，保证与工具同一幂等键
	if ts.lastCmd.AssigneeName != "妈妈" {
		t.Fatalf("AssigneeName 应为「妈妈」，got %q", ts.lastCmd.AssigneeName)
	}
	if ts.lastCmd.Title != "洗碗" {
		t.Fatalf("Title 应为「洗碗」，got %q", ts.lastCmd.Title)
	}

	// 撤销记录写入（toolName 与工具一致，POST /undo 才能撤销）
	data, ok := uw.saved["assign_task"]
	if !ok {
		t.Fatal("未写 assign_task 撤销记录")
	}
	var d struct {
		TaskID string `json:"task_id"`
	}
	json.Unmarshal([]byte(data), &d)
	if d.TaskID != "task-9" {
		t.Fatalf("撤销数据 task_id 应为 task-9，got %q", d.TaskID)
	}

	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["undo_id"] != "undo-1" || out["undoable"] != true {
		t.Fatalf("响应应带撤销信息，got %+v", out)
	}
	if out["id"] != "task-9" {
		t.Fatalf("响应 id 应为 task-9，got %v", out["id"])
	}
}

func TestCreateTaskDuplicatedSkipsUndo(t *testing.T) {
	ts := &stubTaskService{assignID: "task-9", assignDuplicated: true}
	uw := &stubUndoWriter{}
	r, _ := setupTaskRouter(map[string]bool{"task.write": true}, ts, uw, &stubMemberNamer{})

	w := adminDo(r, http.MethodPost, "/api/v1/tasks", `{"title":"洗碗"}`)
	if w.Code != 201 {
		t.Fatalf("幂等命中对用户是成功，got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := uw.saved["assign_task"]; ok {
		t.Fatal("幂等命中不应写撤销记录（会误删早先那个任务）")
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["duplicated"] != true || out["undoable"] != false {
		t.Fatalf("幂等响应应标 duplicated，got %+v", out)
	}
}

func TestCreateTaskRejectsBadAssigneeAndDue(t *testing.T) {
	ts := &stubTaskService{assignID: "task-9"}
	uw := &stubUndoWriter{}

	// 不存在的执行人 → 404
	r, _ := setupTaskRouter(map[string]bool{"task.write": true}, ts, uw, &stubMemberNamer{fail: true})
	w := adminDo(r, http.MethodPost, "/api/v1/tasks", `{"title":"洗碗","assignee_id":"nope"}`)
	if w.Code != 404 {
		t.Fatalf("不存在的执行人应 404，got %d: %s", w.Code, w.Body.String())
	}

	// 格式错的截止时间 → 400
	r2, _ := setupTaskRouter(map[string]bool{"task.write": true}, ts, uw, &stubMemberNamer{name: "妈妈"})
	w = adminDo(r2, http.MethodPost, "/api/v1/tasks", `{"title":"洗碗","due_at":"not-a-date"}`)
	if w.Code != 400 {
		t.Fatalf("非法 due_at 应 400，got %d: %s", w.Code, w.Body.String())
	}

	// 合法 due_at 应被接受
	due := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	w = adminDo(r2, http.MethodPost, "/api/v1/tasks", `{"title":"洗碗","due_at":"`+due+`"}`)
	if w.Code != 201 {
		t.Fatalf("合法 due_at 应 201，got %d: %s", w.Code, w.Body.String())
	}
	if ts.lastCmd.DueAt.IsZero() {
		t.Fatal("due_at 未被解析进 cmd")
	}

	// 空标题 → 400
	w = adminDo(r2, http.MethodPost, "/api/v1/tasks", `{"description":"没标题"}`)
	if w.Code != 400 {
		t.Fatalf("空标题应 400，got %d", w.Code)
	}
}

func TestCompleteTaskWritesUndo(t *testing.T) {
	ts := &stubTaskService{}
	uw := &stubUndoWriter{}
	r, _ := setupTaskRouter(map[string]bool{"task.read": true}, ts, uw, nil)

	w := adminDo(r, http.MethodPost, "/api/v1/tasks/task-9/complete", "")
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if !ts.completeCalled {
		t.Fatal("应调用打卡")
	}
	data, ok := uw.saved["complete_task"]
	if !ok {
		t.Fatal("未写 complete_task 撤销记录")
	}
	if data != `{"task_id":"task-9"}` {
		t.Fatalf("撤销数据应为 task_id=task-9，got %s", data)
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if out["status"] != "done" {
		t.Fatalf("响应应为 done，got %v", out["status"])
	}
}

func TestCompleteTaskConflictBecomes409(t *testing.T) {
	ts := &stubTaskService{failComplete: true}
	uw := &stubUndoWriter{}
	r, _ := setupTaskRouter(map[string]bool{"task.read": true}, ts, uw, nil)

	w := adminDo(r, http.MethodPost, "/api/v1/tasks/task-9/complete", "")
	if w.Code != 409 {
		t.Fatalf("重复打卡应 409，got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := uw.saved["complete_task"]; ok {
		t.Fatal("冲突时不应写撤销记录")
	}
}
