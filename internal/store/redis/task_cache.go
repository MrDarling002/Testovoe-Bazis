package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/redis/go-redis/v9"
)

type TaskCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewTaskCache(rdb *redis.Client, ttl time.Duration) *TaskCache {
	return &TaskCache{
		rdb: rdb,
		ttl: ttl,
	}
}

func (c *TaskCache) GetList(ctx context.Context, teamID int64, filters domain.TaskFilters) ([]byte, bool) {
	key := c.key(ctx, teamID, filters)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	return data, true
}

func (c *TaskCache) SetList(ctx context.Context, teamID int64, filters domain.TaskFilters, data []byte) {
	key := c.key(ctx, teamID, filters)
	if err := c.rdb.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return
	}
}

func (c *TaskCache) InvalidateTeam(ctx context.Context, teamID int64) {
	key := c.versionKey(teamID)
	c.rdb.Incr(ctx, key)
}

func (c *TaskCache) versionKey(teamID int64) string {
	return fmt.Sprintf("teams:%d:tasks:version", teamID)
}

func (c *TaskCache) version(ctx context.Context, teamID int64) string {
	key := c.versionKey(teamID)
	v, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "1"
	}

	return v
}

func (c *TaskCache) key(ctx context.Context, teamID int64, filters domain.TaskFilters) string {
	version := c.version(ctx, teamID)
	status := filters.Status
	if status == "" {
		status = "all"
	}

	assignee := "null"
	if filters.AssigneeID != nil {
		assignee = strconv.FormatInt(*filters.AssigneeID, 10)
	}

	return fmt.Sprintf(
		"teams:%d:tasks:v%s:status:%s:assignee:%s:page:%d:size:%d",
		teamID,
		version,
		status,
		assignee,
		filters.Page,
		filters.PerPage,
	)
}
