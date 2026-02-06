<template>
  <div class="project-overview">
    <!-- 项目头部 -->
    <div class="project-header">
      <div class="project-info">
        <el-icon class="project-icon" :color="getProjectColor(project?.type)" :size="40">
          <component :is="getProjectIcon(project?.type)" />
        </el-icon>
        <div>
          <h2>{{ project?.name }}</h2>
          <p class="project-meta">
            <el-tag :type="getProjectTypeTag(project?.type)" size="small">
              {{ getProjectTypeLabel(project?.type) }}
            </el-tag>
            <span class="description">{{ project?.description || '暂无描述' }}</span>
          </p>
        </div>
      </div>
      <div class="header-actions">
        <el-button @click="editProject">
          <el-icon><Edit /></el-icon>
          编辑
        </el-button>
        <el-button @click="refresh" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <!-- 快捷入口 -->
    <el-row :gutter="20" class="quick-actions">
      <el-col :span="6">
        <el-card shadow="hover" class="quick-card" @click="goToConfig">
          <div class="quick-content">
            <el-icon class="quick-icon" color="#409eff" :size="32"><Setting /></el-icon>
            <div class="quick-info">
              <div class="quick-title">配置中心</div>
              <div class="quick-desc">{{ stats.configCount || 0 }} 项配置</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="quick-card" @click="goToPipeline">
          <div class="quick-content">
            <el-icon class="quick-icon" color="#67c23a" :size="32"><Operation /></el-icon>
            <div class="quick-info">
              <div class="quick-title">流水线</div>
              <div class="quick-desc">{{ stats.pipelineCount || 0 }} 条流水线</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="quick-card" @click="goToAlerts">
          <div class="quick-content">
            <el-icon class="quick-icon" color="#f56c6c" :size="32"><Warning /></el-icon>
            <div class="quick-info">
              <div class="quick-title">告警规则</div>
              <div class="quick-desc">{{ stats.alertRuleCount || 0 }} 条规则</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="quick-card" @click="goToDeployments">
          <div class="quick-content">
            <el-icon class="quick-icon" color="#e6a23c" :size="32"><Upload /></el-icon>
            <div class="quick-info">
              <div class="quick-title">部署记录</div>
              <div class="quick-desc">{{ stats.deploymentCount || 0 }} 次部署</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 详细信息 -->
    <el-row :gutter="20" class="detail-section">
      <!-- 最近执行 -->
      <el-col :span="12">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>最近执行</span>
              <el-button text @click="goToExecutions">查看全部</el-button>
            </div>
          </template>
          <el-empty v-if="!recentExecutions || recentExecutions.length === 0" description="暂无执行记录" :image-size="60" />
          <div v-else class="execution-list">
            <div
              v-for="exec in recentExecutions"
              :key="exec.id"
              class="execution-item"
              @click="viewExecution(exec)"
            >
              <div class="execution-info">
                <div class="execution-name">{{ exec.pipeline_name }}</div>
                <div class="execution-meta">
                  <el-tag :type="getStatusTag(exec.status)" size="small">
                    {{ getStatusLabel(exec.status) }}
                  </el-tag>
                  <span class="execution-time">{{ formatTime(exec.created_at) }}</span>
                </div>
              </div>
              <el-icon class="arrow-icon"><ArrowRight /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>

      <!-- 活跃告警 -->
      <el-col :span="12">
        <el-card shadow="never">
          <template #header>
            <div class="card-header">
              <span>活跃告警</span>
              <el-button text @click="goToAlerts">查看全部</el-button>
            </div>
          </template>
          <el-empty v-if="!activeAlerts || activeAlerts.length === 0" description="暂无活跃告警" :image-size="60" />
          <div v-else class="alert-list">
            <div
              v-for="alert in activeAlerts"
              :key="alert.id"
              class="alert-item"
              :class="`alert-${alert.severity}`"
            >
              <div class="alert-info">
                <div class="alert-name">{{ alert.rule_name }}</div>
                <div class="alert-message">{{ alert.message }}</div>
                <div class="alert-time">{{ formatTime(alert.fired_at) }}</div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Edit, Refresh, Setting, Operation, Warning, Upload, ArrowRight
} from '@element-plus/icons-vue'
import api from '@/api'
import { useUserStore } from '@/stores/user'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

// 数据
const project = ref(null)
const loading = ref(false)
const stats = ref({})
const recentExecutions = ref([])
const activeAlerts = ref([])

// 当前项目ID
const projectId = computed(() => route.query.projectId)

