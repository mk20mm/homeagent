package repo

import (
	"context"
	"fmt"

	"github.com/mk20mm/homeagent/internal/apperr"
	domrun "github.com/mk20mm/homeagent/internal/domain/run"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/agentrun"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/runevent"
	"github.com/mk20mm/homeagent/internal/store/ent/runstep"
	"github.com/mk20mm/homeagent/internal/store/ent/schema"
)

// 编译期确保 Store 实现了 domrun.Repo 接口。
var _ domrun.Repo = (*Store)(nil)

func toRunStepItem(s *ent.RunStep) domrun.RunStepItem {
	return domrun.RunStepItem{
		ID:              s.ID.String(),
		Position:        s.Position,
		ToolName:        s.ToolName,
		Status:          string(s.Status),
		WaitFor:         string(s.WaitFor),
		CompletionKind:  s.CompletionKind,
		OperationKey:    s.OperationKey,
		ResultCard:      s.ResultCard,
		ExpectedVersion: s.ExpectedVersion,
	}
}

func toRunEventItem(e *ent.RunEvent) domrun.RunEventItem {
	return domrun.RunEventItem{
		ID:        e.ID.String(),
		Seq:       e.Seq,
		StepID:    e.StepID,
		EventType: e.EventType,
		Payload:   e.Payload,
		CreatedAt: e.CreatedAt,
	}
}

func toRunItem(r *ent.AgentRun) domrun.RunItem {
	return domrun.RunItem{
		ID:         r.ID.String(),
		Goal:       r.Goal,
		IntentKind: string(r.IntentKind),
		Status:     string(r.Status),
		Version:    r.Version,
		RequestID:  r.RequestID,
		CreatedAt:  r.CreatedAt,
	}
}

func toRunDetail(r *ent.AgentRun) domrun.RunDetail {
	d := domrun.RunDetail{
		ID:         r.ID.String(),
		Goal:       r.Goal,
		IntentKind: string(r.IntentKind),
		Status:     string(r.Status),
		Version:    r.Version,
		RequestID:  r.RequestID,
		CreatedAt:  r.CreatedAt,
		Steps:      make([]domrun.RunStepItem, 0, len(r.Edges.Steps)),
		Events:     make([]domrun.RunEventItem, 0, len(r.Edges.Events)),
	}
	for _, s := range r.Edges.Steps {
		d.Steps = append(d.Steps, toRunStepItem(s))
	}
	for _, e := range r.Edges.Events {
		d.Events = append(d.Events, toRunEventItem(e))
	}
	return d
}

// CreateRun 创建事务编排运行记录。
func (s *Store) CreateRun(ctx context.Context, memberID string, goal string, intentKind schema.IntentKind, requestID string, conversationID string, requestedEffects []string) (domrun.RunDetail, error) {
	memUUID := toUUID(memberID)
	m, err := s.db.Member.Query().Where(member.IDEQ(memUUID)).WithFamily().Only(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeNotFound, "成员不存在", err)
	}

	// 幂等校验：同一 request_id 重复提交返回已有记录
	if requestID != "" {
		existing, err := s.db.AgentRun.Query().
			Where(agentrun.RequestIDEQ(requestID), agentrun.HasMemberWith(member.IDEQ(memUUID))).
			WithSteps(func(q *ent.RunStepQuery) { q.Order(ent.Asc(runstep.FieldPosition)) }).
			WithEvents(func(q *ent.RunEventQuery) { q.Order(ent.Asc(runevent.FieldSeq)) }).
			First(ctx)
		if err == nil && existing != nil {
			return toRunDetail(existing), nil
		}
	}

	b := s.db.AgentRun.Create().
		SetGoal(goal).
		SetIntentKind(agentrun.IntentKind(intentKind)).
		SetStatus(agentrun.StatusRunning).
		SetVersion(1).
		SetMember(m).
		SetFamily(m.Edges.Family)

	if requestID != "" {
		b.SetRequestID(requestID)
	}
	if conversationID != "" {
		b.SetConversationID(toUUID(conversationID))
	}
	if len(requestedEffects) > 0 {
		b.SetRequestedEffects(requestedEffects)
	}

	saved, err := b.Save(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeInternal, "创建任务编排失败", err)
	}

	// 记录初始事件
	_, _ = s.RecordRunEvent(ctx, saved.ID.String(), "", "run_created", fmt.Sprintf(`{"goal":%q}`, goal))
	return s.GetRun(ctx, memberID, saved.ID.String())
}

