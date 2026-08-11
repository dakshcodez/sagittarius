package transfer

import (
	"testing"
	"time"

	"github.com/dakshcodez/sagittarius/internal/filemeta"
)

type stubStorage struct{}

func (stubStorage) HasChunk(fileID string, index int) bool                { return false }
func (stubStorage) LoadChunk(fileID string, index int) ([]byte, error)    { return nil, nil }
func (stubStorage) SaveChunk(fileID string, index int, data []byte) error { return nil }
func (stubStorage) GetMissingChunks(meta *filemeta.FileMeta) []int {
	missing := make([]int, meta.NumChunks)
	for i := range missing {
		missing[i] = i
	}
	return missing
}

func testSession(numChunks int) *DownloadSession {
	meta := &filemeta.FileMeta{
		FileID:    "test-file",
		NumChunks: numChunks,
	}
	return NewDownloadSession(meta, stubStorage{})
}

func TestNextRequestableRespectsWindow(t *testing.T) {
	s := testSession(maxOutstandingRequests + 5)

	for i := range maxOutstandingRequests {
		if _, ok := s.nextRequestable(); !ok {
			t.Fatalf("expected chunk %d to be requestable within the window", i)
		}
	}

	if _, ok := s.nextRequestable(); ok {
		t.Fatal("expected window to be full after maxOutstandingRequests claims")
	}
}

func TestRequeueStaleRequestsRevertsTimedOutChunks(t *testing.T) {
	s := testSession(1)

	chunk, ok := s.nextRequestable()
	if !ok {
		t.Fatal("expected to claim the only chunk")
	}
	if chunk.Status != ChunkRequested {
		t.Fatalf("expected ChunkRequested, got %v", chunk.Status)
	}

	// Backdate the request so it looks like it's been outstanding
	// longer than chunkRequestTimeout.
	s.mu.Lock()
	s.chunks[chunk.Index].RequestedAt = time.Now().Add(-2 * chunkRequestTimeout)
	s.mu.Unlock()

	s.requeueStaleRequests()

	s.mu.Lock()
	got := s.chunks[chunk.Index]
	s.mu.Unlock()

	if got.Status != ChunkMissing {
		t.Fatalf("expected chunk to be requeued as ChunkMissing, got %v", got.Status)
	}
	if got.RetryCount != 1 {
		t.Fatalf("expected RetryCount 1, got %d", got.RetryCount)
	}

	// And it should be requestable again.
	if _, ok := s.nextRequestable(); !ok {
		t.Fatal("expected requeued chunk to be requestable again")
	}
}

func TestRequeueStaleRequestsLeavesFreshRequestsAlone(t *testing.T) {
	s := testSession(1)

	chunk, ok := s.nextRequestable()
	if !ok {
		t.Fatal("expected to claim the only chunk")
	}

	s.requeueStaleRequests()

	s.mu.Lock()
	got := s.chunks[chunk.Index]
	s.mu.Unlock()

	if got.Status != ChunkRequested {
		t.Fatalf("expected a fresh request to remain ChunkRequested, got %v", got.Status)
	}
	if got.RetryCount != 0 {
		t.Fatalf("expected RetryCount 0, got %d", got.RetryCount)
	}
}
