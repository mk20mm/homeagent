import { createBrowserRouter, RouterProvider } from 'react-router'

import { AdminLayout } from './layouts/AdminLayout'
import { AdminPage } from './pages/AdminPage'
import { AuditPage } from './pages/AuditPage'
import { DashboardPage } from './pages/DashboardPage'
import { DebugPage } from './pages/DebugPage'

const router = createBrowserRouter([
  {
    path: '/',
    element: <AdminLayout />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'dashboard', element: <DashboardPage /> },
      { path: 'debug', element: <DebugPage /> },
      { path: 'admin', element: <AdminPage /> },
      { path: 'audit', element: <AuditPage /> },
    ],
  },
])

export function App() {
  return <RouterProvider router={router} />
}
