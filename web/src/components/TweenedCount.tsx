import { useMotionMode } from '../hooks/useMotion'
import { useTweenedNumber } from '../hooks/useTweenedNumber'

/**
 * A live count that eases toward its real value instead of jumping
 * (round-7 heartbeat). Mount it only once real data exists — the first
 * value is adopted without tweening, so initial paint never counts up
 * from zero.
 */
export function TweenedCount({
  value,
  format,
}: {
  value: number
  format: (n: number) => string
}) {
  const motionOn = useMotionMode() !== 'off'
  return <>{format(useTweenedNumber(value, motionOn))}</>
}