// 加载项目数据
const loadProject = async () => {
  if (!projectId.value) return

  try {
    loading.value = true
    const [projectData, statsData] = await Promise.all([
      api.getProject(projectId.value),
      api.getProjectStats(projectId.value)
    ])
    project.value = projectData
    stats.value = statsData

    // 加载最近执行和活跃告警
    const [executionsData, alertsData] = await Promise.all([
      api.getDeployTasks(projectId.value, { limit: 5 }).catch(() => []),
      api.getActiveAlerts().catch(() => [])
    ])
    recentExecutions.value = executionsData || []
    activeAlerts.value = (alertsData || []).filter(a => a.project_id === projectId.value)
  } catch (error) {
    ElMessage.error('加载项目数据失败')
  } finally {
    loading.value = false
  }
}

// 刷新
const refresh = () => {
  loadProject()
}

// 导航方法
const goToConfig = () => {
  router.push({
    path: '/project/config',
    query: { projectId: projectId.value }
  })
}

const goToPipeline = () => {
  router.push({
    path: '/project/pipeline',
    query: { projectId: projectId.value }
  })
}

const goToAlerts = () => {
  router.push({
    path: '/project/alerts',
    query: { projectId: projectId.value }
  })
}

const goToDeployments = () => {
  router.push({
    path: '/project/deployments',
    query: { projectId: projectId.value }
  })
}

const goToExecutions = () => {
  router.push({
    path: '/project/executions',
    query: { projectId: projectId.value }
  })
}

const viewExecution = (exec) => {
  router.push({
    path: '/project/executions',
    query: {
      projectId: projectId.value,
      executionId: exec.id
    }
  })
}

const editProject = () => {
  ElMessage.info('项目编辑功能开发中')
}

// 辅助方法
const getProjectTypeTag = (type) => {
  const map = {
    deploy: '',
    operations: 'success',
    cicd: 'warning',
    custom: 'info'
  }
  return map[type] || ''
}

const getProjectTypeLabel = (type) => {
  const map = {
    deploy: '部署',
    operations: '运维',
    cicd: 'CI/CD',
    custom: '自定义'
  }
  return map[type] || type
}

const getProjectIcon = (type) => {
  const map = {
    deploy: 'Upload',
    operations: 'Tools',
    cicd: 'Timer',
    custom: 'Folder'
  }
  return map[type] || 'Folder'
}

const getProjectColor = (type) => {
  const map = {
    deploy: '#409eff',
    operations: '#67c23a',
    cicd: '#e6a23c',
    custom: '#909399'
  }
  return map[type] || '#909399'
}

const getStatusTag = (status) => {
  const map = {
    pending: 'info',
    running: 'warning',
    success: 'success',
    failed: 'danger',
    cancelled: 'info'
  }
  return map[status] || ''
}

const getStatusLabel = (status) => {
  const map = {
    pending: '待执行',
    running: '执行中',
    success: '成功',
    failed: '失败',
    cancelled: '已取消'
  }
  return map[status] || status
}

const formatTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  const now = new Date()
  const diff = now - date

  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  return date.toLocaleDateString()
}

// 初始化
onMounted(() => {
  loadProject()
})
</script>

<style scoped>
.project-overview {
  padding: 20px;
}

.project-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.project-info {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.project-info h2 {
  margin: 0 0 8px 0;
  font-size: 20px;
  font-weight: 600;
  color: #303133;
}

.project-meta {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  color: #909399;
  font-size: 14px;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.quick-actions {
  margin-bottom: 20px;
}

.quick-card {
  cursor: pointer;
  transition: all 0.3s;
}

.quick-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.quick-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.quick-info {
  flex: 1;
}

.quick-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 4px;
}

.quick-desc {
  font-size: 13px;
  color: #909399;
}

.detail-section {
  margin-top: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.execution-list,
.alert-list {
  max-height: 400px;
  overflow-y: auto;
}

.execution-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.execution-item:hover {
  border-color: #409eff;
  background: #f5f7fa;
}

.execution-name {
  font-weight: 500;
  color: #303133;
  margin-bottom: 4px;
}

.execution-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: #909399;
}

.execution-time {
  color: #909399;
}

.alert-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 6px;
  border-left: 3px solid;
}

.alert-critical {
  background: #fef0f0;
  border-left-color: #f56c6c;
}

.alert-warning {
  background: #fdf6ec;
  border-left-color: #e6a23c;
}

.alert-info {
  background: #f4f4f5;
  border-left-color: #909399;
}

.alert-name {
  font-weight: 500;
  color: #303133;
  margin-bottom: 4px;
}

.alert-message {
  font-size: 13px;
  color: #606266;
  margin-bottom: 4px;
}

.alert-time {
  font-size: 12px;
  color: #909399;
}
</style>
