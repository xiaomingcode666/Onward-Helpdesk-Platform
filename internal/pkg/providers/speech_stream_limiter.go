package providers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var ErrSpeechStreamLimitExceeded = errors.New("speech transcription concurrent stream limit exceeded")

type limitedSpeechTranscriptionProvider struct {
	provider SpeechTranscriptionProvider
	quota    *speechStreamQuota
}

func NewLimitedSpeechTranscriptionProvider(provider SpeechTranscriptionProvider, maxConcurrent int) SpeechTranscriptionProvider {
	if provider == nil || maxConcurrent <= 0 {
		return provider
	}
	return newLimitedSpeechTranscriptionProvider(provider, newSpeechStreamQuota(maxConcurrent))
}

func newLimitedSpeechTranscriptionProvider(provider SpeechTranscriptionProvider, quota *speechStreamQuota) SpeechTranscriptionProvider {
	if provider == nil || quota == nil {
		return provider
	}
	return &limitedSpeechTranscriptionProvider{
		provider: provider,
		quota:    quota,
	}
}

func (p *limitedSpeechTranscriptionProvider) Name() string {
	return p.provider.Name()
}

func (p *limitedSpeechTranscriptionProvider) Configured() bool {
	return p.provider.Configured()
}

func (p *limitedSpeechTranscriptionProvider) Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !p.quota.tryAcquire() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, ErrSpeechStreamLimitExceeded
	}

	stream, err := p.provider.Open(ctx, options)
	if err != nil {
		p.quota.release()
		return nil, err
	}
	return &limitedSpeechTranscriptionStream{
		SpeechTranscriptionStream: stream,
		release:                   p.quota.release,
	}, nil
}

type speechStreamQuota struct {
	limit  atomic.Int64
	active atomic.Int64
}

func newSpeechStreamQuota(maxConcurrent int) *speechStreamQuota {
	quota := &speechStreamQuota{}
	quota.setLimit(maxConcurrent)
	return quota
}

func (q *speechStreamQuota) setLimit(maxConcurrent int) {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	q.limit.Store(int64(maxConcurrent))
}

func (q *speechStreamQuota) tryAcquire() bool {
	for {
		active := q.active.Load()
		if active >= q.limit.Load() {
			return false
		}
		if q.active.CompareAndSwap(active, active+1) {
			if active+1 <= q.limit.Load() {
				return true
			}
			q.active.Add(-1)
			return false
		}
	}
}

func (q *speechStreamQuota) release() {
	if active := q.active.Add(-1); active < 0 {
		q.active.Store(0)
		panic("speech stream quota released without an active stream")
	}
}

type limitedSpeechTranscriptionStream struct {
	SpeechTranscriptionStream
	releaseOnce sync.Once
	release     func()
}

func (s *limitedSpeechTranscriptionStream) ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error) {
	event, err := s.SpeechTranscriptionStream.ReadEvent(ctx)
	if err != nil {
		s.releaseSlot()
	}
	return event, err
}

func (s *limitedSpeechTranscriptionStream) Close(ctx context.Context) error {
	err := s.SpeechTranscriptionStream.Close(ctx)
	s.releaseSlot()
	return err
}

func (s *limitedSpeechTranscriptionStream) releaseSlot() {
	s.releaseOnce.Do(s.release)
}
