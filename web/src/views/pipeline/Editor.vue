<template>
  <div class="pipeline-editor">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <el-divider direction="vertical" />
        <h3>{{ pipeline?.name || '新建流水线' }}</h3>
      </div>
      <div class="toolbar-right">
        <el-button @click="validatePipeline" :loading="validating">
          <el-icon><CircleCheck /></el-icon>
          验证
        </el-button>
        <el-button type="primary" @click="savePipeline" :loading="saving">
          <el-icon><Select /></el-icon>
          保存
        </el-button>
      </div>
    </div>

    <!-- 主要内容区域 -->
    <div class="editor-content">
      <!-- 左侧：阶段列表 -->
      <div class="stages-panel">
        <div class="panel-header">
          <span>阶段列表</span>
          <el-button type="primary" size="small" @click="addStage">
            <el-icon><Plus /></el-icon>
            添加阶段
          </el-button>
        </div>
        <div class="stages-list">
          <draggable
            v-model="stages"
            item-key="id"
            @end="onStageReorder"
          >
            <template #item="{ element: stage, index }">
              <div
                class="stage-item"
                :class="{ active: selectedStageId === stage.id }"
                @click="selectStage(stage)"
              >
                <div class="stage-header">
                  <span class="stage-name">阶段 {{ index + 1 }}: {{ stage.name }}</span>
                  <el-dropdown @command="handleStageAction($event, stage)" @click.stop>
                    <el-icon class="more-icon"><MoreFilled /></el-icon>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">编辑</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
                <div class="stage-tasks">
                  <el-tag
                    v-for="task in stage.tasks"
                    :key="task.id"
                    size="small"
                    :type="getTaskTypeTag(task.type)"
                  >
                    {{ task.name }}
                  </el-tag>
                </div>
              </div>
            </template>
          </draggable>
          <el-empty v-if="stages.length === 0" description="暂无阶段" :image-size="60" />
        </div>
      </div>

      <!-- 中间：DAG 可视化 -->
      <div class="dag-panel">
        <div class="dag-canvas" ref="canvasRef">
          <svg
            class="dag-svg"
            :width="canvasWidth"
            :height="canvasHeight"
            @mousedown="onCanvasMouseDown"
            @mousemove="onCanvasMouseMove"
            @mouseup="onCanvasMouseUp"
            @wheel="onCanvasWheel"
          >
            <g :transform="`translate(${panX}, ${panY}) scale(${scale})`">
              <!-- 连接线 -->
              <g class="connections">
                <path
                  v-for="conn in connections"
                  :key="`${conn.from}-${conn.to}`"
                  :d="getConnectionPath(conn)"
                  :class="['connection', { selected: selectedConnection === conn }]"
                  @click="selectConnection(conn)"
                  stroke="#999"
                  stroke-width="2"
                  fill="none"
                />
              </g>

              <!-- 任务节点 -->
              <g
                v-for="node in nodes"
                :key="node.id"
                :class="['node', `node-${node.type}`, { selected: selectedNodeId === node.id }]"
                :transform="`translate(${node.x}, ${node.y})`"
                @mousedown="onNodeMouseDown($event, node)"
                @click="selectNode(node)"
              >
                <!-- 节点背景 -->
                <rect
                  :width="nodeWidth"
                  :height="nodeHeight"
                  :rx="8"
                  class="node-bg"
                />
                <!-- 节点图标 -->
                <text :x="10" :y="25" class="node-icon">{{ getTaskIcon(node.type) }}</text>
                <!-- 节点名称 -->
                <text :x="40" :y="25" class="node-label">{{ node.name }}</text>
                <!-- 节点状态指示器 -->
                <circle
                  v-if="node.status"
                  :cx="nodeWidth - 15"
                  :cy="15"
                  :r="6"
                  :class="['status-dot', `status-${node.status}`]"
                />
                <!-- 输入端口 -->
                <circle
                  v-if="node.hasInput"
                  :cx="nodeWidth / 2"
                  :cy="0"
                  :r="6"
                  class="port input-port"
                  @mousedown.stop="onPortMouseDown($event, node, 'input')"
                />
                <!-- 输出端口 -->
                <circle
                  v-if="node.hasOutput"
                  :cx="nodeWidth / 2"
                  :cy="nodeHeight"
                  :r="6"
                  class="port output-port"
                  @mousedown.stop="onPortMouseDown($event, node, 'output')"
                />
              </g>

              <!-- 正在创建的连接线 -->
              <path
                v-if="creatingConnection"
                :d="getCreatingConnectionPath()"
                class="connection creating"
                stroke="#409eff"
                stroke-width="2"
                stroke-dasharray="5,5"
                fill="none"
              />
            </g>
          </svg>
        </div>

        <!-- DAG 工具栏 -->
        <div class="dag-toolbar">
          <el-button-group>
            <el-button size="small" @click="zoomIn">
              <el-icon><ZoomIn /></el-icon>
            </el-button>
            <el-button size="small" @click="zoomOut">
              <el-icon><ZoomOut /></el-icon>
            </el-button>
            <el-button size="small" @click="resetView">
              <el-icon><FullScreen /></el-icon>
            </el-button>
          </el-button-group>
          <el-button-group style="margin-left: 10px">
            <el-button size="small" @click="autoLayout">
              <el-icon><MagicStick /></el-icon>
              自动布局
            </el-button>
            <el-button size="small" @click="fitToScreen">
              <el-icon><Crop /></el-icon>
              适应屏幕
            </el-button>
          </el-button-group>
        </div>
      </div>

      <!-- 右侧：任务配置 -->
      <div class="config-panel">
        <div class="panel-header">
          <span>任务配置</span>
          <el-button
            v-if="selectedNode"
            type="primary"
            size="small"
            @click="editTaskConfig"
          >
            编辑
          </el-button>
        </div>
        <div class="config-content" v-if="selectedNode">
          <el-form label-width="80px" size="small">
            <el-form-item label="任务名称">
              <el-input v-model="selectedNode.name" disabled />
            </el-form-item>
            <el-form-item label="任务类型">
              <el-tag :type="getTaskTypeTag(selectedNode.type)">
                {{ getTaskTypeLabel(selectedNode.type) }}
              </el-tag>
            </el-form-item>
            <el-form-item label="超时时间">
              <span>{{ selectedNode.timeout || 300 }} 秒</span>
            </el-form-item>
            <el-form-item label="重试次数">
              <span>{{ selectedNode.retry || 0 }} 次</span>
            </el-form-item>
            <el-form-item label="依赖任务" v-if="selectedNode.dependsOn?.length">
              <el-tag
                v-for="depId in selectedNode.dependsOn"
                :key="depId"
                size="small"
                style="margin: 2px"
              >
                {{ getNodeName(depId) }}
              </el-tag>
            </el-form-item>
          </el-form>
        </div>
        <el-empty v-else description="请选择任务节点" :image-size="60" />
      </div>
    </div>

    <!-- 添加阶段对话框 -->
    <el-dialog v-model="showStageDialog" title="添加阶段" width="500px">
      <el-form :model="stageForm" :rules="stageRules" ref="stageFormRef" label-width="100px">
        <el-form-item label="阶段名称" prop="name">
          <el-input v-model="stageForm.name" placeholder="请输入阶段名称" />
        </el-form-item>
        <el-form-item label="并行执行" prop="parallel">
          <el-switch v-model="stageForm.parallel" />
          <span style="margin-left: 10px; color: #999; font-size: 12px">
            开启后阶段内任务将并行执行
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showStageDialog = false">取消</el-button>
        <el-button type="primary" @click="confirmAddStage">确定</el-button>
      </template>
    </el-dialog>

    <!-- 添加任务对话框 -->
    <el-dialog v-model="showTaskDialog" title="添加任务" width="600px">
      <el-form :model="taskForm" :rules="taskRules" ref="taskFormRef" label-width="100px">
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="taskForm.name" placeholder="请输入任务名称" />
        </el-form-item>
        <el-form-item label="任务类型" prop="type">
          <el-select v-model="taskForm.type" placeholder="请选择任务类型">
            <el-option label="Shell 脚本" value="shell" />
            <el-option label="HTTP 请求" value="http" />
            <el-option label="延迟" value="delay" />
            <el-option label="条件判断" value="condition" />
          </el-select>
        </el-form-item>
        <el-form-item label="超时时间" prop="timeout">
          <el-input-number v-model="taskForm.timeout" :min="30" :max="3600" />
          <span style="margin-left: 10px">秒</span>
        </el-form-item>
        <el-form-item label="重试次数" prop="retry">
          <el-input-number v-model="taskForm.retry" :min="0" :max="5" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showTaskDialog = false">取消</el-button>
        <el-button type="primary" @click="confirmAddTask">确定</el-button>
      </template>
    </el-dialog>

    <!-- 任务配置对话框 -->
    <el-dialog
      v-model="showConfigDialog"
      :title="`配置任务: ${selectedNode?.name}`"
      width="700px"
    >
      <MonacoEditor
        v-model="taskConfig"
        language="json"
        :height="300"
        :options="{ minimap: { enabled: false } }"
      />
      <template #footer>
        <el-button @click="showConfigDialog = false">取消</el-button>
        <el-button type="primary" @click="saveTaskConfig">保存配置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ArrowLeft, CircleCheck, Select, Plus, MoreFilled,
  ZoomIn, ZoomOut, FullScreen, MagicStick, Crop
} from '@element-plus/icons-vue'
import draggable from 'vuedraggable'
import MonacoEditor from '@/components/MonacoEditor.vue'
import api from '@/api'