// GetRun 获取运行详情与完整步骤树、事件链。
func (s *Store) GetRun(ctx context.Context, memberID string, runID string) (domrun.RunDetail, error) {
	runUUID := toUUID(runID)
	memUUID := toUUID(memberID)

	r, err := s.db.AgentRun.Query().
		Where(agentrun.IDEQ(runUUID), agentrun.HasMemberWith(member.IDEQ(memUUID))).
		WithSteps(func(q *ent.RunStepQuery) { q.Order(ent.Asc(runstep.FieldPosition)) }).
		WithEvents(func(q *ent.RunEventQuery) { q.Order(ent.Asc(runevent.FieldSeq)) }).
		Only(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeNotFound, "任务编排不存在或无权访问", err)
	}
	return toRunDetail(r), nil
}

func (s *Store) getRunByID(ctx context.Context, runID string) (domrun.RunDetail, error) {
	runUUID := toUUID(runID)
	r, err := s.db.AgentRun.Query().
		Where(agentrun.IDEQ(runUUID)).
		WithSteps(func(q *ent.RunStepQuery) { q.Order(ent.Asc(runstep.FieldPosition)) }).
		WithEvents(func(q *ent.RunEventQuery) { q.Order(ent.Asc(runevent.FieldSeq)) }).
		Only(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeNotFound, "任务编排不存在", err)
	}
	return toRunDetail(r), nil
}

// ListRuns 列出当前成员获授权的编排列表。
func (s *Store) ListRuns(ctx context.Context, memberID string, status string, limit int) ([]domrun.RunItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	memUUID := toUUID(memberID)

	q := s.db.AgentRun.Query().
		Where(agentrun.HasMemberWith(member.IDEQ(memUUID))).
		Order(ent.Desc(agentrun.FieldCreatedAt)).
		Limit(limit)

	if status != "" {
		q = q.Where(agentrun.StatusEQ(agentrun.Status(status)))
	}

	list, err := q.All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询任务编排列表失败", err)
	}

	out := make([]domrun.RunItem, 0, len(list))
	for _, r := range list {
		out = append(out, toRunItem(r))
	}
	return out, nil
}

// UpdateRunStatus 更新运行状态（乐观锁防并发覆盖）。
func (s *Store) UpdateRunStatus(ctx context.Context, runID string, expectedVersion int, nextStatus schema.RunStatus) (domrun.RunDetail, error) {
	runUUID := toUUID(runID)
	r, err := s.db.AgentRun.Query().Where(agentrun.IDEQ(runUUID)).Only(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeNotFound, "任务编排不存在", err)
	}

	if expectedVersion > 0 && r.Version != expectedVersion {
		return domrun.RunDetail{}, apperr.New(apperr.CodeConflict, "任务状态已更新，请刷新重试", nil)
	}

	updated, err := r.Update().
		SetStatus(agentrun.Status(nextStatus)).
		SetVersion(r.Version + 1).
		Save(ctx)
	if err != nil {
		return domrun.RunDetail{}, apperr.New(apperr.CodeInternal, "更新任务状态失败", err)
	}

	_, _ = s.RecordRunEvent(ctx, runID, "", "run_status_changed", fmt.Sprintf(`{"from":%q,"to":%q,"version":%d}`, r.Status, nextStatus, updated.Version))
	return s.getRunByID(ctx, runID)
}

