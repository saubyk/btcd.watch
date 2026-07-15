import { appConfig } from '../appConfig'
import { useMediaQuery } from './useMediaQuery'

/**
 * The effective motion level: the configured level, forced to 'off' when
 * the OS asks for reduced motion. CSS keyframes are already neutralized
 * globally by the reduced-motion media rule (animations.css); JS-spawned
 * effect nodes (particles, detach chunks) check this so they are never
 * mounted just to play a zero-length animation.
 */
export function useMotionMode(): 'ambient' | 'moments' | 'off' {
  const reduced = useMediaQuery('(prefers-reduced-motion: reduce)')
  return reduced ? 'off' : appConfig.motion
}
