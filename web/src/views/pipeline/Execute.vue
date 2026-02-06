<template>
  <div class="pipeline-execute">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <el-divider direction="vertical" />
        <h3>{{ pipeline?.name }}</h3>
        <el-tag :type="getStatusTag(execution?.status)" style="margin-left: 10px">
          {{ getStatusLabel(execution?.status) }}
        </el-tag>
      </div>
      <div class="toolbar-right">
        <el-button
          v-if="execution?.status === 'running'"
          type="danger"
          @click="cancelExecution"
        >
          <el-icon><CircleClose /></el-icon>
          取消执行
        </el-button>
        <el-button
          v-else-if="!execution || execution?.status === 'completed' || execution?.status === 'failed'"
          type="primary"
          @click="startExecution"
          :loading="starting"
        >
          <el-icon><VideoPlay /></el-icon>
          开始执行
        </el-button>
        <el-button @click="refreshExecution" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <!-- 执行信息 -->
    <div class="execution-info" v-if="execution">
      <el-row :gutter="20">
        <el-col :span="6">
          <div class="info-item">
            <span class="label">执行 ID</span>
            <span class="value">{{ execution.id?.slice(0, 8) }}</span>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="info-item">
            <span class="label">开始时间</span>
            <span class="value">{{ formatTime(execution.started_at) }}</span>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="info-item">
            <span class="label">持续时间</span>
            <span class="value">{{ formatDuration(execution.started_at, execution.finished_at) }}</span>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="info-item">
            <span class="label">执行进度</span>
            <span class="value">{{ executionProgress }}</span>
          </div>
        </el-col>
      </el-row>
    </div>

    <!-- 主要内容区域 -->
    <div class="execute-content">
      <!-- 左侧：DAG 执行状态 -->
      <div class="dag-status-panel">
        <div class="panel-header">
          <span>执行状态</span>
          <el-progress
            :percentage="executionProgressValue"
            :status="execution?.status === 'failed' ? 'exception' : execution?.status === 'completed' ? 'success' : ''"
            style="width: 200px"
          />
        </div>
        <div class="dag-canvas" ref="canvasRef">
          <svg
            class="dag-svg"
            :width="canvasWidth"
            :height="canvasHeight"
          >
            <g :transform="`translate(${panX}, ${panY}) scale(${scale})`">
              <!-- 连接线 -->
              <g class="connections">
                <path
                  v-for="conn in connections"
                  :key="`${conn.from}-${conn.to}`"
                  :d="getConnectionPath(conn)"
                  :class="['connection', getConnectionClass(conn)]"
                  stroke="#999"
                  stroke-width="2"
                  fill="none"
                />
              </g>

              <!-- 任务节点 -->
              <g
                v-for="node in nodes"
                :key="node.id"
                :class="['node', `node-${node.type}`, `status-${node.status}`]"
                :transform="`translate(${node.x}, ${node.y})`"
                @click="selectNode(node)"
              >
                <!-- 节点背景 -->
                <rect
                  :width="nodeWidth"
                  :height="nodeHeight"
                  :rx="8"
                  class="node-bg"
                />
                <!-- 节点名称 -->
                <text :x="10" :y="20" class="node-label">{{ node.name }}</text>
                <!-- 节点状态指示器 -->
                <circle
                  :cx="nodeWidth - 15"
                  :cy="15"
                  :r="6"
                  :class="['status-dot', `status-${node.status}`]"
                />
                <!-- 加载动画（运行中） -->
                <circle
                  v-if="node.status === 'running'"
                  :cx="nodeWidth - 15"
                  :cy="15"
                  :r="8"
                  class="loading-ring"
                />
              </g>
            </g>
          </svg>
        </div>
      </div>

      <!-- 右侧：任务日志 -->
      <div class="logs-panel">
        <div class="panel-header">
          <span>执行日志</span>
          <div class="header-actions">
            <el-button size="small" @click="clearLogs">清空</el-button>
            <el-button size="small" @click="downloadLogs">下载</el-button>
          </div>
        </div>
        <div class="logs-content" ref="logsRef">
          <div
            v-for="(log, index) in logs"
            :key="index"
            :class="['log-item', `log-${log.level}`]"
          >
            <span class="log-time">{{ formatLogTime(log.timestamp) }}</span>
            <span class="log-level">{{ log.level }}</span>
            <span class="log-task">{{ log.task_name }}</span>
            <span class="log-message">{{ log.message }}</span>
          </div>
          <el-empty v-if="logs.length === 0" description="暂无日志" :image-size="60" />
        </div>
      </div>
    </div>

    <!-- 任务详情对话框 -->
    <el-dialog
      v-model="showTaskDetail"
      :title="`任务详情: ${selectedNode?.name}`"
      width="800px"
    >
      <el-descriptions :column="2" border>
        <el-descriptions-item label="任务名称">{{ selectedNode?.name }}</el-descriptions-item>
        <el-descriptions-item label="任务类型">{{ getTaskTypeLabel(selectedNode?.type) }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="getStatusTag(selectedNode?.status)">
            {{ getStatusLabel(selectedNode?.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="开始时间">{{ formatTime(selectedNode?.started_at) }}</el-descriptions-item>
        <el-descriptions-item label="结束时间">{{ formatTime(selectedNode?.finished_at) }}</el-descriptions-item>
        <el-descriptions-item label="持续时间">{{ formatNodeDuration(selectedNode) }}</el-descriptions-item>
        <el-descriptions-item label="重试次数" :span="2">{{ selectedNode?.retry_count || 0 }}</el-descriptions-item>
        <el-descriptions-item label="错误信息" :span="2" v-if="selectedNode?.error">
          <el-alert type="error" :closable="false">{{ selectedNode.error }}</el-alert>
        </el-descriptions-item>
        <el-descriptions-item label="输出" :span="2">
          <pre class="task-output">{{ selectedNode?.output || '无输出' }}</pre>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="showTaskDetail = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowLeft, CircleClose, VideoPlay, Refresh
} from '@element-plus/icons-vue'
import api from '@/api'

const route = useRoute()
const router = useRouter()

// 数据
const pipeline = ref(null)
const execution = ref(null)
const loading = ref(false)
const starting = ref(false)
const nodes = ref([])
const connections = ref([])
const logs = ref([])
const selectedNode = ref(null)
const showTaskDetail = ref(false)

// DAG 相关
const canvasRef = ref()
const logsRef = ref()
const canvasWidth = ref(1200)
const canvasHeight = ref(800)
const panX = ref(20)
const panY = ref(20)
const scale = ref(1)
const nodeWidth = 140
const nodeHeight = 40

// 自动刷新定时器
let refreshTimer = null

// 计算执行进度
const executionProgressValue = computed(() => {
  if (!execution.value || nodes.value.length === 0) return 0
  const completed = nodes.value.filter(n => n.status === 'completed').length
  const failed = nodes.value.filter(n => n.status === 'failed').length
  return Math.round(((completed + failed) / nodes.value.length) * 100)
})

const executionProgress = computed(() => {
  const completed = nodes.value.filter(n => n.status === 'completed').length
  const failed = nodes.value.filter(n => n.status === 'failed').length
  const running = nodes.value.filter(n => n.status === 'running').length
  return `${completed + failed}/${nodes.value.length} 已完成`
})

// 加载流水线
const loadPipeline = async () => {
  const pipelineId = route.query.id
  if (!pipelineId) return

  try {
    loading.value = true
    const data = await api.getPipeline(pipelineId)
    pipeline.value = data
    buildDAG()
  } catch (error) {
    ElMessage.error('加载流水线失败')
  } finally {
    loading.value = false
  }
}

// 构建执行 DAG
const buildDAG = () => {
  const stages = pipeline.value?.stages || []
  const nodeList = []
  const connList = []

  let x = 50
  let y = 50

  stages.forEach((stage, stageIndex) => {
    const stageTasks = stage.tasks || []

    stageTasks.forEach((task, taskIndex) => {
      const node = {
        id: task.id,
        name: task.name,
        type: task.type,
        x: x + taskIndex * (nodeWidth + 20),
        y: y + stageIndex * (nodeHeight + 60),
        status: 'pending',
        started_at: null,
        finished_at: null,
        retry_count: 0,
        output: null,
        error: null
      }
      nodeList.push(node)

      // 添加依赖连接
      if (task.depends_on) {
        task.depends_on.forEach(depId => {
          connList.push({ from: depId, to: task.id })
        })
      }
    })
  })

  nodes.value = nodeList
  connections.value = connList
}

// 获取连接线路径
const getConnectionPath = (conn) => {
  const fromNode = nodes.value.find(n => n.id === conn.from)
  const toNode = nodes.value.find(n => n.id === conn.to)

  if (!fromNode || !toNode) return ''

  const x1 = fromNode.x + nodeWidth / 2
  const y1 = fromNode.y + nodeHeight
  const x2 = toNode.x + nodeWidth / 2
  const y2 = toNode.y

  const midY = (y1 + y2) / 2
  return `M ${x1} ${y1} C ${x1} ${midY}, ${x2} ${midY}, ${x2} ${y2}`
}

// 获取连接线样式类
const getConnectionClass = (conn) => {
  const toNode = nodes.value.find(n => n.id === conn.to)
  if (!toNode) return ''

  const status = toNode.status
  if (status === 'completed') return 'connection-success'
  if (status === 'failed') return 'connection-failed'
  if (status === 'running') return 'connection-running'
  return ''
}

// 选择节点查看详情
const selectNode = (node) => {
  selectedNode.value = node
  showTaskDetail.value = true
}

// 开始执行
const startExecution = async () => {
  try {
    await ElMessageBox.confirm('确定要开始执行此流水线吗？', '确认执行', {
      type: 'warning'
    })

    starting.value = true
    const data = await api.startRelease(route.query.id)
    execution.value = data

    ElMessage.success('开始执行')
    startAutoRefresh()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '启动失败')
    }
  } finally {
    starting.value = false
  }
}

// 取消执行
const cancelExecution = async () => {
  try {
    await ElMessageBox.confirm('确定要取消执行吗？', '确认取消', {
      type: 'warning'
    })

    await api.cancelRelease(route.query.id)
    ElMessage.success('已取消执行')
    stopAutoRefresh()
    refreshExecution()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '取消失败')
    }
  }
}

