import { useAppActions } from '../store/AppContext'

export function useToast() {
  const { pushToast } = useAppActions()
  return { pushToast }
}
