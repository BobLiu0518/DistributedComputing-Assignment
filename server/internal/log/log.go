package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

func New(component string) *slog.Logger {
	return slog.Default().With("component", component)
}

type CompactHandler struct {
	w     io.Writer
	attrs []slog.Attr
}

func NewCompactHandler(w io.Writer) *CompactHandler {
	return &CompactHandler{w: w}
}

func (h *CompactHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelDebug
}

func (h *CompactHandler) Handle(_ context.Context, r slog.Record) error {
	var buf strings.Builder
	fmt.Fprintf(&buf, "%s %-5s %-28s",
		r.Time.Format("15:04:05"),
		r.Level.String(),
		r.Message,
	)
	for _, a := range h.attrs {
		fmt.Fprintf(&buf, " %s=%v", a.Key, a.Value)
	}
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&buf, " %s=%v", a.Key, a.Value)
		return true
	})
	buf.WriteByte('\n')
	_, err := h.w.Write([]byte(buf.String()))
	return err
}

func (h *CompactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(combined, h.attrs)
	copy(combined[len(h.attrs):], attrs)
	return &CompactHandler{w: h.w, attrs: combined}
}

func (h *CompactHandler) WithGroup(name string) slog.Handler { return h }

