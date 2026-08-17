package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tantrum4132/task-service/internal/model"
	"github.com/Tantrum4132/task-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultTaskCacheTTL = 5 * time.Minute

	taskCachePrefix     = "tasks:list:"
	teamCacheKeysPrefix = "tasks:team_keys:"
)

type TaskCacheDecorator struct {
	next repository.TaskRepository
	rdb  redis.Cmdable
	ttl  time.Duration
}

func NewTaskCacheDecorator(next repository.TaskRepository, rdb redis.Cmdable, ttl time.Duration) repository.TaskRepository {
	if ttl <= 0 {
		ttl = DefaultTaskCacheTTL
	}
	return &TaskCacheDecorator{
		next: next,
		rdb:  rdb,
		ttl:  ttl,
	}
}

func (d *TaskCacheDecorator) CreateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	err := d.next.CreateTask(ctx, exec, task)
	if err != nil {
		return err
	}

	if task != nil {
		_ = d.InvalidateTeamCache(ctx, task.TeamID)
	}

	return nil
}

func (d *TaskCacheDecorator) GetTaskByID(ctx context.Context, exec repository.DBEngine, id int64) (*model.Task, error) {
	return d.next.GetTaskByID(ctx, exec, id)
}

func (d *TaskCacheDecorator) UpdateTask(ctx context.Context, exec repository.DBEngine, task *model.Task) error {
	err := d.next.UpdateTask(ctx, exec, task)
	if err != nil {
		return err
	}

	if task != nil {
		_ = d.InvalidateTeamCache(ctx, task.TeamID)
	}

	return nil
}

func (d *TaskCacheDecorator) DeleteTask(ctx context.Context, exec repository.DBEngine, id int64) error {
	var teamID int64
	if task, err := d.next.GetTaskByID(ctx, exec, id); err == nil && task != nil {
		teamID = task.TeamID
	}

	err := d.next.DeleteTask(ctx, exec, id)
	if err != nil {
		return err
	}

	if teamID != 0 {
		_ = d.InvalidateTeamCache(ctx, teamID)
	}

	return nil
}

func (d *TaskCacheDecorator) ListTasks(ctx context.Context, exec repository.DBEngine, filter repository.TaskFilter) ([]model.Task, error) {
	if filter.TeamID == 0 {
		return nil, repository.ErrTeamIDRequired
	}

	cacheKey := buildTaskFilterCacheKey(filter)

	val, err := d.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		tasks := make([]model.Task, 0)
		if err := json.Unmarshal([]byte(val), &tasks); err == nil {
			return tasks, nil
		}
	}

	tasks, err := d.next.ListTasks(ctx, exec, filter)
	if err != nil {
		return nil, err
	}

	if tasks == nil {
		tasks = make([]model.Task, 0)
	}

	data, err := json.Marshal(tasks)
	if err == nil {
		pipe := d.rdb.Pipeline()
		pipe.Set(ctx, cacheKey, data, d.ttl)

		teamSetKey := fmt.Sprintf("%s%d", teamCacheKeysPrefix, filter.TeamID)
		pipe.SAdd(ctx, teamSetKey, cacheKey)
		pipe.Expire(ctx, teamSetKey, d.ttl+time.Minute)

		_, _ = pipe.Exec(ctx)
	}

	return tasks, nil
}

func (d *TaskCacheDecorator) InvalidateTeamCache(ctx context.Context, teamID int64) error {
	if teamID == 0 {
		return nil
	}

	teamSetKey := fmt.Sprintf("%s%d", teamCacheKeysPrefix, teamID)

	keys, err := d.rdb.SMembers(ctx, teamSetKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	pipe := d.rdb.Pipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}
	pipe.Del(ctx, teamSetKey)

	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return err
	}

	return nil
}

func buildTaskFilterCacheKey(filter repository.TaskFilter) string {
	statusVal := ""
	if filter.Status != nil {
		statusVal = string(*filter.Status)
	}

	assigneeVal := int64(0)
	if filter.AssigneeID != nil {
		assigneeVal = *filter.AssigneeID
	}

	raw := fmt.Sprintf("team:%d:status:%s:assignee:%d:limit:%d:offset:%d",
		filter.TeamID,
		statusVal,
		assigneeVal,
		filter.Limit,
		filter.Offset,
	)

	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%s%d:%s", taskCachePrefix, filter.TeamID, hex.EncodeToString(hash[:8]))
}
