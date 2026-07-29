package explorer

import (
	"github.com/btcsuite/btcd/chainhash/v2"
)

// recentBlockCount is how deep the "blocks already mined" ribbon reaches.
// The landing page shows 6 tiles (3 on phones) plus the one squeezing
// out; the rest is headroom so the ribbon survives a short reorg without
// going short.
const recentBlockCount = 10

// RecentBlock is one mined block as the ribbon shows it: the tile's
// height, its transaction count (the whole block, matching the block
// view it links to), and when it was mined.
type RecentBlock struct {
	Height  int64 `json:"height"`
	TxCount int   `json:"txCount"`
	Time    int64 `json:"time"`
}

// recentEntry is a cached block plus the link to its parent, so the
// window is rebuilt by walking hashes instead of re-reading heights.
type recentEntry struct {
	block RecentBlock
	// prev is the parent's hash, empty at the genesis block.
	prev string
}

// recentBlocks returns the newest blocks ending at tipHash, newest
// first. Blocks are immutable, so a walk only fetches the ones mined
// since the last call; a reorg changes the hashes and refills the window
// by itself.
//
// Best effort by design: a failed read returns the part of the window
// already gathered (nil if that is nothing) rather than an error. The
// ribbon is decorative — one unreadable block must not blank the
// dashboard — and clients keep the tiles they already hold.
func (s *Service) recentBlocks(tipHash string) []RecentBlock {
	s.recentMu.Lock()
	if s.recentHead == tipHash {
		list := s.recentList
		s.recentMu.Unlock()
		return list
	}
	s.recentMu.Unlock()

	// No lock is held across the RPCs below: OnBlock runs on the
	// rpcclient notification goroutine, and anything it waits for while
	// an RPC is in flight deadlocks the connection (see tx.go).
	out := make([]RecentBlock, 0, recentBlockCount)
	next := tipHash
	for next != "" && len(out) < recentBlockCount {
		entry, err := s.recentEntry(next)
		if err != nil {
			break
		}
		out = append(out, entry.block)
		next = entry.prev
	}
	if len(out) == 0 {
		return nil
	}

	// Only a complete window is published as the cached answer, so a
	// walk cut short by a failed read is retried on the next refresh
	// instead of sticking until the next block.
	if len(out) < recentBlockCount && next != "" {
		return out
	}

	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	// The published slice is never mutated afterwards — each walk builds
	// a fresh one — so readers can hold on to it without copying.
	s.recentList, s.recentHead = out, tipHash
	return out
}

// recentEntry reads one block, cached by hash. It is also the mined-block
// push's source (see BlockFlash), so the block announced over the
// websocket is already in hand when the ribbon walk reaches it.
func (s *Service) recentEntry(hashStr string) (recentEntry, error) {
	if e, ok := s.recentCache.get(hashStr); ok {
		return e, nil
	}

	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return recentEntry{}, err
	}
	raw, err := s.backend.GetBlockVerbose(hash)
	if err != nil {
		return recentEntry{}, err
	}

	entry := recentEntry{
		block: RecentBlock{
			Height:  raw.Height,
			TxCount: len(raw.Tx),
			Time:    raw.Time,
		},
		prev: raw.PreviousHash,
	}
	s.recentCache.put(hashStr, entry)
	return entry, nil
}
