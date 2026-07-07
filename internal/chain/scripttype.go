package chain

import (
	"github.com/btcsuite/btcd/address/v2"
)

// Script-type codes shared by the address and transaction classification.
// The frontend maps codes to friendly names and explainer copy; an empty
// code means "unknown / non-standard" and hides the type UI.
const (
	ScriptP2TR   = "P2TR"
	ScriptP2WSH  = "P2WSH"
	ScriptP2WPKH = "P2WPKH"
	ScriptP2SH   = "P2SH"
	ScriptP2PKH  = "P2PKH"
)

// ScriptTypeOf classifies a decoded address by its concrete type — exact
// on every network, unlike the design prototype's mainnet prefix rules.
func ScriptTypeOf(addr address.Address) string {
	switch addr.(type) {
	case *address.AddressTaproot:
		return ScriptP2TR
	case *address.AddressWitnessScriptHash:
		return ScriptP2WSH
	case *address.AddressWitnessPubKeyHash:
		return ScriptP2WPKH
	case *address.AddressScriptHash:
		return ScriptP2SH
	case *address.AddressPubKeyHash:
		return ScriptP2PKH
	}
	return ""
}

// ScriptTypeFromRPC maps btcd's scriptPubKey type strings (txscript
// ScriptClass names as they appear in verbose RPC results) to the same
// codes.
func ScriptTypeFromRPC(class string) string {
	switch class {
	case "pubkeyhash":
		return ScriptP2PKH
	case "scripthash":
		return ScriptP2SH
	case "witness_v0_keyhash":
		return ScriptP2WPKH
	case "witness_v0_scripthash":
		return ScriptP2WSH
	case "witness_v1_taproot":
		return ScriptP2TR
	}
	return ""
}
