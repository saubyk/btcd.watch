package explorer

import (
	"testing"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

// testAddr builds a decodable regtest address for driving Service.Address.
func testAddr(t *testing.T) address.Address {
	t.Helper()
	addr, err := address.NewAddressWitnessPubKeyHash(
		make([]byte, 20), &chaincfg.RegressionNetParams,
	)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

func prevOut(value float64, addrs ...string) btcjson.VinPrevOut {
	return btcjson.VinPrevOut{
		Txid: hexID("parent"),
		PrevOut: &btcjson.PrevOut{
			Addresses: addrs,
			Value:     value,
		},
	}
}

func searchTx(txid string, confirmed bool, vins []btcjson.VinPrevOut,
	vouts []btcjson.Vout) *btcjson.SearchRawTransactionsResult {

	tx := &btcjson.SearchRawTransactionsResult{
		Txid: txid,
		Vin:  vins,
		Vout: vouts,
	}
	if confirmed {
		tx.BlockHash = hexID("blockhash")
		tx.Confirmations = 12
		tx.Blocktime = 1735000000
	} else {
		tx.Time = 1735000500
	}
	return tx
}

func TestAddressSummary(t *testing.T) {
	m := newMockBackend()
	addr := testAddr(t)
	me := addr.EncodeAddress()
	other := "bcrt1qother"

	m.history[me] = []*btcjson.SearchRawTransactionsResult{
		// Newest first: a pending incoming payment…
		searchTx(hexID("t3"), false,
			[]btcjson.VinPrevOut{prevOut(1.0, other)},
			[]btcjson.Vout{vout(0, 0.5, me), vout(1, 0.4999, other)},
		),
		// …a spend with change back to us (0.8 in, 0.3 to other, 0.4999
		// change): net outflow 0.3001 > fee 0.0001 → "sent" 0.3001.
		searchTx(hexID("t2"), true,
			[]btcjson.VinPrevOut{prevOut(0.8, me)},
			[]btcjson.Vout{vout(0, 0.3, other), vout(1, 0.4999, me)},
		),
		// …and the original 1.0 BTC received.
		searchTx(hexID("t1"), true,
			[]btcjson.VinPrevOut{prevOut(1.5, other)},
			[]btcjson.Vout{vout(0, 1.0, me)},
		),
	}

	got, err := newTestService(m).Address(addr, 0, 25)
	if err != nil {
		t.Fatal(err)
	}

	// received = 0.5 + 0.4999 + 1.0 ; sent = 0.8
	if got.ReceivedSats != 199_990_000 || got.SentSats != 80_000_000 {
		t.Errorf("received/sent = %d/%d", got.ReceivedSats, got.SentSats)
	}
	if got.BalanceSats != 119_990_000 {
		t.Errorf("balance = %d", got.BalanceSats)
	}
	if got.TxCount != 3 || got.Approximate || got.HasMore {
		t.Errorf("txCount=%d approx=%v hasMore=%v",
			got.TxCount, got.Approximate, got.HasMore)
	}
	if got.FiatUSD == nil || *got.FiatUSD != 119_990.0 {
		t.Errorf("fiat = %v", got.FiatUSD)
	}

	if len(got.Activity) != 3 {
		t.Fatalf("activity len = %d", len(got.Activity))
	}
	a := got.Activity
	if a[0].Direction != "received" || a[0].AmountSats != 50_000_000 ||
		a[0].Status != "pending" || a[0].Time != 1735000500 {
		t.Errorf("a[0] = %+v", a[0])
	}
	if a[1].Direction != "sent" || a[1].AmountSats != 30_010_000 ||
		a[1].Status != "confirmed" || a[1].Confirmations != 12 {
		t.Errorf("a[1] = %+v", a[1])
	}
	if a[2].Direction != "received" || a[2].AmountSats != 100_000_000 {
		t.Errorf("a[2] = %+v", a[2])
	}
}

// TestAddressSelfTransfer: everything except the fee returns to the same
// address → classified self, amount = the fee.
func TestAddressSelfTransfer(t *testing.T) {
	m := newMockBackend()
	addr := testAddr(t)
	me := addr.EncodeAddress()

	m.history[me] = []*btcjson.SearchRawTransactionsResult{
		searchTx(hexID("self"), true,
			[]btcjson.VinPrevOut{prevOut(1.0, me)},
			[]btcjson.Vout{vout(0, 0.9999, me)},
		),
		// The original funding of the 1.0 BTC being self-spent.
		searchTx(hexID("fund"), true,
			[]btcjson.VinPrevOut{prevOut(2.0, "bcrt1qother")},
			[]btcjson.Vout{vout(0, 1.0, me)},
		),
	}

	got, err := newTestService(m).Address(addr, 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	if got.Activity[0].Direction != "self" ||
		got.Activity[0].AmountSats != 10_000 {
		t.Errorf("activity = %+v", got.Activity[0])
	}
	// Balance reflects only the fee leaving.
	if got.BalanceSats != 99_990_000 {
		t.Errorf("balance = %d", got.BalanceSats)
	}
}

func TestAddressNoHistory(t *testing.T) {
	m := newMockBackend()
	addr := testAddr(t)

	got, err := newTestService(m).Address(addr, 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	if got.TxCount != 0 || got.BalanceSats != 0 || len(got.Activity) != 0 ||
		got.HasMore {
		t.Errorf("summary = %+v", got)
	}
}

func TestAddressPaginationAndCache(t *testing.T) {
	m := newMockBackend()
	addr := testAddr(t)
	me := addr.EncodeAddress()
	other := "bcrt1qother"

	// 7 received txs of 0.1 each, newest first.
	var history []*btcjson.SearchRawTransactionsResult
	for i := range 7 {
		history = append(history, searchTx(hexID("p"+string(rune('a'+i))), true,
			[]btcjson.VinPrevOut{prevOut(0.2, other)},
			[]btcjson.Vout{vout(0, 0.1, me)},
		))
	}
	m.history[me] = history

	svc := newTestService(m)

	page1, err := svc.Address(addr, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Activity) != 3 || !page1.HasMore || page1.TxCount != 7 {
		t.Errorf("page1 = %d rows hasMore=%v count=%d",
			len(page1.Activity), page1.HasMore, page1.TxCount)
	}
	if page1.ReceivedSats != 70_000_000 {
		t.Errorf("received = %d", page1.ReceivedSats)
	}

	fetchesAfterPage1 := m.searchFetches

	page3, err := svc.Address(addr, 6, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(page3.Activity) != 1 || page3.HasMore {
		t.Errorf("page3 = %d rows hasMore=%v",
			len(page3.Activity), page3.HasMore)
	}
	// Totals were cached — only the activity page itself was fetched.
	if m.searchFetches != fetchesAfterPage1+1 {
		t.Errorf("search fetches = %d, want %d (cached totals)",
			m.searchFetches, fetchesAfterPage1+1)
	}
}

// TestAddressScanCap: more history than MaxScanTxs → approximate totals.
func TestAddressScanCap(t *testing.T) {
	m := newMockBackend()
	addr := testAddr(t)
	me := addr.EncodeAddress()
	other := "bcrt1qother"

	var history []*btcjson.SearchRawTransactionsResult
	for i := range 250 {
		history = append(history, searchTx(hexID("x"+string(rune(i))), true,
			[]btcjson.VinPrevOut{prevOut(0.2, other)},
			[]btcjson.Vout{vout(0, 0.01, me)},
		))
	}
	m.history[me] = history

	svc := NewService(m, Config{
		Params: &chaincfg.RegressionNetParams,
		Price: func() PriceQuote {
			return PriceQuote{OK: false}
		},
		MaxScanTxs: 200,
	})

	got, err := svc.Address(addr, 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Approximate {
		t.Error("approximate = false, want true at scan cap")
	}
	if got.TxCount != 200 {
		t.Errorf("txCount = %d, want 200 (capped)", got.TxCount)
	}
	if got.ReceivedSats != 200*1_000_000 {
		t.Errorf("received = %d", got.ReceivedSats)
	}
	if got.FiatUSD != nil {
		t.Error("fiat should be null without a price")
	}
}
