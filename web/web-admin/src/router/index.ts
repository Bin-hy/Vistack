import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/views/Index/index.vue'),
    meta: { layout: 'bili' },
  },
  {
    path: '/sensitive-words',
    name: 'sensitive-words',
    component: () => import('@/views/SensitiveWords/index.vue'),
    meta: { layout: 'bili' },
  },
  // 移除示例关于页路由
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/AuthLogin/index.vue'),
    meta: { layout: 'auth' },
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('@/views/AuthRegister/index.vue'),
    meta: { layout: 'auth' },
  },
]

const router = createRouter({
  // BASE_URL 与 vite base 对齐：本地 dev 为 "/"，Docker 镜像内构建为 "/admin/"
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
