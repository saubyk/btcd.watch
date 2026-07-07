import { SCRIPT_TYPES } from '../lib/scriptTypes'

/**
 * The script-type chip pair: mono code + friendly name. Renders nothing
 * for unknown codes. `suffix` extends the friendly chip ("Native SegWit
 * transaction" on the tx views).
 */
export function TypeChips({
  code,
  suffix = '',
}: {
  code: string
  suffix?: string
}) {
  const meta = SCRIPT_TYPES[code]
  if (!meta) return null

  return (
    <>
      <span className="bp-type-chip">{code}</span>
      <span className="bp-type-chip bp-type-chip--name">
        {meta.name}
        {suffix}
      </span>
    </>
  )
}
