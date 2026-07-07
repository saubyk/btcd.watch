package chain

import (
	"testing"

	"github.com/btcsuite/btcd/address/v2"
	"github.com/btcsuite/btcd/chaincfg/v2"
)

func TestScriptTypeOf(t *testing.T) {
	params := &chaincfg.RegressionNetParams
	h20 := make([]byte, 20)
	h32 := make([]byte, 32)

	p2wpkh, err := address.NewAddressWitnessPubKeyHash(h20, params)
	if err != nil {
		t.Fatal(err)
	}
	p2wsh, err := address.NewAddressWitnessScriptHash(h32, params)
	if err != nil {
		t.Fatal(err)
	}
	p2tr, err := address.NewAddressTaproot(h32, params)
	if err != nil {
		t.Fatal(err)
	}
	p2sh, err := address.NewAddressScriptHashFromHash(h20, params)
	if err != nil {
		t.Fatal(err)
	}
	p2pkh, err := address.NewAddressPubKeyHash(h20, params)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		addr address.Address
		want string
	}{
		{p2wpkh, ScriptP2WPKH},
		{p2wsh, ScriptP2WSH},
		{p2tr, ScriptP2TR},
		{p2sh, ScriptP2SH},
		{p2pkh, ScriptP2PKH},
	}
	for _, c := range cases {
		if got := ScriptTypeOf(c.addr); got != c.want {
			t.Errorf("ScriptTypeOf(%T) = %q, want %q", c.addr, got, c.want)
		}
	}
}

func TestScriptTypeFromRPC(t *testing.T) {
	cases := map[string]string{
		"pubkeyhash":            ScriptP2PKH,
		"scripthash":            ScriptP2SH,
		"witness_v0_keyhash":    ScriptP2WPKH,
		"witness_v0_scripthash": ScriptP2WSH,
		"witness_v1_taproot":    ScriptP2TR,
		"nulldata":              "",
		"nonstandard":           "",
		"":                      "",
	}
	for class, want := range cases {
		if got := ScriptTypeFromRPC(class); got != want {
			t.Errorf("ScriptTypeFromRPC(%q) = %q, want %q", class, got, want)
		}
	}
}
