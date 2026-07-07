package chain

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

// regtestAddr builds a valid regtest P2WPKH address string.
func regtestAddr(t *testing.T) string {
	t.Helper()
	addr, err := address.NewAddressWitnessPubKeyHash(
		make([]byte, 20), &chaincfg.RegressionNetParams,
	)
	if err != nil {
		t.Fatalf("build regtest address: %v", err)
	}
	return addr.EncodeAddress()
}

func TestClassifyQuery(t *testing.T) {
	params := &chaincfg.RegressionNetParams

	txid := strings.Repeat("ab", 32)
	bcrt1 := regtestAddr(t)
	// BIP-173 test vector: valid mainnet P2WPKH, must be rejected on
	// regtest.
	mainnetBech32 := "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"

	tests := []struct {
		name string
		in   string
		want QueryKind
	}{
		{"txid lowercase", txid, QueryHex},
		{"txid uppercase normalized", strings.ToUpper(txid), QueryHex},
		{"txid with whitespace", "  " + txid + "  ", QueryHex},
		{"64 chars non-hex", strings.Repeat("zz", 32), QueryInvalid},
		{"regtest bech32", bcrt1, QueryAddress},
		{"mainnet bech32 rejected on regtest", mainnetBech32, QueryInvalid},
		{"garbage", "not-a-txid-or-address", QueryInvalid},
		{"block height digits", "842317", QueryBlockHeight},
		{"block height with commas", "842,317", QueryBlockHeight},
		{"empty", "", QueryInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyQuery(tt.in, params)
			if got.Kind != tt.want {
				t.Fatalf("ClassifyQuery(%q) = %v, want %v",
					tt.in, got.Kind, tt.want)
			}
			if got.Kind == QueryHex && got.Hex != strings.ToLower(strings.TrimSpace(tt.in)) {
				t.Fatalf("hex not normalized: %q", got.Hex)
			}
			if got.Kind == QueryAddress && got.Address.EncodeAddress() != tt.in {
				t.Fatalf("address round-trip mismatch: %q", got.Address.EncodeAddress())
			}
		})
	}
}

func TestClassifyQueryMainnetAcceptsOwnAddress(t *testing.T) {
	got := ClassifyQuery("bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		&chaincfg.MainNetParams)
	if got.Kind != QueryAddress {
		t.Fatalf("mainnet address on mainnet = %v, want address", got.Kind)
	}
}

func TestBlocksUntilHalving(t *testing.T) {
	params := &chaincfg.RegressionNetParams // interval 150

	tests := []struct {
		height int64
		want   int64
	}{
		{0, 150},
		{1, 149},
		{149, 1},
		{150, 150},
		{299, 1},
		{300, 150},
	}
	for _, tt := range tests {
		if got := BlocksUntilHalving(tt.height, params); got != tt.want {
			t.Errorf("BlocksUntilHalving(%d) = %d, want %d",
				tt.height, got, tt.want)
		}
	}
}

func TestParamsForNetwork(t *testing.T) {
	for _, name := range []string{"mainnet", "testnet3", "regtest", "signet", "simnet"} {
		if _, err := ParamsForNetwork(name); err != nil {
			t.Errorf("ParamsForNetwork(%q): %v", name, err)
		}
	}
	if _, err := ParamsForNetwork("bogus"); err == nil {
		t.Error("ParamsForNetwork(bogus) should fail")
	}
}
