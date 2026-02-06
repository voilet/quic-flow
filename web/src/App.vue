<template>
  <!-- 初始化引导页面（无布局） -->
  <router-view v-if="$route.meta.hideLayout" />

  <!-- 主应用布局 -->
  <el-container v-else class="app-container">
    <!-- 侧边栏 -->
    <el-aside width="240px" class="app-aside">
      <div class="logo">
        <h2>PantheonQuic</h2>
      </div>
      <el-menu
        :default-active="$route.path"
        router
        class="el-menu-vertical"
      >
        <!-- 客户端管理 -->
        <el-sub-menu index="client">
          <template #title>
            <el-icon><Monitor /></el-icon>
            <span>客户端管理</span>
          </template>
          <el-menu-item index="/">
            <el-icon><Platform /></el-icon>
            <span>客户端列表</span>
          </el-menu-item>
          <el-menu-item index="/terminal">
            <el-icon><Monitor /></el-icon>
            <span>SSH 终端</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 命令管理 -->
        <el-sub-menu index="command">
          <template #title>
            <el-icon><DocumentAdd /></el-icon>
            <span>命令管理</span>
          </template>
          <el-menu-item index="/command">
            <el-icon><DocumentAdd /></el-icon>
            <span>命令下发</span>
          </el-menu-item>
          <el-menu-item index="/history">
            <el-icon><Document /></el-icon>
            <span>命令历史</span>
          </el-menu-item>
          <el-menu-item index="/audit">
            <el-icon><List /></el-icon>
            <span>命令审计</span>
          </el-menu-item>
          <el-menu-item index="/recordings">
            <el-icon><VideoCamera /></el-icon>
            <span>会话录像</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 发布管理 -->
        <el-sub-menu index="release">
          <template #title>
            <el-icon><Upload /></el-icon>
            <span>发布管理</span>
          </template>
          <el-menu-item index="/release">
            <el-icon><Platform /></el-icon>
            <span>项目列表</span>
          </el-menu-item>
          <el-menu-item index="/credentials">
            <el-icon><Key /></el-icon>
            <span>凭证中心</span>
          </el-menu-item>
          <el-menu-item index="/webhooks">
            <el-icon><Connection /></el-icon>
            <span>Webhook 配置</span>
          </el-menu-item>
          <el-menu-item index="/trigger-history">
            <el-icon><Clock /></el-icon>
            <span>触发历史</span>
          </el-menu-item>
          <el-menu-item index="/callback-config">
            <el-icon><Bell /></el-icon>
            <span>回调配置</span>
          </el-menu-item>
          <el-menu-item index="/callback-history">
            <el-icon><Clock /></el-icon>
            <span>回调历史</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 权限管理 -->
        <el-sub-menu index="permission">
          <template #title>
            <el-icon><Lock /></el-icon>
            <span>权限管理</span>
          </template>
          <el-menu-item index="/project-members">
            <el-icon><User /></el-icon>
            <span>项目成员</span>
          </el-menu-item>
          <el-menu-item index="/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 定时任务 -->
        <el-sub-menu index="task">
          <template #title>
            <el-icon><Timer /></el-icon>
            <span>定时任务</span>
          </template>
          <el-menu-item index="/task">
            <el-icon><List /></el-icon>
            <span>任务管理</span>
          </el-menu-item>
          <el-menu-item index="/task/execution">
            <el-icon><Document /></el-icon>
            <span>执行记录</span>
          </el-menu-item>
          <el-menu-item index="/task/group">
            <el-icon><User /></el-icon>
            <span>分组管理</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 告警系统 -->
        <el-sub-menu index="alert">
          <template #title>
            <el-icon><Warning /></el-icon>
            <span>告警系统</span>
          </template>
          <el-menu-item index="/alerts">
            <el-icon><Bell /></el-icon>
            <span>告警列表</span>
          </el-menu-item>
          <el-menu-item index="/alert-rules">
            <el-icon><Document /></el-icon>
            <span>告警规则</span>
          </el-menu-item>
          <el-menu-item index="/alert-channels">
            <el-icon><Connection /></el-icon>
            <span>通知渠道</span>
          </el-menu-item>
          <el-menu-item index="/silence-rules">
            <el-icon><MuteNotification /></el-icon>
            <span>抑制规则</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 配置中心 -->
        <el-sub-menu index="config">
          <template #title>
            <el-icon><Setting /></el-icon>
            <span>配置中心</span>
          </template>
          <el-menu-item index="/config">
            <el-icon><Files /></el-icon>
            <span>配置管理</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 流水线管理 -->
        <el-sub-menu index="pipeline">
          <template #title>
            <el-icon><Operation /></el-icon>
            <span>流水线</span>
          </template>
          <el-menu-item index="/pipeline">
            <el-icon><List /></el-icon>
            <span>流水线列表</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 系统工具 -->
        <el-sub-menu index="tools">
          <template #title>
            <el-icon><TrendCharts /></el-icon>
            <span>系统工具</span>
          </template>
          <el-menu-item index="/filetransfer">
            <el-icon><Files /></el-icon>
            <span>文件传输</span>
          </el-menu-item>
          <el-menu-item index="/profiling">
            <el-icon><TrendCharts /></el-icon>
            <span>性能分析</span>
          </el-menu-item>
        </el-sub-menu>

        <!-- 系统设置 -->
        <el-menu-item index="/setup" class="setup-menu-item">
          <el-icon><Setting /></el-icon>
          <span>数据库设置</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <!-- 主内容区 -->
    <el-container>
      <!-- 顶部导航栏 -->
      <el-header class="app-header">
        <div class="header-content">
          <span class="header-title">{{ pageTitle }}</span>
          <div class="header-actions">
            <!-- <el-button
              :icon="theme === 'dark' ? 'Sunny' : 'Moon'"
              circle
              @click="toggleTheme"
              class="theme-toggle-btn"
              :title="theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
            /> -->
            <el-tag :type="dbStatus.type">
              <el-icon><component :is="dbStatus.icon" /></el-icon>
              {{ dbStatus.text }}
            </el-tag>

            <!-- 用户信息 -->
            <el-dropdown v-if="userStore.isLoggedIn" @command="handleUserCommand">
              <div class="user-info">
                <el-icon class="user-icon"><User /></el-icon>
                <span class="user-name">{{ userStore.displayName }}</span>
                <el-icon class="dropdown-icon"><ArrowDown /></el-icon>
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item disabled>
                    <span class="user-detail">
                      <el-icon><User /></el-icon>
                      {{ userStore.userInfo?.username }}
                    </span>
                  </el-dropdown-item>
                  <el-dropdown-item disabled v-if="userStore.userInfo?.email">
                    <span class="user-detail">
                      <el-icon><Message /></el-icon>
                      {{ userStore.userInfo?.email }}
                    </span>
                  </el-dropdown-item>
                  <el-dropdown-item divided :icon="SwitchButton" command="logout">
                    退出登录
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </el-header>

      <!-- 内容区 -->
      <el-main class="app-main">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  User, ArrowDown, Message, SwitchButton, TrendCharts, Files, Bell, Clock, Key, Connection,
  Monitor, DocumentAdd, Document, List, VideoCamera, Upload, Setting, Platform, Lock, Timer,
  Warning, MuteNotification, Operation
} from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { request } from '@/api'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

