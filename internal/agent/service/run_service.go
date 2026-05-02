package service

import (
	"context"
	"time"

	agentmodel "myclouddrive-go/internal/agent/model"
)

// runService 管理 run/step 的持久化。
// 当前为占位实现，后续接入 agent_run / agent_step 表。
type runService struct{}

func newRunService() *runService {
	return &runService{}
}

func (r *runService) createRun(_ context.Context, run *agentmodel.Run) error {
	run.CreatedAt = time.Now()
	run.UpdatedAt = time.Now()
	return nil
}

func (r *runService) updateRunStatus(_ context.Context, runID, status string) error {
	return nil
}

func (r *runService) createStep(_ context.Context, step *agentmodel.Step) error {
	step.CreatedAt = time.Now()
	return nil
}
