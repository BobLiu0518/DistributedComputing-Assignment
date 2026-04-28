package router

import (
	"context"
	"fmt"
	"sync"
)

type HandlerFunc func(ctx context.Context, payload []byte) ([]byte, error)

type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

func New() *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
	}
}

func (r *Router) Register(service, method string, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[fmt.Sprintf("%s.%s", service, method)] = handler
}

func (r *Router) Route(service, method string) (HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[fmt.Sprintf("%s.%s", service, method)]
	return h, ok
}