// 主题管理
const theme = ref(localStorage.getItem('theme') || 'light')

function toggleTheme() {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  localStorage.setItem('theme', theme.value)
  updateTheme()
}

function updateTheme() {
  document.documentElement.setAttribute('data-theme', theme.value)
}

// 数据库状态
const dbInitialized = ref(null) // null=检查中, true=已初始化, false=未初始化

const pageTitle = computed(() => {
  const titles = {
    '/': '客户端列表',
    '/command': '命令下发',
    '/history': '命令历史',
    '/terminal': 'SSH 终端',
    '/audit': '命令审计',
    '/recordings': '会话录像',
    '/release': '发布管理',
    '/credentials': '凭证中心',
    '/webhooks': 'Webhook 配置',
    '/trigger-history': '触发历史',
    '/project-members': '项目成员',
    '/callback-config': '回调配置',
    '/callback-history': '回调历史',
    '/profiling': '性能分析',
    '/filetransfer': '文件传输',
    '/task': '任务管理',
    '/task/execution': '执行记录',
    '/task/group': '分组管理',
    '/alerts': '告警列表',
    '/alert-rules': '告警规则',
    '/alert-channels': '通知渠道',
    '/silence-rules': '抑制规则',
    '/config': '配置中心',
    '/pipeline': '流水线管理',
    '/pipeline/editor': '流水线编辑器',
    '/pipeline/execute': '流水线执行',
    '/setup': '数据库设置',
    '/users': '用户管理'
  }
  return titles[route.path] || 'Pantheon Quic　管理系统'
})