// 刷新执行状态
const refreshExecution = async () => {
  const releaseId = route.query.releaseId
  if (!releaseId) return

  try {
    loading.value = true
    const data = await api.getRelease(releaseId)
    execution.value = data

    // 更新节点状态
    if (data.task_instances) {
      data.task_instances.forEach(instance => {
        const node = nodes.value.find(n => n.id === instance.task_id)
        if (node) {
          node.status = instance.status
          node.started_at = instance.started_at
          node.finished_at = instance.finished_at
          node.retry_count = instance.retry_count
          node.output = instance.output
          node.error = instance.error
        }
      })
    }

    // 更新日志
    if (data.logs) {
      logs.value = data.logs
      scrollToBottom()
    }

    // 如果执行完成，停止自动刷新
    if (data.status === 'completed' || data.status === 'failed' || data.status === 'cancelled') {
      stopAutoRefresh()
    }
  } catch (error) {
    console.error('刷新执行状态失败', error)
  } finally {
    loading.value = false
  }
}

// 启动自动刷新
const startAutoRefresh = () => {
  stopAutoRefresh()
  refreshTimer = setInterval(() => {
    refreshExecution()
  }, 2000)
}

// 停止自动刷新
const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

// 清空日志
const clearLogs = () => {
  logs.value = []
}

