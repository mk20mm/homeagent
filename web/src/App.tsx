import { createBrowserRouter, RouterProvider } from 'react-router'

import { TabBar } from './components/TabBar'
import { AppLayout } from './layouts'
import { ChatPage } from './pages/ChatPage'
import { ChoresPage } from './pages/ChoresPage'
import { MealPage } from './pages/MealPage'
import { MoneyPage } from './pages/MoneyPage'
import { SettingsPage } from './pages/SettingsPage'

const router = createBrowserRouter([
  {
    path: '/',
    element: (
      <AppLayout>
        <ChatPage />
        <TabBar />
      </AppLayout>
    ),
  },
  {
    path: '/chores',
    element: (
      <AppLayout>
        <ChoresPage />
        <TabBar />
      </AppLayout>
    ),
  },
  {
    path: '/money',
    element: (
      <AppLayout>
        <MoneyPage />
        <TabBar />
      </AppLayout>
    ),
  },
  {
    path: '/meal',
    element: (
      <AppLayout>
        <MealPage />
        <TabBar />
      </AppLayout>
    ),
  },
  {
    path: '/settings',
    element: (
      <AppLayout>
        <SettingsPage />
        <TabBar />
      </AppLayout>
    ),
  },
])

export function App() {
  return <RouterProvider router={router} />
}
