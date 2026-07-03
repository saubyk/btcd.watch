import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'

import { CheckIcon } from './Icons'

const ToastContext = createContext<(message: string) => void>(() => {})

export function useToast() {
  return useContext(ToastContext)
}

/** Copies text and reports it via the toast. */
export function useCopy() {
  const toast = useToast()
  return useCallback(
    (text: string) => {
      navigator.clipboard?.writeText(text).catch(() => {})
      toast('Copied to clipboard')
    },
    [toast],
  )
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<string | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)

  const show = useCallback((msg: string) => {
    setMessage(msg)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setMessage(null), 1500)
  }, [])

  useEffect(() => () => clearTimeout(timer.current), [])

  return (
    <ToastContext.Provider value={show}>
      {children}
      {message && (
        <div className="bp-toast" role="status">
          <CheckIcon size={16} color="#5CD08A" width={2.6} />
          {message}
        </div>
      )}
    </ToastContext.Provider>
  )
}