// 下载日志
const downloadLogs = () => {
  const content = logs.value.map(log =>
    `[${formatLogTime(log.timestamp)}] [${log.level}] [${log.task_name}] ${log.message}`
  ).join('\n')

  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `pipeline-${execution.value?.id?.slice(0, 8)}-${Date.now()}.log`
  link.click()
  URL.revokeObjectURL(url)
}

// 滚动到底部
const scrollToBottom = () => {
  setTimeout(() => {
    if (logsRef.value) {
      logsRef.value.scrollTop = logsRef.value.scrollHeight
    }
  }, 100)
}

// 格式化时间
const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

const formatLogTime = (timestamp) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  return date.toLocaleTimeString('zh-CN', { hour12: false }) + '.' +
         String(date.getMilliseconds()).padStart(3, '0')
}

// 格式化持续时间
const formatDuration = (start, end) => {
  if (!start) return '-'
  const startTime = new Date(start).getTime()
  const endTime = end ? new Date(end).getTime() : Date.now()
  const duration = Math.floor((endTime - startTime) / 1000)

  if (duration < 60) return `${duration}秒`
  if (duration < 3600) return `${Math.floor(duration / 60)}分${duration % 60}秒`
  return `${Math.floor(duration / 3600)}时${Math.floor((duration % 3600) / 60)}分`
}

