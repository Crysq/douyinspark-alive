package handler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// Stream 实时事件流（SSE），只推送当前用户的事件。
func (h *Handler) Stream(c *gin.Context) {
	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		c.String(500, "stream unsupported")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	events := h.Bus.Subscribe()
	defer h.Bus.Unsubscribe(events)
	userID := uid(c)

	fmt.Fprint(c.Writer, ": connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case evt, open := <-events:
			if !open {
				return
			}
			if evt.UserID != 0 && evt.UserID != userID {
				continue // 其他用户的事件不越权推送
			}
			raw, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: spark\ndata: %s\n\n", raw)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
