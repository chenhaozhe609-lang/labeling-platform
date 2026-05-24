import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { AnnotationWorkbench } from '@/features/annotation/AnnotationWorkbench'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/workspace" replace />} />
        <Route path="/workspace" element={<AnnotationWorkbench />} />
        <Route path="*" element={<Navigate to="/workspace" replace />} />
      </Routes>
      <Toaster position="bottom-right" />
    </BrowserRouter>
  )
}

export default App
