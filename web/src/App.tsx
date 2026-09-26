import { createBrowserRouter, RouterProvider } from 'react-router'

import { TabBar } from './components/TabBar'
import { RequireAuth } from './components/RequireAuth'
import { AppLayout } from './layouts'
import { ChatPage } from './pages/ChatPage'
import { ChoresPage } from './pages/ChoresPage'
import { EventDetailPage } from './pages/EventDetailPage'
import { EventsPage } from './pages/EventsPage'
import { LoginPage } from './pages/LoginPage'
import { TodayPage } from './pages/TodayPage'
import { MealPage } from './pages/MealPage'
import { MoneyPage } from './pages/MoneyPage'
import { SettingsPage } from './pages/SettingsPage'
import { UndoPage } from './pages/UndoPage'

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
          <TodayPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/chat',
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
    path: '/events',
    element: (
      <RequireAuth>
        <AppLayout>
          <EventsPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
  {
    path: '/events/:eventId',
    element: (
      <RequireAuth>
        <AppLayout>
          <EventDetailPage />
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
  {
    path: '/undo',
    element: (
      <RequireAuth>
        <AppLayout>
          <UndoPage />
          <TabBar />
        </AppLayout>
      </RequireAuth>
    ),
  },
])

export function App() {
  return <RouterProvider router={router} />
}
