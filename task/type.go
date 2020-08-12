package task

import "context"

// Step 任务执行的步骤
type Step interface {
	// Step 名字
	Name() string
	// 执行
	Do(ctx context.Context, stepIndex int, taskObj Task) *StepResult
	// 回滚
	Cancel(ctx context.Context, stepIndex int, taskObj Task) error
}

// StepResult 任务执行的结果
type StepResult struct {
	StepName string
	Result   interface{}
	Err      error
}

// Task 任务对象，管理Step
type Task interface {
	// Name 任务名
	Name() string
	// Run 执行任务
	Run() (*TaskResult, error)
	// Rollback 任务回滚
	Rollback()
	// 得到运行参数
	GetTaskArgs() interface{}
	//
	GetStepResult(stepIndex int) *StepResult
}
