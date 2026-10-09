package tool

// IsReplayable 返回工具是否支持重放（如只读查询类工具）。
// 未注册或未声明 Replayable 的工具默认返回 false。
func (e *Executor) IsReplayable(name string) bool {
	t, ok := e.registry.Get(name)
	if !ok {
		return false
	}
	return t.Spec().Replayable
}
