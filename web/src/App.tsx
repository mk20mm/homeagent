import { createBrowserRouter, RouterProvider } from 'react-router'

import { TabBar } from './components/TabBar'
import { RequireAuth } from './components/RequireAuth'
import { AppLayout } from './layouts'
import { ChatPage } from './pages/ChatPage'
import { ChoresPage } from './pages/ChoresPage'
import { LoginPage } from './pages/LoginPage'
import { MealPage } from './pages/MealPage'
import { MoneyPage } from './pages/MoneyPage'
import { SettingsPage } from './pages/SettingsPage'

const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppLayout>
          <ChatPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/chores',
    element: (
      <RequireAuth>
        <AppLayout>
          <ChoresPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/money',
    element: (
      <RequireAuth>
        <AppLayout>
          <MoneyPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/meal',
    element: (
      <RequireAuth>
        <AppLayout>
          <MealPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/settings',
    element: (
      <RequireAuth>
        <AppLayout>
          <SettingsPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
])

export function App() {
  return <RouterProvider router={router} />
}
