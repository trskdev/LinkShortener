package app

import (
	"context"
	"log/slog"
)

type LogCollector struct {
	Records []slog.Record
}

func (c *LogCollector) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (c *LogCollector) WithAttrs(attrs []slog.Attr) slog.Handler     { return c }
func (c *LogCollector) WithGroup(name string) slog.Handler           { return c }

func (c *LogCollector) Handle(_ context.Context, r slog.Record) error {
	c.Records = append(c.Records, r.Clone())
	return nil
}
