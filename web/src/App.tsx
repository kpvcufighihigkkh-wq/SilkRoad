import { useEffect, useState } from 'react'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { Spin } from 'antd'
import { UNAUTHORIZED_EVENT } from '@/api/client'
import { isLoggedIn } from '@/auth/session'
import MainLayout from '@/layouts/MainLayout'
import LoginPage from '@/pages/LoginPage'
import LotsPage from '@/pages/LotsPage'
import LotDetailPage from '@/pages/LotDetailPage'
import ReportsPage from '@/pages/ReportsPage'

/**
 * AuthGate 负责两件事：
 *   1. 未登录访问受保护路由时重定向到 /login，并记住原本要去的地方；
 *   2. 监听请求层的 401 事件 —— token 过期或账号被停用后，
 *      立即把用户送回登录页，而不是让页面停在报错状态。
 *
 * 不用路由加载器去校验会话：那会让每次导航都多一次请求。
 * 这里只信本地 token 是否存在，真正的有效性由后端每请求判定。
 */
function AuthGate({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const [, forceRender] = useState(0)

  useEffect(() => {
    const onUnauthorized = () => forceRender((n) => n + 1)
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized)
  }, [])

  if (!isLoggedIn()) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
  }

  return <>{children}</>
}

function LoginRoute() {
  // 已登录时直接进主界面，避免回退键回到登录页。
  if (isLoggedIn()) {
    return <Navigate to="/lots" replace />
  }
  return <LoginPage />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginRoute />} />

        <Route
          path="/"
          element={
            <AuthGate>
              <MainLayout />
            </AuthGate>
          }
        >
          {/* 默认落在批次管理：它是当前唯一有数据的业务页 */}
          <Route index element={<Navigate to="/lots" replace />} />
          <Route path="lots" element={<LotsPage />} />
          <Route path="lots/:id" element={<LotDetailPage />} />
          <Route path="reports" element={<ReportsPage />} />
        </Route>

        <Route path="*" element={<Navigate to="/lots" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

/** 供页面共用的整页加载态。 */
export function PageLoading() {
  return (
    <div style={{ display: 'grid', placeItems: 'center', minHeight: 320 }}>
      <Spin size="large" />
    </div>
  )
}
