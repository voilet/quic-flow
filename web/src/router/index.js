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
    path: '/config/history',
    name: 'ConfigHistory',
    component: () => import('@/views/ConfigList.vue'),
    meta: { title: '配置历史' }
  },
  {
    path: '/config/releases',
    name: 'ConfigReleases',
    component: () => import('@/views/config/Releases.vue'),
    meta: { title: '发布管理' }
  },
  {
    path: '/config/gray-rules',
    name: 'ConfigGrayRules',
    component: () => import('@/views/config/GrayRules.vue'),
    meta: { title: '灰度规则' }
  },
  {
    path: '/config/subscribers',
    name: 'ConfigSubscribers',
    component: () => import('@/views/config/Subscribers.vue'),
    meta: { title: '订阅者' }
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
    path: '/',
    name: 'ProjectWorkspace',
    component: () => import('@/views/ProjectWorkspace.vue'),
    meta: { title: '项目工作台' }
  },
  // 项目工作台路由
  {
    path: '/project/overview',
    name: 'ProjectOverview',
    component: () => import('@/views/project/Overview.vue'),
    meta: { title: '项目概览' }
  },
  {
    path: '/project/config',
    name: 'ProjectConfig',
    component: () => import('@/views/ConfigList.vue'),
    meta: { title: '配置中心' }
  },
  
  {
    path: '/project/config/history',
    name: 'ProjectConfigHistory',
    component: () => import('@/views/ConfigList.vue'),
    meta: { title: '配置历史' }
  },
  {
    path: '/project/pipeline',
    name: 'ProjectPipeline',
    component: () => import('@/views/pipeline/List.vue'),
    meta: { title: '流水线列表' }
  },
  {
    path: '/project/pipeline/editor',
    name: 'ProjectPipelineEditor',
    component: () => import('@/views/pipeline/Editor.vue'),
    meta: { title: '流水线编辑器' }
  },
  {
    path: '/project/pipeline/editor/:id',
    name: 'ProjectPipelineEditorEdit',
    component: () => import('@/views/pipeline/Editor.vue'),
    meta: { title: '编辑流水线' }
  },
  {
    path: '/project/pipeline/templates',
    name: 'ProjectPipelineTemplates',
    component: () => import('@/views/pipeline/Templates.vue'),
    meta: { title: '模板管理' }
  },
  {
    path: '/project/executions',
    name: 'ProjectExecutions',
    component: () => import('@/views/pipeline/Execute.vue'),
    meta: { title: '执行历史' }
  },
  {
    path: '/project/alerts',
    name: 'ProjectAlerts',
    component: () => import('@/views/AlertList.vue'),
    meta: { title: '告警列表' }
  },
  {
    path: '/project/alert-rules',
    name: 'ProjectAlertRules',
    component: () => import('@/views/AlertRules.vue'),
    meta: { title: '告警规则' }
  },
  {
    path: '/project/alert-channels',
    name: 'ProjectAlertChannels',
    component: () => import('@/views/AlertChannels.vue'),
    meta: { title: '通知渠道' }
  },
  {
    path: '/project/silence-rules',
    name: 'ProjectSilenceRules',
    component: () => import('@/views/SilenceRules.vue'),
    meta: { title: '抑制规则' }
  },
  {
    path: '/project/oncall',
    name: 'ProjectOnCall',
    component: () => import('@/views/OnCallSchedule.vue'),
    meta: { title: '值班管理' }
  },
  {
    path: '/project/config/gray-rules',
    name: 'ProjectConfigGrayRules',
    component: () => import('@/views/config/GrayRules.vue'),
    meta: { title: '灰度规则' }
  },
  {
    path: '/project/config/subscribers',
    name: 'ProjectConfigSubscribers',
    component: () => import('@/views/config/Subscribers.vue'),
    meta: { title: '订阅者' }
  },
  {
    path: '/project/config/releases',
    name: 'ProjectConfigReleases',
    component: () => import('@/views/config/Releases.vue'),
    meta: { title: '发布管理' }
  },
  // 兼容旧路由
  {
    path: '/pipeline',
    redirect: '/'
  },
  {
    path: '/pipeline/editor',
    redirect: '/'
  },
  {
    path: '/pipeline/execute',
    redirect: '/'
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