// AddRunStep 向编排中添加步骤。
func (s *Store) AddRunStep(ctx context.Context, runID string, step domrun.RunStepInput) (domrun.RunStepItem, error) {
	runUUID := toUUID(runID)

	// 幂等防重：同 operation_key 返回已有
	if step.OperationKey != "" {
		existing, err := s.db.RunStep.Query().
			Where(runstep.OperationKeyEQ(step.OperationKey), runstep.HasRunWith(agentrun.IDEQ(runUUID))).
			First(ctx)
		if err == nil && existing != nil {
			return toRunStepItem(existing), nil
		}
	}

	b := s.db.RunStep.Create().
		SetRunID(runUUID).
		SetPosition(step.Position).
		SetToolName(step.ToolName).
		SetInputJSON(step.InputJSON).
		SetStatus(runstep.StatusPending).
		SetWaitFor(runstep.WaitFor(step.WaitFor)).
		SetExpectedVersion(1)

	if step.OperationKey != "" {
		b.SetOperationKey(step.OperationKey)
	}
	if len(step.DependsOnStepIDs) > 0 {
		b.SetDependsOnStepIds(step.DependsOnStepIDs)
	}
	if step.CompletionKind != "" {
		b.SetCompletionKind(step.CompletionKind)
	}
	if step.ConfirmationNonce != "" {
		b.SetConfirmationNonce(step.ConfirmationNonce)
	}

	saved, err := b.Save(ctx)
	if err != nil {
		return domrun.RunStepItem{}, apperr.New(apperr.CodeInternal, "创建步骤失败", err)
	}

	_, _ = s.RecordRunEvent(ctx, runID, saved.ID.String(), "step_added", fmt.Sprintf(`{"tool":%q,"position":%d}`, step.ToolName, step.Position))
	return toRunStepItem(saved), nil
}

// UpdateRunStep 更新步骤状态与执行卡片。
func (s *Store) UpdateRunStep(ctx context.Context, stepID string, expectedVersion int, status schema.StepStatus, resultCard string) (domrun.RunStepItem, error) {
	stepUUID := toUUID(stepID)
	st, err := s.db.RunStep.Query().Where(runstep.IDEQ(stepUUID)).WithRun().Only(ctx)
	if err != nil {
		return domrun.RunStepItem{}, apperr.New(apperr.CodeNotFound, "步骤不存在", err)
	}

	if expectedVersion > 0 && st.ExpectedVersion != expectedVersion {
		return domrun.RunStepItem{}, apperr.New(apperr.CodeConflict, "步骤状态已过期", nil)
	}

	upd := st.Update().
		SetStatus(runstep.Status(status)).
		SetExpectedVersion(st.ExpectedVersion + 1)

	if resultCard != "" {
		upd.SetResultCard(resultCard)
	}

	saved, err := upd.Save(ctx)
	if err != nil {
		return domrun.RunStepItem{}, apperr.New(apperr.CodeInternal, "更新步骤状态失败", err)
	}

	runID := ""
	if st.Edges.Run != nil {
		runID = st.Edges.Run.ID.String()
	}
	_, _ = s.RecordRunEvent(ctx, runID, stepID, "step_updated", fmt.Sprintf(`{"status":%q,"version":%d}`, status, saved.ExpectedVersion))
	return toRunStepItem(saved), nil
}

// RecordRunEvent 追加记录运行事件（只增不改）。
func (s *Store) RecordRunEvent(ctx context.Context, runID string, stepID string, eventType string, payload string) (domrun.RunEventItem, error) {
	runUUID := toUUID(runID)

	// 计算当前最大 seq
	lastSeq := 0
	last, err := s.db.RunEvent.Query().
		Where(runevent.HasRunWith(agentrun.IDEQ(runUUID))).
		Order(ent.Desc(runevent.FieldSeq)).
		First(ctx)
	if err == nil && last != nil {
		lastSeq = last.Seq
	}

	b := s.db.RunEvent.Create().
		SetRunID(runUUID).
		SetSeq(lastSeq + 1).
		SetEventType(eventType).
		SetPayload(payload)

	if stepID != "" {
		b.SetStepID(stepID)
	}

	saved, err := b.Save(ctx)
	if err != nil {
		return domrun.RunEventItem{}, err
	}
	return toRunEventItem(saved), nil
}
