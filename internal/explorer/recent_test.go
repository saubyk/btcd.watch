package explorer

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcjson"
)

func TestRecentBlocksWindow(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)

	blocks := newTestService(m).recentBlocks(m.hashes[20])

	if len(blocks) != recentBlockCount {
		t.Fatalf("got %d blocks, want %d", len(blocks), recentBlockCount)
	}
	for i, b := range blocks {
		wantHeight := int64(20 - i)
		if b.Height != wantHeight {
			t.Errorf("blocks[%d].Height = %d, want %d (newest first)",
				i, b.Height, wantHeight)
		}
		if want := blockTxCount(wantHeight); b.TxCount != want {
			t.Errorf("block %d txCount = %d, want %d",
				b.Height, b.TxCount, want)
		}
		if b.Time != m.headers[m.hashes[wantHeight]].Time {
			t.Errorf("block %d time = %d, want %d",
				b.Height, b.Time, m.headers[m.hashes[wantHeight]].Time)
		}
	}
}

// A chain shorter than the window ends at the genesis block instead of
// looping or erroring.
func TestRecentBlocksShortChain(t *testing.T) {
	m := newMockBackend()
	installChain(m, 3, 60*time.Second)

	blocks := newTestService(m).recentBlocks(m.hashes[3])

	if len(blocks) != 4 {
		t.Fatalf("got %d blocks, want 4 (heights 3..0)", len(blocks))
	}
	if blocks[3].Height != 0 {
		t.Errorf("oldest height = %d, want 0", blocks[3].Height)
	}
}

// Blocks are immutable: a new tip must only cost one block read, and an
// unchanged tip none at all.
func TestRecentBlocksReusesCachedBlocks(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	s := newTestService(m)

	s.recentBlocks(m.hashes[20])
	fetched := func() int {
		total := 0
		for h := int64(0); h <= m.tip; h++ {
			total += m.blockFetchCount(m.hashes[h])
		}
		return total
	}
	afterFirst := fetched()
	if afterFirst != recentBlockCount {
		t.Fatalf("first walk read %d blocks, want %d",
			afterFirst, recentBlockCount)
	}

	s.recentBlocks(m.hashes[20])
	if got := fetched(); got != afterFirst {
		t.Errorf("re-reading the same tip fetched %d blocks, want 0",
			got-afterFirst)
	}

	installChain(m, 21, 60*time.Second)
	blocks := s.recentBlocks(m.hashes[21])
	if got := fetched() - afterFirst; got != 1 {
		t.Errorf("extending the chain fetched %d blocks, want 1", got)
	}
	if blocks[0].Height != 21 {
		t.Errorf("newest height = %d, want 21", blocks[0].Height)
	}
}

// A reorg keeps the tip height but changes its hash; the window must
// follow the new chain rather than serve the cached one.
func TestRecentBlocksFollowsReorg(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	s := newTestService(m)
	s.recentBlocks(m.hashes[20])

	replacement := hexID("reorgtip")
	m.hashes[20] = replacement
	m.blocks[replacement] = &btcjson.GetBlockVerboseResult{
		Hash:         replacement,
		Height:       20,
		Time:         m.blocks[chainHash(20)].Time + 30,
		Tx:           make([]string, 99),
		PreviousHash: chainHash(19),
	}

	blocks := s.recentBlocks(replacement)

	if len(blocks) != recentBlockCount {
		t.Fatalf("got %d blocks, want %d", len(blocks), recentBlockCount)
	}
	if blocks[0].TxCount != 99 {
		t.Errorf("tip txCount = %d, want 99 (the replacement block)",
			blocks[0].TxCount)
	}
	if blocks[1].Height != 19 {
		t.Errorf("second tile = %d, want the shared parent 19",
			blocks[1].Height)
	}
}

// A read that fails mid-walk returns what it has, and the short window is
// not cached — the next refresh tries again.
func TestRecentBlocksPartialWalkIsRetried(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	missing := m.hashes[17]
	delete(m.blocks, missing)
	s := newTestService(m)

	blocks := s.recentBlocks(m.hashes[20])
	if len(blocks) != 3 {
		t.Fatalf("got %d blocks, want 3 (20, 19, 18)", len(blocks))
	}

	m.blocks[missing] = &btcjson.GetBlockVerboseResult{
		Hash:         missing,
		Height:       17,
		Time:         m.headers[missing].Time,
		Tx:           make([]string, blockTxCount(17)),
		PreviousHash: chainHash(16),
	}
	if blocks = s.recentBlocks(m.hashes[20]); len(blocks) != recentBlockCount {
		t.Errorf("after the block came back the walk returned %d blocks, "+
			"want %d — the short window was cached", len(blocks),
			recentBlockCount)
	}
}

// The mined-block push and the ribbon read the same cache, so announcing
// a block leaves it in hand for the next window walk.
func TestBlockFlashPrimesTheRibbon(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)
	s := newTestService(m)

	flash, err := s.BlockFlash(m.hashes[20])
	if err != nil {
		t.Fatal(err)
	}
	if flash.Height != 20 || flash.TxCount != blockTxCount(20) {
		t.Errorf("flash = %+v, want height 20 with the whole block's "+
			"transactions", flash)
	}

	s.recentBlocks(m.hashes[20])
	if got := m.blockFetchCount(m.hashes[20]); got != 1 {
		t.Errorf("tip read %d times, want 1 (the flash primed the cache)",
			got)
	}
}

func TestStatsCarriesTipTimeAndRibbon(t *testing.T) {
	m := newMockBackend()
	installChain(m, 20, 60*time.Second)

	stats, err := newTestService(m).computeStats()
	if err != nil {
		t.Fatal(err)
	}

	if want := m.headers[m.hashes[20]].Time; stats.TipTime != want {
		t.Errorf("tipTime = %d, want %d", stats.TipTime, want)
	}
	if len(stats.RecentBlocks) != recentBlockCount {
		t.Fatalf("recentBlocks = %d entries, want %d",
			len(stats.RecentBlocks), recentBlockCount)
	}
	if stats.RecentBlocks[0].Height != stats.BlockHeight {
		t.Errorf("newest tile = %d, want the tip %d",
			stats.RecentBlocks[0].Height, stats.BlockHeight)
	}
}