const route = useRoute()
const router = useRouter()

// 数据
const pipeline = ref(null)
const stages = ref([])
const loading = ref(false)
const saving = ref(false)
const validating = ref(false)
const selectedStageId = ref(null)
const selectedNodeId = ref(null)
const selectedConnection = ref(null)
const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  for (const stage of stages.value) {
    const task = stage.tasks?.find(t => t.id === selectedNodeId.value)
    if (task) return task
  }
  return null
})

// DAG 相关
const canvasRef = ref()
const canvasWidth = ref(2000)
const canvasHeight = ref(1500)
const panX = ref(0)
const panY = ref(0)
const scale = ref(1)
const nodeWidth = 160
const nodeHeight = 50
const nodes = ref([])
const connections = ref([])
const creatingConnection = ref(null)
const isDragging = ref(false)
const dragTarget = ref(null)
const dragStartPos = ref({ x: 0, y: 0 })

// 对话框
const showStageDialog = ref(false)
const showTaskDialog = ref(false)
const showConfigDialog = ref(false)
const stageFormRef = ref()
const taskFormRef = ref()

// 表单数据
const stageForm = ref({
  name: '',
  parallel: false
})

const taskForm = ref({
  name: '',
  type: 'shell',
  timeout: 300,
  retry: 0
})

