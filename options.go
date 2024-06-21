package gocomfy

import "log/slog"

type CommonOption interface {
	ClientOption
}

func WithLog(log *slog.Logger) CommonOption {
	return (*withLog)(log)
}

type withLog slog.Logger

func (opt *withLog) applyToClient(c *clientOptions) {
	c.Log = (*slog.Logger)(opt)
}
