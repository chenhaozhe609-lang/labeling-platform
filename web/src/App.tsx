import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { AppShell } from '@/components/AppShell'
import { AnnotationWorkbench } from '@/features/annotation/AnnotationWorkbench'
import { LoginPage } from '@/features/auth/LoginPage'
import { DatasetsListPage } from '@/features/dataset/DatasetsListPage'
import { UploadPage } from '@/features/dataset/UploadPage'
import { DatasetDetailPage } from '@/features/dataset/DatasetDetailPage'
import { SchemaEditorPage } from '@/features/dataset/SchemaEditorPage'
import { DashboardPage } from '@/features/dashboard/DashboardPage'
import { queryClient } from '@/lib/query'
import { useAuth } from '@/stores/auth'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAuth((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />

          {/* 沉浸区：标注工作台（无外壳） */}
          <Route
            path="/workspace"
            element={
              <RequireAuth>
                <AnnotationWorkbench />
              </RequireAuth>
            }
          />

          {/* 管理区：带 Activity Rail 外壳 */}
          <Route
            element={
              <RequireAuth>
                <AppShell />
              </RequireAuth>
            }
          >
            <Route path="/datasets" element={<DatasetsListPage />} />
            <Route path="/datasets/upload" element={<UploadPage />} />
            <Route path="/datasets/:id" element={<DatasetDetailPage />} />
            <Route path="/datasets/:id/schema" element={<SchemaEditorPage />} />
            <Route path="/dashboard" element={<DashboardPage />} />
          </Route>

          <Route path="/" element={<Navigate to="/datasets" replace />} />
          <Route path="*" element={<Navigate to="/datasets" replace />} />
        </Routes>
        <Toaster position="bottom-right" />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
