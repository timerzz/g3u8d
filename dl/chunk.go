package dl

import (
	"context"
	"sync"
)

const (
	chunk_status_fail = chunkStatus(iota - 1)
	chunk_status_wait
	chunk_status_downloading
	chunk_status_success
	chunk_status_merged
)

type chunkStatus int

type chunk struct {
	index      int
	status     chunkStatus
	filepath   string
	url        string
	lock       sync.Mutex
	cancel     context.CancelFunc
	ctx        context.Context
	retryTimes int32 //重试的次数
}
