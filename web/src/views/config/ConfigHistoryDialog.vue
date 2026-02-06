<template>
  <el-dialog
    v-model="dialogVisible"
    title="配置变更历史"
    width="1000px"
    @close="handleClose"
  >
    <div class="history-container">
      <!-- 时间线 -->
      <div class="timeline-wrapper">
        <el-timeline>
          <el-timeline-item
            v-for="item in historyList"
            :key="item.id"
            :timestamp="formatDate(item.created_at)"
            placement="top"
            :type="getTimelineType(item)"
            :icon="getTimelineIcon(item)"
          >
            <div class="timeline-card" @click="selectVersion(item)">
              <div class="card-header">
                <span class="version-tag">v{{ item.version }}</span>
                <el-tag :type="getActionType(item.action)" size="small">
                  {{ getActionText(item.action) }}
                </el-tag>
              </div>
              <div class="card-content">
                <div class="content-row">
                  <span class="label">操作人:</span>
                  <span class="value">{{ item.operator || '-' }}</span>
                </div>
                <div class="content-row">
                  <span class="label">备注:</span>
                  <span class="value">{{ item.comment || '-' }}</span>
                </div>
                <div v-if="item.change_summary" class="change-summary">
                  <span class="label">变更:</span>
                  <span>{{ item.change_summary }}</span>
                </div>
              </div>
            </div>
          </el-timeline-item>
        </el-timeline>

        <div v-if="hasMore" class="load-more">
          <el-button link @click="loadMore">加载更多</el-button>
        </div>
      </div>

      <!-- 版本详情/对比 -->
      <div class="detail-panel">
        <el-empty v-if="!selectedVersion" description="请选择一个版本查看详情" />

        <div v-else class="version-detail">
          <div class="detail-header">
            <div class="header-left">
              <h4>版本 v{{ selectedVersion.version }}</h4>
              <el-tag :type="getActionType(selectedVersion.action)" size="small">
                {{ getActionText(selectedVersion.action) }}
              </el-tag>
            </div>
            <div class="header-actions">
              <el-button size="small" @click="handleViewDiff">
                <el-icon><Comparison /></el-icon>
                对比当前版本
              </el-button>
              <el-button
                v-if="canRollback"
                size="small"
                type="warning"
                @click="handleRollback"
              >
                <el-icon><RefreshLeft /></el-icon>
                回滚到此版本
              </el-button>
            </div>
          </div>

          <div class="detail-content">
            <div class="content-info">
              <div class="info-item">
                <span class="label">操作时间:</span>
                <span>{{ formatDate(selectedVersion.created_at) }}</span>
              </div>
              <div class="info-item">
                <span class="label">操作人:</span>
                <span>{{ selectedVersion.operator || '-' }}</span>
              </div>
              <div class="info-item">
                <span class="label">备注:</span>
                <span>{{ selectedVersion.comment || '-' }}</span>
              </div>
            </div>

            <el-divider>配置内容</el-divider>

            <div class="content-editor">
              <pre class="config-content">{{ selectedVersion.content || '无内容' }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 对比对话框 -->
    <el-dialog
      v-model="diffVisible"
      title="版本对比"
      width="900px"
      append-to-body
    >
      <div class="diff-container">
        <div class="diff-header">
          <div class="diff-source">
            <span class="version-label">v{{ diffFromVersion }}</span>
          </div>
          <div class="diff-arrow">→</div>
          <div class="diff-target">
            <span class="version-label">v{{ diffToVersion }}</span>
          </div>
        </div>
        <div v-if="diffResult" class="diff-result" v-html="diffResult"></div>
        <el-empty v-else description="加载中..." />
      </div>
    </el-dialog>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Comparison, RefreshLeft } from '@element-plus/icons-vue'
