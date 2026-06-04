import React, { createContext, useContext, useState, useCallback } from 'react'
import type { Project, Meeting, Task, Stats, ViewName, TaskViewName, ModalName, Toast } from '../types'
import * as api from '../api/client'

interface AppState {
  projects: Project[]
  meetings: Meeting[]
  tasks: Task[]
  stats: Stats | null
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
}

const defaultState: AppState = {
  projects: [],
  meetings: [],
  tasks: [],
  stats: null,
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
    const [projects, meetings, tasks, stats] = await Promise.all([
      api.listProjects().catch(() => [] as Project[]),
      api.listMeetings().catch(() => [] as Meeting[]),
      api.listTasks().catch(() => [] as Task[]),
      api.getStats().catch(() => null),
    ])
    setState(prev => ({ ...prev, projects, meetings, tasks, stats }))
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
