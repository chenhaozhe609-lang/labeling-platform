import { QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { SmallScreenGate } from '@/components/SmallScreenGate'
import { AppShell } from '@/components/AppShell'
import { AnnotationWorkbench } from '@/features/annotation/AnnotationWorkbench'
import { ReviewPage } from '@/features/review/ReviewPage'
import { LoginPage } from '@/features/auth/LoginPage'
import { SignupPage } from '@/features/auth/SignupPage'
import { AcceptInvitePage } from '@/features/auth/AcceptInvitePage'
import { PlatformOrgsPage } from '@/features/platform/PlatformOrgsPage'
import { DatasetsListPage } from '@/features/dataset/DatasetsListPage'
import { UploadPage } from '@/features/dataset/UploadPage'
import { DatasetDetailPage } from '@/features/dataset/DatasetDetailPage'
import { SchemaEditorPage } from '@/features/dataset/SchemaEditorPage'
import { DashboardPage } from '@/features/dashboard/DashboardPage'
import { UsersPage } from '@/features/admin/UsersPage'
import { MyTasksPage } from '@/features/me/MyTasksPage'
import { LandingPage } from '@/features/marketing/LandingPage'
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
          <Route path="/signup" element={<SignupPage />} />
          <Route path="/accept-invite" element={<AcceptInvitePage />} />

          {/* 沉浸区：标注工作台（无外壳） */}
          <Route
            path="/workspace"
            element={
              <RequireAuth>
                <AnnotationWorkbench />
              </RequireAuth>
            }
          />

          {/* 沉浸区：审核抽检台（无外壳） */}
          <Route
            path="/review"
            element={
              <RequireAuth>
                <ReviewPage />
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
            <Route path="/admin/users" element={<UsersPage />} />
            <Route path="/platform/orgs" element={<PlatformOrgsPage />} />
            <Route path="/me/tasks" element={<MyTasksPage />} />
          </Route>

          {/* 公开门面：着陆页 */}
          <Route path="/" element={<LandingPage />} />
          <Route path="*" element={<Navigate to="/datasets" replace />} />
        </Routes>
        <Toaster position="bottom-right" />
        <SmallScreenGate />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
