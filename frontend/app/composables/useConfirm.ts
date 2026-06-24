interface ConfirmOptions {
  title: string
  description?: string
  confirmLabel?: string
  color?: 'error' | 'primary'
}

interface ConfirmState extends ConfirmOptions {
  open: boolean
}

let resolver: ((value: boolean) => void) | null = null

export function useConfirm() {
  const state = useState<ConfirmState>('confirm-dialog', () => ({
    open: false,
    title: ''
  }))

  function open(opts: ConfirmOptions): Promise<boolean> {
    state.value = { ...opts, open: true }
    return new Promise<boolean>((resolve) => {
      resolver = resolve
    })
  }

  function resolve(value: boolean) {
    state.value = { ...state.value, open: false }
    resolver?.(value)
    resolver = null
  }

  return { state, open, resolve }
}
