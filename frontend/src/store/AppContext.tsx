import React, { createContext, useContext, useState, useCallback } from 'react'
import type { Project, Meeting, Task, Stats, ViewName, TaskViewName, ModalName, Toast, Status, MenuOptionStatus } from '../types'
import * as api from '../api/client'

interface AppState {
  projects: Project[]
  meetings: Meeting[]
  tasks: Task[]
  stats: Stats | null
  menuOptions: MenuOptionStatus[]
  currentView: ViewName
  taskView: TaskViewName
  filterStatus: string
  activeProject: number | null
  editingTaskId: number | null
  activeMeetingId: number | null
  activeModal: ModalName
  toasts: Toast[]
}

interface AppActions {
  loadAll: () => Promise<void>
  setView: (view: ViewName) => void
  setTaskView: (view: TaskViewName) => void
  setFilterStatus: (status: string) => void
  setActiveProject: (id: number) => void
  setEditingTaskId: (id: number | null) => void
  setActiveMeetingId: (id: number | null) => void
  openModal: (modal: ModalName) => void
  closeModal: () => void
  pushToast: (message: string, isError?: boolean) => void
  updateTaskStatus: (id: number, status: Status) => Promise<void>
  toggleMenuOption: (id: string, enabled: boolean) => Promise<void>
}

const defaultState: AppState = {
  projects: [],
  meetings: [],
  tasks: [],
  stats: null,
  menuOptions: [],
  currentView: 'dashboard',
  taskView: 'list',
  filterStatus: 'all',
  activeProject: null,
  editingTaskId: null,
  activeMeetingId: null,
  activeModal: null,
  toasts: [],
}

const AppStateCtx = createContext<AppState>(defaultState)
const AppActionsCtx = createContext<AppActions>({} as AppActions)

let toastIdCounter = 0

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AppState>(defaultState)

  const loadAll = useCallback(async () => {
    const [projects, meetings, tasks, stats, menuOptions] = await Promise.all([
      api.listProjects().catch(() => [] as Project[]),
      api.listMeetings().catch(() => [] as Meeting[]),
      api.listTasks().catch(() => [] as Task[]),
      api.getStats().catch(() => null),
      api.listMenuOptions().catch(() => [] as MenuOptionStatus[]),
    ])
    setState(prev => ({ ...prev, projects, meetings, tasks, stats, menuOptions }))
  }, [])

  const setView = useCallback((view: ViewName) => {
    setState(prev => ({ ...prev, currentView: view }))
  }, [])

  const setTaskView = useCallback((view: TaskViewName) => {
    setState(prev => ({ ...prev, taskView: view }))
  }, [])

  const setFilterStatus = useCallback((status: string) => {
    setState(prev => ({ ...prev, filterStatus: status }))
  }, [])

  const setActiveProject = useCallback((id: number) => {
    setState(prev => ({
      ...prev,
      activeProject: prev.activeProject === id ? null : id,
    }))
  }, [])

  const setEditingTaskId = useCallback((id: number | null) => {
    setState(prev => ({ ...prev, editingTaskId: id }))
  }, [])

  const setActiveMeetingId = useCallback((id: number | null) => {
    setState(prev => ({ ...prev, activeMeetingId: id }))
  }, [])

  const openModal = useCallback((modal: ModalName) => {
    setState(prev => ({ ...prev, activeModal: modal }))
  }, [])

  const closeModal = useCallback(() => {
    setState(prev => ({ ...prev, activeModal: null }))
  }, [])

  const pushToast = useCallback((message: string, isError = false) => {
    const id = ++toastIdCounter
    setState(prev => ({ ...prev, toasts: [...prev.toasts, { id, message, isError }] }))
    setTimeout(() => {
      setState(prev => ({ ...prev, toasts: prev.toasts.filter(t => t.id !== id) }))
    }, 3200)
  }, [])

  const updateTaskStatus = useCallback(async (id: number, status: Status) => {
    setState(prev => ({
      ...prev,
      tasks: prev.tasks.map(t => t.id === id ? { ...t, status } : t),
    }))
    try {
      await api.updateTask(id, { status })
    } catch {
      setState(prev => ({
        ...prev,
        tasks: prev.tasks.map(t => t.id === id ? { ...t } : t),
      }))
      const id2 = ++toastIdCounter
      setState(prev => ({ ...prev, toasts: [...prev.toasts, { id: id2, message: 'Failed to update task status', isError: true }] }))
      setTimeout(() => {
        setState(prev => ({ ...prev, toasts: prev.toasts.filter(t => t.id !== id2) }))
      }, 3200)
    }
  }, [])

  // toggleMenuOption applies an optimistic enabled/disabled flip, then
  // confirms against the server. On failure it reverts the flip and shows an
  // error toast. On a successful disable of the currently active view, it
  // redirects to the first still-enabled option — done here, not in a
  // useEffect, so the transient pre-load empty state never triggers a
  // spurious redirect (design decision #6).
  const toggleMenuOption = useCallback(async (id: string, enabled: boolean) => {
    let previous: MenuOptionStatus[] = []
    setState(prev => {
      previous = prev.menuOptions
      return {
        ...prev,
        menuOptions: prev.menuOptions.map(m => m.id === id ? { ...m, enabled } : m),
      }
    })
    try {
      await api.setMenuOptionEnabled(id, enabled)
      if (!enabled) {
        setState(prev => {
          if (prev.currentView !== id) return prev
          const firstEnabled = prev.menuOptions.find(m => m.enabled)
          return firstEnabled ? { ...prev, currentView: firstEnabled.id } : prev
        })
      }
    } catch (err: unknown) {
      setState(prev => ({ ...prev, menuOptions: previous }))
      const message = err instanceof Error ? err.message : 'Failed to update menu option'
      const toastId = ++toastIdCounter
      setState(prev => ({ ...prev, toasts: [...prev.toasts, { id: toastId, message, isError: true }] }))
      setTimeout(() => {
        setState(prev => ({ ...prev, toasts: prev.toasts.filter(t => t.id !== toastId) }))
      }, 3200)
    }
  }, [])

  const actions: AppActions = {
    loadAll,
    setView,
    setTaskView,
    setFilterStatus,
    setActiveProject,
    setEditingTaskId,
    setActiveMeetingId,
    openModal,
    closeModal,
    pushToast,
    updateTaskStatus,
    toggleMenuOption,
  }

  return (
    <AppStateCtx.Provider value={state}>
      <AppActionsCtx.Provider value={actions}>
        {children}
      </AppActionsCtx.Provider>
    </AppStateCtx.Provider>
  )
}

export function useAppState() {
  return useContext(AppStateCtx)
}

export function useAppActions() {
  return useContext(AppActionsCtx)
}
