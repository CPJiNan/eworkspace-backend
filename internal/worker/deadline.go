package worker

import (
	"context"
	"log/slog"
	"time"

	"eworkspace/internal/model"
	"eworkspace/internal/repository"
)

type DeadlineTimer struct {
	repos    *repository.Repositories
	log      *slog.Logger
	interval time.Duration
}

func NewDeadlineTimer(repos *repository.Repositories, log *slog.Logger) *DeadlineTimer {
	return &DeadlineTimer{repos: repos, log: log, interval: time.Minute}
}

func (t *DeadlineTimer) Run(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	t.sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.sweep(ctx)
		}
	}
}

func (t *DeadlineTimer) sweep(ctx context.Context) {
	projects, err := t.repos.Project.ListExpiredActive(ctx, time.Now())
	if err != nil {
		t.log.Warn("扫描到期项目失败", "err", err)
		return
	}
	if len(projects) == 0 {
		return
	}

	for _, p := range projects {
		fields := map[string]any{"status": model.ProjectStatusFinished}
		if err := t.repos.Project.UpdateFields(ctx, p.ID, fields); err != nil {
			t.log.Warn("自动结束项目失败", "projectId", p.ID, "err", err)
			continue
		}
		t.log.Info("项目已到截止时间，自动置为已结束", "projectId", p.ID, "deadline", p.Deadline)
	}
}
