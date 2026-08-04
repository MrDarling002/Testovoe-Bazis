package rediscache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/example/Testovoe-Bazis/internal/metrics"
)

// TaskCache caches serialized task list pages per team. Invalidation bumps a
// per-team version counter, which changes every cache key for that team.
type TaskCache struct {
	rdb     *redis.Client
	ttl     time.Duration
	metrics *metrics.Metrics
	logger  *slog.Logger
}

func NewTaskCache(rdb *redis.Client, ttl time.Duration, m *metrics.Metrics, logger *slog.Logger) *TaskCache {
	return &TaskCache{
		rdb:     rdb,
		ttl:     ttl,
		metrics: m,
		logger:  logger,
	}
}

func (c *TaskCache) GetList(ctx context.Context, teamID int64, filters domain.TaskFilters) ([]byte, bool) {
	key := c.key(ctx, teamID, filters)

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("task cache get failed", "key", key, "error", err)
		}

		c.metrics.CacheMisses.Inc()

		return nil, false
	}

	c.metrics.CacheHits.Inc()

	return data, true
}

func (c *TaskCache) SetList(ctx context.Context, teamID int64, filters domain.TaskFilters, data []byte) {
	key := c.key(ctx, teamID, filters)

	if err := c.rdb.Set(ctx, key, data, c.ttl).Err(); err != nil {
		c.logger.Warn("task cache set failed", "key", key, "error", err)
	}
}

func (c *TaskCache) InvalidateTeam(ctx context.Context, teamID int64) {
	if err := c.rdb.Incr(ctx, c.versionKey(teamID)).Err(); err != nil {
		c.logger.Warn("task cache invalidation failed", "team_id", teamID, "error", err)
	}
}

func (c *TaskCache) versionKey(teamID int64) string {
	return fmt.Sprintf("teams:%d:tasks:version", teamID)
}

// version returns "0" when the version key does not exist yet: INCR of a
// missing key produces 1, so the very first invalidation is guaranteed to
// change the cache keys.
func (c *TaskCache) version(ctx context.Context, teamID int64) string {
	v, err := c.rdb.Get(ctx, c.versionKey(teamID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("task cache version read failed", "team_id", teamID, "error", err)
		}

		return "0"
	}

	return v
}

func (c *TaskCache) key(ctx context.Context, teamID int64, filters domain.TaskFilters) string {
	version := c.version(ctx, teamID)

	status := filters.Status
	if status == "" {
		status = "all"
	}

	assignee := "none"
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
