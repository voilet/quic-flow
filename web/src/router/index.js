import { createRouter, createWebHashHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { title: '登录', hideLayout: true, public: true }
  },
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/Setup.vue'),
    meta: { title: '初始化向导', hideLayout: true, public: true }
  },
  {
    path: '/',
    name: 'ClientList',
    component: () => import('@/views/ClientList.vue'),
    meta: { title: '客户端列表' }
  },
  {
    path: '/command',
    name: 'CommandSend',
    component: () => import('@/views/CommandSend.vue'),
    meta: { title: '命令发送' }
  },
  {
    path: '/history',
    name: 'CommandHistory',
    component: () => import('@/views/CommandHistory.vue'),
    meta: { title: '命令历史' }
  },
  {
    path: '/terminal',
    name: 'Terminal',
    component: () => import('@/views/Terminal.vue'),
    meta: { title: 'SSH 终端' }
  },
  {
    path: '/audit',
    name: 'AuditLog',
    component: () => import('@/views/AuditLog.vue'),
    meta: { title: '命令审计' }
  },
  {
    path: '/recordings',
    name: 'Recordings',
    component: () => import('@/views/Recordings.vue'),
    meta: { title: '会话录像' }
  },
  {
    path: '/release',
    name: 'Release',
    component: () => import('@/views/Release.vue'),
    meta: { title: '发布管理' }
  },
  {
    path: '/callback-config',
    name: 'CallbackConfig',
    component: () => import('@/views/CallbackConfig.vue'),
    meta: { title: '回调配置' }
  },
  {
    path: '/callback-history',
    name: 'CallbackHistory',
    component: () => import('@/views/CallbackHistory.vue'),
    meta: { title: '回调历史' }
  },
  {
    path: '/credentials',
    name: 'Credentials',
    component: () => import('@/views/Credentials.vue'),
    meta: { title: '凭证中心' }
  },
  {
    path: '/webhooks',
    name: 'Webhooks',
    component: () => import('@/views/Webhooks.vue'),
    meta: { title: 'Webhook 配置' }
  },
  {
    path: '/trigger-history',
    name: 'TriggerHistory',
    component: () => import('@/views/TriggerHistory.vue'),
    meta: { title: '触发历史' }
  },
  {
    path: '/project-members',
    name: 'ProjectMembers',
    component: () => import('@/views/Members.vue'),
    meta: { title: '成员管理' }
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('@/views/Users.vue'),
    meta: { title: '用户管理' }
  },
  {
    path: '/profiling',
    name: 'Profiling',
    component: () => import('@/views/Profiling.vue'),
    meta: { title: '性能分析' }
  },
  {
    path: '/filetransfer',
    name: 'FileTransfer',
    component: () => import('@/views/FileTransfer.vue'),
    meta: { title: '文件传输' }
  },
  {
    path: '/task',
    name: 'TaskList',
    component: () => import('@/views/task/List.vue'),
    meta: { title: '任务管理' }
  },
  {
    path: '/task/execution',
    name: 'TaskExecution',
    component: () => import('@/views/task/Execution.vue'),
    meta: { title: '执行记录' }
  },
  {
    path: '/task/group',
    name: 'TaskGroup',
    component: () => import('@/views/task/Group.vue'),
    meta: { title: '分组管理' }
  },
  {
    path: '/config',
    name: 'ConfigList',
    component: () => import('@/views/ConfigList.vue'),
    meta: { title: '配置中心' }
  },
  {
    path: '/alerts',
    name: 'AlertList',
    component: () => import('@/views/AlertList.vue'),
    meta: { title: '告警列表' }
  },
  {
    path: '/alert-rules',
    name: 'AlertRules',
    component: () => import('@/views/AlertRules.vue'),
    meta: { title: '告警规则' }
  },
  {
    path: '/alert-channels',
    name: 'AlertChannels',
    component: () => import('@/views/AlertChannels.vue'),
    meta: { title: '通知渠道' }
  },
  {
    path: '/silence-rules',
    name: 'SilenceRules',
    component: () => import('@/views/SilenceRules.vue'),
    meta: { title: '抑制规则' }
  },
  {
    path: '/pipeline',
    name: 'PipelineList',
    component: () => import('@/views/pipeline/List.vue'),
    meta: { title: '流水线管理' }
  },
  {
    path: '/pipeline/editor',
    name: 'PipelineEditor',
    component: () => import('@/views/pipeline/Editor.vue'),
    meta: { title: '流水线编辑器' }
  },
  {
    path: '/pipeline/execute',
    name: 'PipelineExecute',
    component: () => import('@/views/pipeline/Execute.vue'),
    meta: { title: '流水线执行' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// 白名单路由
const whiteList = ['/login', '/setup']

// 路由守卫
router.beforeEach(async (to, from, next) => {
  const userStore = useUserStore()

  // 设置页面标题
  if (to.meta.title) {
    document.title = `${to.meta.title} - QUIC Flow`
  }

  const hasToken = userStore.token

  if (hasToken) {
    if (to.path === '/login') {
      // 已登录则跳转到首页
      next({ path: '/' })
    } else {
      // 有 token 就允许访问，不强制获取用户信息
      // 如果需要刷新用户信息，可以在页面组件中自行调用
      next()
    }
  } else {
    // 未登录
    if (whiteList.includes(to.path) || to.meta.public) {
      next()
    } else {
      next(`/login?redirect=${to.path}`)
    }
  }
})

export default router
