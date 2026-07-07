// Friendly names and plain-English explainers per script-type code. The
// backend classifies (from decoded addresses / scriptPubKey classes) and
// sends only the code; copy is verbatim from the round-3 design handoff.

export interface ScriptTypeMeta {
  name: string
  desc: string
}

export const SCRIPT_TYPES: Record<string, ScriptTypeMeta> = {
  P2TR: {
    name: 'Taproot',
    desc: "This is a Taproot (P2TR) address — Bitcoin's newest format, live since 2021. It's cheaper to spend from and more private: even complex spending rules look like ordinary payments on-chain.",
  },
  P2WSH: {
    name: 'SegWit script',
    desc: 'This is a SegWit script (P2WSH) address — it locks coins behind a custom script, most often a multisig wallet shared between several keys.',
  },
  P2WPKH: {
    name: 'Native SegWit',
    desc: "This is a Native SegWit (P2WPKH) address — today's standard format. Transactions from it are smaller, so it pays the lowest fees of the common address types.",
  },
  P2SH: {
    name: 'Script wrapper',
    desc: 'This is a script-hash (P2SH) address — a wrapper that can hold multisig or wrapped-SegWit setups. Common in older wallets; fees are a bit higher than Native SegWit.',
  },
  P2PKH: {
    name: 'Legacy',
    desc: 'This is a Legacy (P2PKH) address — the original format from 2009. It still works everywhere, but transactions from it are larger and pay the highest fees.',
  },
}
