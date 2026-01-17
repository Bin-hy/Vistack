import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/views/Index/index.vue'),
    meta: { layout: 'bili' },
  },
  {
		path: '/profile',
		name: 'profile',
		component: () => import('@/views/Profile/index.vue'),
		meta: { layout: 'bili' },
	},
  {
		path: '/creator',
		name: 'creator',
		component: () => import('@/views/Creator/index.vue'),
		meta: { layout: 'bili', requiresAuth: true },
	},
  {
		path: '/video/:id',
		name: 'video-player',
		component: () => import('@/views/VideoPlayer/index.vue'),
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
  history: createWebHistory(),
  routes,
})

export default router