const taskConfig = ref('')

// 验证规则
const stageRules = {
  name: [{ required: true, message: '请输入阶段名称', trigger: 'blur' }]
}

const taskRules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择任务类型', trigger: 'change' }]
}

// 加载流水线
const loadPipeline = async () => {
  const pipelineId = route.query.id
  if (!pipelineId) return

  try {
    loading.value = true
    const data = await api.getPipeline(pipelineId)
    pipeline.value = data
    stages.value = data.stages || []
    buildDAG()
  } catch (error) {
    ElMessage.error('加载流水线失败')
  } finally {
    loading.value = false
  }
}

// 构建 DAG
const buildDAG = () => {
  const nodeList = []
  const connList = []

  let x = 50
  let y = 50

  stages.value.forEach((stage, stageIndex) => {
    const stageTasks = stage.tasks || []

    stageTasks.forEach((task, taskIndex) => {
      const node = {
        id: task.id,
        name: task.name,
        type: task.type,
        x: x + taskIndex * (nodeWidth + 30),
        y: y + stageIndex * (nodeHeight + 80),
        timeout: task.timeout,
        retry: task.retry,
        dependsOn: task.depends_on || [],
        config: task.config,
        hasInput: taskIndex > 0 || stageIndex > 0,
        hasOutput: taskIndex < stageTasks.length - 1 || stageIndex < stages.value.length - 1,
        status: null
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

// 获取正在创建的连接线路径
const getCreatingConnectionPath = () => {
  if (!creatingConnection.value) return ''

  const node = creatingConnection.value.node
  const x = node.x + nodeWidth / 2
  const y = creatingConnection.value.port === 'output' ? node.y + nodeHeight : node.y

  const mouseX = creatingConnection.value.mouseX
  const mouseY = creatingConnection.value.mouseY

  return `M ${x} ${y} L ${mouseX} ${mouseY}`
}

// 选择节点
const selectNode = (node) => {
  selectedNodeId.value = node.id
  selectedConnection.value = null
}

// 选择连接
const selectConnection = (conn) => {
  selectedConnection.value = conn
  selectedNodeId.value = null
}

// 画布鼠标事件
const onCanvasMouseDown = () => {
  selectedNodeId.value = null
  selectedConnection.value = null
}

const onCanvasMouseMove = (e) => {
  if (creatingConnection.value) {
    const rect = canvasRef.value.getBoundingClientRect()
    creatingConnection.value.mouseX = (e.clientX - rect.left - panX.value) / scale.value
    creatingConnection.value.mouseY = (e.clientY - rect.top - panY.value) / scale.value
  }

  if (isDragging.value && dragTarget.value) {
    const dx = e.clientX - dragStartPos.value.x
    const dy = e.clientY - dragStartPos.value.y
    dragTarget.value.x += dx
    dragTarget.value.y += dy
    dragStartPos.value = { x: e.clientX, y: e.clientY }
  }
}

const onCanvasMouseUp = () => {
  isDragging.value = false
  dragTarget.value = null
  creatingConnection.value = null
}

// 节点拖动
const onNodeMouseDown = (e, node) => {
  isDragging.value = true
  dragTarget.value = node
  dragStartPos.value = { x: e.clientX, y: e.clientY }
}

// 端口连接
const onPortMouseDown = (e, node, portType) => {
  const rect = canvasRef.value.getBoundingClientRect()
  creatingConnection.value = {
    node,
    port: portType,
    mouseX: (e.clientX - rect.left - panX.value) / scale.value,
    mouseY: (e.clientY - rect.top - panY.value) / scale.value
  }
}

// 缩放
const onCanvasWheel = (e) => {
  e.preventDefault()
  const delta = e.deltaY > 0 ? 0.9 : 1.1
  scale.value = Math.max(0.1, Math.min(3, scale.value * delta))
}

const zoomIn = () => {
  scale.value = Math.min(3, scale.value * 1.2)
}

const zoomOut = () => {
  scale.value = Math.max(0.1, scale.value / 1.2)
}

const resetView = () => {
  scale.value = 1
  panX.value = 0
  panY.value = 0
}

// 自动布局
const autoLayout = () => {
  // 简单的分层布局
  const levels = []
  const visited = new Set()

  const getLevel = (nodeId, level = 0) => {
    if (visited.has(nodeId)) return
    visited.add(nodeId)

    const node = nodes.value.find(n => n.id === nodeId)
    if (!node) return

    if (!levels[level]) levels[level] = []
    levels[level].push(node)

    const deps = node.dependsOn || []
    deps.forEach(depId => {
      const depNode = nodes.value.find(n => n.id === depId)
      if (depNode) getLevel(depId, level - 1)
    })
  }

  nodes.value.forEach(node => getLevel(node.id))

  let y = 50
  levels.forEach((levelNodes, index) => {
    const totalWidth = levelNodes.length * (nodeWidth + 30) - 30
    let x = (canvasWidth.value - totalWidth) / 2

    levelNodes.forEach(node => {
      node.x = x
      node.y = y
      x += nodeWidth + 30
    })

    y += nodeHeight + 100
  })
}

// 适应屏幕
const fitToScreen = () => {
  if (nodes.value.length === 0) return

  let minX = Infinity, minY = Infinity
  let maxX = -Infinity, maxY = -Infinity

  nodes.value.forEach(node => {
    minX = Math.min(minX, node.x)
    minY = Math.min(minY, node.y)
    maxX = Math.max(maxX, node.x + nodeWidth)
    maxY = Math.max(maxY, node.y + nodeHeight)
  })

  const width = maxX - minX + 100
  const height = maxY - minY + 100

  scale.value = Math.min(
    canvasWidth.value / width,
    canvasHeight.value / height,
    1
  )

  panX.value = (canvasWidth.value - width * scale.value) / 2 - minX * scale.value
  panY.value = (canvasHeight.value - height * scale.value) / 2 - minY * scale.value
}

// 阶段操作
const addStage = () => {
  stageForm.value = { name: '', parallel: false }
  showStageDialog.value = true
}

const confirmAddStage = async () => {
  await stageFormRef.value.validate()
  const stage = {
    id: `stage-${Date.now()}`,
    name: stageForm.value.name,
    parallel: stageForm.value.parallel,
    tasks: []
  }
  stages.value.push(stage)
  showStageDialog.value = false
  buildDAG()
}

const selectStage = (stage) => {
  selectedStageId.value = stage.id
}

const handleStageAction = (command, stage) => {
  if (command === 'edit') {
    stageForm.value = { name: stage.name, parallel: stage.parallel }
    showStageDialog.value = true
  } else if (command === 'delete') {
    const index = stages.value.findIndex(s => s.id === stage.id)
    if (index > -1) {
      stages.value.splice(index, 1)
      buildDAG()
    }
  }
}

const onStageReorder = () => {
  buildDAG()
}

// 添加任务到选中阶段
const addTask = () => {
  if (!selectedStageId.value) {
    ElMessage.warning('请先选择阶段')
    return
  }
  taskForm.value = { name: '', type: 'shell', timeout: 300, retry: 0 }
  showTaskDialog.value = true
}

const confirmAddTask = async () => {
  await taskFormRef.value.validate()
  const stage = stages.value.find(s => s.id === selectedStageId.value)
  if (!stage) return

  const task = {
    id: `task-${Date.now()}`,
    name: taskForm.value.name,
    type: taskForm.value.type,
    timeout: taskForm.value.timeout,
    retry: taskForm.value.retry,
    config: {},
    depends_on: []
  }

  if (!stage.tasks) stage.tasks = []
  stage.tasks.push(task)
  showTaskDialog.value = false
  buildDAG()
}

// 编辑任务配置
const editTaskConfig = () => {
  if (!selectedNode.value) return
  taskConfig.value = JSON.stringify(selectedNode.value.config || {}, null, 2)
  showConfigDialog.value = true
}

const saveTaskConfig = () => {
  try {
    const config = JSON.parse(taskConfig.value)
    selectedNode.value.config = config
    showConfigDialog.value = false
    ElMessage.success('配置保存成功')
  } catch (error) {
    ElMessage.error('配置格式错误，请检查 JSON 格式')
  }
}

// 获取节点名称
const getNodeName = (nodeId) => {
  const node = nodes.value.find(n => n.id === nodeId)
  return node?.name || nodeId
}

// 获取任务类型标签
const getTaskTypeTag = (type) => {
  const map = {
    shell: '',
    http: 'success',
    delay: 'warning',
    condition: 'info'
  }
  return map[type] || ''
}

const getTaskTypeLabel = (type) => {
  const map = {
    shell: 'Shell',
    http: 'HTTP',
    delay: '延迟',
    condition: '条件'
  }
  return map[type] || type
}

const getTaskIcon = (type) => {
  const map = {
    shell: '⚙️',
    http: '🌐',
    delay: '⏱️',
    condition: '🔀'
  }
  return map[type] || '📦'
}

// 验证流水线
const validatePipeline = async () => {
  validating.value = true
  try {
    // 检查是否有环
    const checkCycle = (nodeId, visited = new Set(), recStack = new Set()) => {
      if (recStack.has(nodeId)) return true
      if (visited.has(nodeId)) return false

      visited.add(nodeId)
      recStack.add(nodeId)

      const node = nodes.value.find(n => n.id === nodeId)
      if (node?.dependsOn) {
        for (const depId of node.dependsOn) {
          if (checkCycle(depId, visited, recStack)) return true
        }
      }

      recStack.delete(nodeId)
      return false
    }

    let hasCycle = false
    for (const node of nodes.value) {
      if (checkCycle(node.id)) {
        hasCycle = true
        break
      }
    }

    if (hasCycle) {
      ElMessage.error('流水线存在循环依赖，请检查任务依赖关系')
      return
    }

    ElMessage.success('流水线验证通过')
  } finally {
    validating.value = false
  }
}

// 保存流水线
const savePipeline = async () => {
  try {
    saving.value = true
    const data = {
      name: pipeline.value?.name,
      type: pipeline.value?.type,
      description: pipeline.value?.description,
      stages: stages.value
    }

    if (pipeline.value?.id) {
      await api.updatePipeline(pipeline.value.id, data)
    } else {
      await api.createPipeline(route.query.projectId, data)
    }

    ElMessage.success('保存成功')
    goBack()
  } catch (error) {
    ElMessage.error(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 返回
const goBack = () => {
  router.push('/pipeline')
}

// 初始化
onMounted(() => {
  loadPipeline()
  autoLayout()
})
</script>

<style scoped>
.pipeline-editor {
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

.editor-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.stages-panel,
.dag-panel,
.config-panel {
  display: flex;
  flex-direction: column;
  border-right: 1px solid #eee;
  background: #fff;
}

.stages-panel {
  width: 250px;
  min-width: 250px;
}

.dag-panel {
  flex: 1;
  position: relative;
}

.config-panel {
  width: 300px;
  min-width: 300px;
  border-right: none;
  border-left: 1px solid #eee;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px;
  border-bottom: 1px solid #eee;
  font-weight: 600;
}

.stages-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
}

.stage-item {
  padding: 12px;
  margin-bottom: 10px;
  border: 1px solid #eee;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.stage-item:hover {
  border-color: #409eff;
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.1);
}

.stage-item.active {
  border-color: #409eff;
  background: #f0f7ff;
}

.stage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.stage-name {
  font-weight: 500;
}

.more-icon {
  cursor: pointer;
  opacity: 0.6;
}

.more-icon:hover {
  opacity: 1;
}

.stage-tasks {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.dag-canvas {
  flex: 1;
  overflow: hidden;
  position: relative;
  background: #f5f7fa;
}

.dag-svg {
  display: block;
  cursor: grab;
}

.dag-svg:active {
  cursor: grabbing;
}

.node {
  cursor: pointer;
  transition: all 0.2s;
}

.node:hover {
  filter: brightness(0.95);
}

.node.selected {
  filter: drop-shadow(0 0 4px #409eff);
}

.node-bg {
  fill: #fff;
  stroke: #ddd;
  stroke-width: 1;
}

.node-shell .node-bg { stroke: #409eff; }
.node-http .node-bg { stroke: #67c23a; }
.node-delay .node-bg { stroke: #e6a23c; }
.node-condition .node-bg { stroke: #909399; }

.node-icon {
  font-size: 16px;
}

.node-label {
  font-size: 12px;
  fill: #333;
  pointer-events: none;
}

.status-dot {
  stroke: #fff;
  stroke-width: 1;
}

.status-pending { fill: #909399; }
.status-running { fill: #409eff; }
.status-success { fill: #67c23a; }
.status-failed { fill: #f56c6c; }

.port {
  cursor: crosshair;
  fill: #409eff;
  transition: r 0.2s;
}

.port:hover {
  r: 8 !important;
}

.connection {
  cursor: pointer;
  transition: stroke 0.2s;
}

.connection:hover {
  stroke: #409eff !important;
}

.connection.selected {
  stroke: #409eff !important;
  stroke-width: 3 !important;
}

.connection.creating {
  pointer-events: none;
}

.dag-toolbar {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #fff;
  padding: 8px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.config-content {
  flex: 1;
  overflow-y: auto;
  padding: 15px;
}
</style>
