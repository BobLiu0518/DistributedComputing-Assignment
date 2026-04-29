package log

import "log/slog"

func New(component string) *slog.Logger {
	return slog.Default().With("component", component)
}
