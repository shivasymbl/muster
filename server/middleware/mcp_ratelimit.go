package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type mcpBucket struct {
	start time.Time
	n     int
}

var (
	mcpLimitMu sync.Mutex
	mcpUsers   = map[string]*mcpBucket{}
	mcpFails   = map[string]*mcpBucket{}
)

// ResetMCPLimits clears the in-process windows. Tests call it.
func ResetMCPLimits() {
	mcpLimitMu.Lock()
	mcpUsers = map[string]*mcpBucket{}
	mcpFails = map[string]*mcpBucket{}
	mcpLimitMu.Unlock()
}

func cleanupMCP(buckets map[string]*mcpBucket, now time.Time) {
	for key, bucket := range buckets {
		if now.Sub(bucket.start) > 2*time.Minute {
			delete(buckets, key)
		}
	}
}

// AuthBlocked reports that this client address already used its 30 failed auths in the current minute.
// It does not increment. The caller must check this before hashing a token.
func AuthBlocked(c *gin.Context, now time.Time) bool {
	mcpLimitMu.Lock()
	defer mcpLimitMu.Unlock()
	cleanupMCP(mcpFails, now)
	bucket := mcpFails[c.ClientIP()]
	if bucket == nil || now.Sub(bucket.start) >= time.Minute {
		return false
	}
	return bucket.n >= 30
}

// NoteAuthFail counts one failed sign-in for this client address.
func NoteAuthFail(c *gin.Context, now time.Time) {
	mcpLimitMu.Lock()
	defer mcpLimitMu.Unlock()
	key := c.ClientIP()
	bucket := mcpFails[key]
	if bucket == nil || now.Sub(bucket.start) >= time.Minute {
		mcpFails[key] = &mcpBucket{start: now, n: 1}
		return
	}
	bucket.n++
}

// AllowUser counts one authenticated call. It returns false when this user is already at 60 in the window.
func AllowUser(userID string, now time.Time) bool {
	mcpLimitMu.Lock()
	defer mcpLimitMu.Unlock()
	cleanupMCP(mcpUsers, now)
	bucket := mcpUsers[userID]
	if bucket == nil || now.Sub(bucket.start) >= time.Minute {
		mcpUsers[userID] = &mcpBucket{start: now, n: 1}
		return true
	}
	if bucket.n >= 60 {
		return false
	}
	bucket.n++
	return true
}