import { configApi } from '@/api/config'
import dayjs from 'dayjs'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  configId: {
    type: String,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const historyList = ref([])
const selectedVersion = ref(null)
const diffVisible = ref(false)
const diffResult = ref('')
const diffFromVersion = ref('')
const diffToVersion = ref('')

const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const hasMore = ref(false)

// 当前配置版本
const currentVersion = ref(0)

// 是否可以回滚
const canRollback = computed(() => {
  return selectedVersion.value && selectedVersion.value.version < currentVersion.value
})

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 获取时间线类型
const getTimelineType = (item) => {
  switch (item.action) {
    case 'create':
      return 'success'
    case 'update':
      return 'primary'
    case 'rollback':
      return 'warning'
    case 'delete':
      return 'danger'
    default:
      return 'info'
  }
}

// 获取时间线图标
const getTimelineIcon = (item) => {
  // 可以根据 action 返回不同的图标
  return null
}

// 获取操作类型标签颜色
const getActionType = (action) => {
  switch (action) {
    case 'create':
      return 'success'
    case 'update':
      return 'primary'
    case 'rollback':
      return 'warning'
    case 'delete':
      return 'danger'
    case 'publish':
      return 'success'
    default:
      return 'info'
  }
}

// 获取操作文本
const getActionText = (action) => {
  const actionMap = {
    create: '创建',
    update: '更新',
    rollback: '回滚',
    delete: '删除',
    publish: '发布'
  }
  return actionMap[action] || action
}

// 加载历史记录
const loadHistory = async (reset = false) => {
  if (!props.configId) return

  if (reset) {
    currentPage.value = 1
    historyList.value = []
  }

  try {
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    const res = await configApi.listConfigHistory(props.configId, params)
    if (res.success) {
      const items = res.data.items || []
      if (reset) {
        historyList.value = items
      } else {
        historyList.value.push(...items)
      }
      total.value = res.data.total || 0
      hasMore.value = historyList.value.length < total.value

      // 获取当前版本号（第一个版本）
      if (items.length > 0 && reset) {
        currentVersion.value = items[0].version
      }
    }
  } catch (error) {
    ElMessage.error('加载历史记录失败')
  }
}

// 加载更多
const loadMore = () => {
  currentPage.value++
  loadHistory()
}

// 选择版本
const selectVersion = (item) => {
  selectedVersion.value = item
}

// 查看对比
const handleViewDiff = async () => {
  if (!selectedVersion.value) return

  diffFromVersion.value = selectedVersion.value.version
  diffToVersion.value = currentVersion.value
  diffVisible.value = true
  diffResult.value = ''

  try {
    const res = await configApi.compareConfig(props.configId, {
      from_version: diffFromVersion.value
    })
    if (res.success) {
      const { old_content, new_content } = res.data
      // 简单的行级别对比
      const oldLines = (old_content || '').split('\n')
      const newLines = (new_content || '').split('\n')
      const maxLines = Math.max(oldLines.length, newLines.length)

      let html = ''
      for (let i = 0; i < maxLines; i++) {
        const oldLine = oldLines[i] || ''
        const newLine = newLines[i] || ''

        if (oldLine === newLine) {
          html += `<div class="diff-line unchanged">${escapeHtml(oldLine)}</div>`
        } else {
          if (oldLine) {
            html += `<div class="diff-line removed" style="background-color: #f8d7da;">${escapeHtml(oldLine)}</div>`
          }
          if (newLine) {
            html += `<div class="diff-line added" style="background-color: #d4edda;">${escapeHtml(newLine)}</div>`
          }
        }
      }
      diffResult.value = html
    }
  } catch (error) {
    ElMessage.error('加载对比失败')
  }
}

// 回滚到指定版本
const handleRollback = async () => {
  if (!selectedVersion.value) return

  try {
    await ElMessageBox.confirm(
      `确定要回滚到版本 v${selectedVersion.value.version} 吗？此操作将创建一个新版本。`,
      '确认回滚',
      { type: 'warning' }
    )

    const res = await configApi.rollbackConfig(props.configId, {
      to_version: selectedVersion.value.version,
      comment: `回滚到 v${selectedVersion.value.version}`
    })

    if (res.success) {
      ElMessage.success('回滚成功')
      dialogVisible.value = false
      emit('refresh')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('回滚失败')
    }
  }
}

// HTML 转义
const escapeHtml = (text) => {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

// 关闭对话框
const handleClose = () => {
  emit('update:modelValue', false)
}

// 监听对话框打开
watch(() => props.modelValue, (val) => {
  if (val) {
    selectedVersion.value = null
    loadHistory(true)
  }
})
</script>

<style scoped>
.history-container {
  display: flex;
  gap: 20px;
  height: 600px;
}

.timeline-wrapper {
  flex: 1;
  overflow-y: auto;
  padding-right: 12px;
}

.timeline-card {
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.3s;
}

.timeline-card:hover {
  background: var(--el-fill-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.version-tag {
  font-weight: 600;
  color: var(--el-color-primary);
}

.card-content {
  font-size: 13px;
}

.content-row {
  display: flex;
  margin-bottom: 4px;
}

.content-row .label {
  width: 60px;
  color: var(--el-text-color-secondary);
}

.content-row .value {
  flex: 1;
  color: var(--el-text-color-primary);
}

.change-summary {
  margin-top: 8px;
  padding: 8px;
  background: var(--el-fill-color-lighter);
  border-radius: 4px;
  font-size: 12px;
}

.change-summary .label {
  font-weight: 500;
  display: block;
  margin-bottom: 4px;
}

.load-more {
  text-align: center;
  padding: 12px 0;
}

.detail-panel {
  flex: 1;
  border-left: 1px solid var(--el-border-color);
  padding-left: 20px;
  overflow-y: auto;
}

.version-detail {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h4 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.detail-content {
  flex: 1;
  overflow-y: auto;
}

.content-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.info-item {
  font-size: 13px;
}

.info-item .label {
  color: var(--el-text-color-secondary);
  margin-right: 8px;
}

.content-editor {
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.config-content {
  margin: 0;
  padding: 16px;
  background: var(--el-fill-color-lighter);
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: monospace;
  font-size: 13px;
  max-height: 400px;
  overflow-y: auto;
}

/* 对比对话框样式 */
.diff-container {
  padding: 16px 0;
}

.diff-header {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 24px;
  margin-bottom: 20px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.version-label {
  font-weight: 600;
  color: var(--el-color-primary);
}

.diff-arrow {
  font-size: 20px;
  color: var(--el-text-color-secondary);
}

.diff-result {
  padding: 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  max-height: 400px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.6;
}

.diff-line {
  padding: 2px 4px;
}

.diff-line.added {
  border-left: 3px solid var(--el-color-success);
}

.diff-line.removed {
  border-left: 3px solid var(--el-color-danger);
}

.diff-line.unchanged {
  color: var(--el-text-color-secondary);
}
</style>