const dbStatus = computed(() => {
  if (dbInitialized.value === null) {
    return { type: 'info', text: '检查中...', icon: 'Loading' }
  } else if (dbInitialized.value) {
    return { type: 'success', text: '数据库已连接', icon: 'Connection' }
  } else {
    return { type: 'warning', text: '数据库未配置', icon: 'WarningFilled' }
  }
})

// 用户操作处理
async function handleUserCommand(command) {
  switch (command) {
    case 'logout':
      try {
        await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })
        await userStore.logout()
        ElMessage.success('已退出登录')
        router.push('/login')
      } catch (error) {
        // 用户取消或登出失败
        if (error !== 'cancel') {
          console.error('Logout error:', error)
        }
      }
      break
  }
}

// 检查数据库状态
async function checkDatabaseStatus() {
  try {
    const res = await request.get('/setup/status')
    if (res.success) {
      dbInitialized.value = res.status.initialized
    }
  } catch (e) {
    dbInitialized.value = false
  }
}

onMounted(() => {
  updateTheme() // 初始化主题
  checkDatabaseStatus()
})
</script>

<style scoped>
.app-container {
  height: 100vh;
  background: var(--tech-bg-gradient);
  position: relative;
  overflow: hidden;
  transition: background-color 0.3s ease;
}

.app-container::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: 
    radial-gradient(circle at 20% 30%, rgba(64, 158, 255, 0.03) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(103, 194, 58, 0.03) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

[data-theme="dark"] .app-container::before {
  background: 
    radial-gradient(circle at 20% 30%, rgba(64, 158, 255, 0.05) 0%, transparent 50%),
    radial-gradient(circle at 80% 70%, rgba(103, 194, 58, 0.05) 0%, transparent 50%);
}

.app-aside {
  background: var(--tech-bg-secondary);
  color: var(--tech-text-primary);
  border-right: 1px solid var(--tech-border);
  position: relative;
  z-index: 1;
  box-shadow: var(--tech-shadow-md);
  transition: background-color 0.3s ease, border-color 0.3s ease;
}

.app-aside::after {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  width: 1px;
  height: 100%;
  background: linear-gradient(
    180deg,
    transparent,
    var(--tech-primary),
    transparent
  );
  opacity: 0.2;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.08) 0%, rgba(64, 158, 255, 0.03) 100%);
  border-bottom: 1px solid var(--tech-border);
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.logo::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(64, 158, 255, 0.1),
    transparent
  );
  transition: left 0.5s ease;
}

.logo:hover::before {
  left: 100%;
}

.logo h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--tech-primary);
  letter-spacing: 0.5px;
  position: relative;
  z-index: 1;
  transition: color 0.3s ease;
}

.logo:hover h2 {
  color: var(--tech-primary-light);
}

[data-theme="dark"] .logo h2 {
  color: #66B1FF;
}

[data-theme="dark"] .logo:hover h2 {
  color: #85C1FF;
}

.el-menu-vertical {
  border: none;
  background: transparent;
  padding: 12px 0;
}

:deep(.el-menu-item) {
  color: var(--tech-text-secondary);
  border-left: 3px solid transparent;
  margin: 6px 12px;
  border-radius: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

:deep(.el-menu-item::before) {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  background: var(--tech-primary);
  transform: scaleY(0);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

:deep(.el-menu-item:hover) {
  background: linear-gradient(90deg, rgba(64, 158, 255, 0.08) 0%, rgba(64, 158, 255, 0.03) 100%);
  color: var(--tech-primary);
  transform: translateX(2px);
}

:deep(.el-menu-item:hover::before) {
  transform: scaleY(1);
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, rgba(64, 158, 255, 0.12) 0%, rgba(64, 158, 255, 0.05) 100%);
  color: var(--tech-primary);
  font-weight: 600;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.15);
}

:deep(.el-menu-item.is-active::before) {
  transform: scaleY(1);
  box-shadow: 0 0 12px var(--tech-primary);
}

:deep(.el-menu-item .el-icon) {
  color: inherit;
  margin-right: 8px;
}

.app-header {
  background: var(--tech-bg-secondary);
  border-bottom: 1px solid var(--tech-border);
  display: flex;
  align-items: center;
  padding: 0 24px;
  height: 64px;
  position: relative;
  z-index: 1;
  box-shadow: var(--tech-shadow-md);
  transition: background-color 0.3s ease, border-color 0.3s ease;
  backdrop-filter: blur(10px);
}