const formatNodeDuration = (node) => {
  return formatDuration(node.started_at, node.finished_at)
}

// 获取状态标签
const getStatusTag = (status) => {
  const map = {
    pending: 'info',
    running: 'warning',
    completed: 'success',
    failed: 'danger',
    cancelled: 'info'
  }
  return map[status] || ''
}

const getStatusLabel = (status) => {
  const map = {
    pending: '待执行',
    running: '执行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消'
  }
  return map[status] || status
}

const getTaskTypeLabel = (type) => {
  const map = {
    shell: 'Shell 脚本',
    http: 'HTTP 请求',
    delay: '延迟',
    condition: '条件判断'
  }
  return map[type] || type
}

// 返回
const goBack = () => {
  stopAutoRefresh()
  router.push('/pipeline')
}

// 初始化
onMounted(() => {
  loadPipeline()

  // 如果有 releaseId，说明是查看已有执行
  if (route.query.releaseId) {
    refreshExecution()
    // 如果执行中，启动自动刷新
    const checkStatus = setInterval(() => {
      if (execution.value?.status === 'running') {
        startAutoRefresh()
        clearInterval(checkStatus)
      }
    }, 1000)
  }
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.pipeline-execute {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 20px;
  border-bottom: 1px solid #eee;
  background: #fff;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.execution-info {
  padding: 15px 20px;
  background: #f5f7fa;
  border-bottom: 1px solid #eee;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.info-item .label {
  font-size: 12px;
  color: #999;
}

.info-item .value {
  font-size: 14px;
  font-weight: 500;
  color: #333;
}

.execute-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.dag-status-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #eee;
  background: #fff;
}

.logs-panel {
  width: 400px;
  min-width: 400px;
  display: flex;
  flex-direction: column;
  background: #fff;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px;
  border-bottom: 1px solid #eee;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 5px;
}

.dag-canvas {
  flex: 1;
  overflow: auto;
  padding: 20px;
  background: #f5f7fa;
}

.dag-svg {
  display: block;
}

.node {
  cursor: pointer;
  transition: all 0.2s;
}

.node:hover {
  filter: brightness(0.95);
}

.node-bg {
  fill: #fff;
  stroke: #ddd;
  stroke-width: 1;
}

.node.status-pending .node-bg { stroke: #909399; }
.node.status-running .node-bg { stroke: #409eff; fill: #ecf5ff; }
.node.status-completed .node-bg { stroke: #67c23a; fill: #f0f9ff; }
.node.status-failed .node-bg { stroke: #f56c6c; fill: #fef0f0; }

.node-label {
  font-size: 12px;
  fill: #333;
  pointer-events: none;
}

.status-dot {
  stroke: #fff;
  stroke-width: 1;
}

.status-pending .status-dot { fill: #909399; }
.status-running .status-dot { fill: #409eff; }
.status-completed .status-dot { fill: #67c23a; }
.status-failed .status-dot { fill: #f56c6c; }

.loading-ring {
  stroke: #409eff;
  stroke-width: 2;
  fill: none;
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; r: 8; }
  50% { opacity: 0.5; r: 10; }
}

.connection {
  transition: stroke 0.3s;
}

.connection-success {
  stroke: #67c23a !important;
}

.connection-failed {
  stroke: #f56c6c !important;
}

.connection-running {
  stroke: #409eff !important;
  stroke-dasharray: 5, 5;
  animation: dash 1s linear infinite;
}

@keyframes dash {
  to { stroke-dashoffset: -10; }
}

.logs-content {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  background: #1e1e1e;
  color: #d4d4d4;
}

.log-item {
  padding: 4px 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-item:hover {
  background: #2d2d2d;
}

.log-time {
  color: #858585;
  margin-right: 8px;
}

.log-level {
  margin-right: 8px;
  font-weight: 600;
}

.log-info .log-level { color: #4fc3f7; }
.log-warn .log-level { color: #ffa726; }
.log-error .log-level { color: #ef5350; }

.log-task {
  color: #66bb6a;
  margin-right: 8px;
}

.log-message {
  color: #d4d4d4;
}

.task-output {
  margin: 0;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 4px;
  font-size: 12px;
  max-height: 300px;
  overflow-y: auto;
}
</style>
