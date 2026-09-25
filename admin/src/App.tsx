import { createBrowserRouter, Navigate, RouterProvider } from 'react-router'

import { AdminLayout } from './layouts/AdminLayout'
import { AuditPage } from './pages/AuditPage'
import { DashboardPage } from './pages/DashboardPage'
import { DebugPage } from './pages/DebugPage'
import { LoginPage } from './pages/LoginPage'
import { ModelPage } from './pages/ModelPage'
import { PermissionPage } from './pages/PermissionPage'

const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: <AdminLayout />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'dashboard', element: <DashboardPage /> },
      { path: 'debug', element: <DebugPage /> },
      { path: 'models', element: <ModelPage /> },
      { path: 'permissions', element: <PermissionPage /> },
      { path: 'admin', element: <Navigate to="/models" replace /> },
      { path: 'audit', element: <AuditPage /> },
    ],
  },
])

export function App() {
  return <RouterProvider router={router} />
}