.app-header::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent,
    var(--tech-primary),
    transparent
  );
  opacity: 0.3;
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  font-size: 20px;
  font-weight: 700;
  color: var(--tech-primary);
  letter-spacing: -0.02em;
  position: relative;
  transition: color 0.3s ease;
}

[data-theme="dark"] .header-title {
  color: #66B1FF;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.theme-toggle-btn {
  background: var(--tech-bg-card);
  border: 1px solid var(--tech-border);
  color: var(--tech-text-primary);
  transition: all 0.3s ease;
}

.theme-toggle-btn:hover {
  background: var(--tech-bg-glass);
  border-color: var(--tech-primary);
  color: var(--tech-primary);
  box-shadow: var(--tech-shadow-glow);
}

.header-actions :deep(.el-tag) {
  border-radius: 4px;
  font-weight: 500;
}

/* 用户信息下拉菜单样式 */
.user-info {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.3s ease;
  background: var(--tech-bg-card);
  border: 1px solid var(--tech-border);
}

.user-info:hover {
  background: var(--tech-bg-glass);
  border-color: var(--tech-primary);
}

.user-icon {
  font-size: 18px;
  color: var(--tech-primary);
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--tech-text-primary);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-icon {
  font-size: 12px;
  color: var(--tech-text-secondary);
}

.user-detail {
  display: flex;
  align-items: center;
  gap: 8px;
}


.app-main {
  background: var(--tech-bg-primary);
  padding: 20px 16px;
  position: relative;
  z-index: 1;
  overflow-y: auto;
}

.app-main.tech-scrollbar {
  scrollbar-width: thin;
  scrollbar-color: rgba(0, 255, 255, 0.3) rgba(13, 13, 13, 0.5);
}

/* 过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease, transform 0.3s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

#app {
  font-family: var(--tech-font-body);
  background: var(--tech-bg-primary);
  color: var(--tech-text-primary);
  min-height: 100vh;
}

body {
  background: var(--tech-bg-primary);
  overflow-x: hidden;
}

/* Element Plus 组件 Gin-Vue-Admin 风格覆盖 */
:deep(.el-card) {
  background: var(--tech-bg-card);
  border: 1px solid var(--tech-border);
  border-radius: 8px;
  color: var(--tech-text-primary);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: var(--tech-shadow-sm);
  position: relative;
  overflow: hidden;
}

:deep(.el-card::before) {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--tech-gradient-primary);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

:deep(.el-card:hover) {
  box-shadow: var(--tech-shadow-md);
  transform: translateY(-2px);
  border-color: rgba(64, 158, 255, 0.3);
}

:deep(.el-card:hover::before) {
  transform: scaleX(1);
}

:deep(.el-card__header) {
  border-bottom: 1px solid var(--tech-border);
  background: linear-gradient(135deg, var(--tech-bg-tertiary) 0%, var(--tech-bg-card) 100%);
  padding: 16px 20px;
  font-weight: 600;
}

:deep(.el-button) {
  border-radius: 6px;
  transition: all 0.3s ease;
  font-weight: 500;
}

:deep(.el-button--primary) {
  background: var(--tech-gradient-primary);
  border: none;
  color: #ffffff;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

:deep(.el-button--primary:hover) {
  background: linear-gradient(135deg, #66B1FF 0%, #409EFF 100%);
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.4);
  transform: translateY(-2px);
}

:deep(.el-button--primary:active) {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(64, 158, 255, 0.3);
}

:deep(.el-button--success) {
  background-color: var(--tech-secondary);
  border-color: var(--tech-secondary);
  color: #ffffff;
}

:deep(.el-input__wrapper) {
  background-color: var(--tech-bg-secondary);
  border-color: var(--tech-border);
}

:deep(.el-input__wrapper:hover) {
  border-color: var(--tech-border-active);
}

:deep(.el-input__wrapper.is-focus) {
  border-color: var(--tech-primary);
}

:deep(.el-input__inner) {
  color: var(--tech-text-primary);
}

:deep(.el-table) {
  background: transparent;
  color: var(--tech-text-primary);
}

:deep(.el-table th) {
  background-color: var(--tech-bg-tertiary);
  border-color: var(--tech-border);
  color: var(--tech-text-primary);
}

:deep(.el-table td) {
  border-color: var(--tech-border);
}

:deep(.el-table--striped .el-table__body tr.el-table__row--striped td) {
  background-color: var(--tech-bg-tertiary);
}

:deep(.el-table__row:hover) {
  background-color: var(--tech-bg-tertiary);
}
</style>
