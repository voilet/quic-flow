<template>
  <div class="alert-list-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>告警列表</span>
          <div class="header-actions">
            <el-button
              :type="sseConnected ? 'success' : 'info'"
              :icon="sseConnected ? Connection : Warning"
              @click="toggleSSE"
            >
              {{ sseConnected ? '实时监控中' : '开启实时监控' }}
            </el-button>
            <el-button
              type="primary"
              :disabled="selectedRows.length === 0"
              @click="handleBatchResolve"
            >
              批量解决
            </el-button>
            <el-button
              type="warning"
              :disabled="selectedRows.length === 0"
              @click="handleBatchSilence"
            >
              批量抑制
            </el-button>
          </div>
        </div>
      </template>

      <!-- 统计卡片 -->
      <div class="stats-cards">
        <div class="stat-card">
          <div class="stat-value critical">{{ stats.critical || 0 }}</div>
          <div class="stat-label">严重告警</div>
        </div>
        <div class="stat-card">
          <div class="stat-value warning">{{ stats.warning || 0 }}</div>
          <div class="stat-label">警告告警</div>
        </div>
        <div class="stat-card">
          <div class="stat-value info">{{ stats.info || 0 }}</div>
          <div class="stat-label">信息告警</div>
        </div>
        <div class="stat-card">
          <div class="stat-value">{{ stats.total || 0 }}</div>
          <div class="stat-label">告警总数</div>
        </div>
      </div>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="活跃" value="firing" />
            <el-option label="已解决" value="resolved" />
            <el-option label="已抑制" value="silenced" />
          </el-select>
        </el-form-item>
        <el-form-item label="严重程度">
          <el-select v-model="searchForm.severity" placeholder="请选择严重程度" clearable>
            <el-option label="全部" value="" />
            <el-option label="严重" value="critical" />
            <el-option label="警告" value="warning" />
            <el-option label="信息" value="info" />
          </el-select>
        </el-form-item>
        <el-form-item label="规则名称">
          <el-input
            v-model="searchForm.rule_name"
            placeholder="请输入规则名称"
            clearable
          />
        </el-form-item>
        <el-form-item label="告警名称">
          <el-input
            v-model="searchForm.alert_name"
            placeholder="请输入告警名称"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 批量操作栏 -->
      <div v-if="selectedRows.length > 0" class="batch-actions">
        <span class="selection-info">已选择 {{ selectedRows.length }} 项</span>
      </div>

      <!-- 告警列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="severity" label="严重程度" width="100">
          <template #default="{ row }">
            <el-tag :type="getSeverityTagType(row.severity)" size="small">
              {{ getSeverityLabel(row.severity) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="alert_name" label="告警名称" min-width="200" />
        <el-table-column prop="rule_name" label="规则名称" width="180" />
        <el-table-column prop="fingerprint" label="指纹" width="120">
          <template #default="{ row }">
            <el-tooltip :content="row.fingerprint" placement="top">
              <span class="fingerprint-text">{{ row.fingerprint.slice(0, 8) }}...</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="标签" min-width="200">
          <template #default="{ row }">
            <el-tag
              v-for="(value, key) in row.labels"
              :key="key"
              size="small"
              class="tag-item"
            >
              {{ key }}: {{ value }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusTagType(row.status)" size="small">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="starts_at" label="开始时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.starts_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleViewDetail(row)">详情</el-button>
            <el-button
              v-if="row.status === 'firing'"
              link
              type="primary"
              @click="handleResolve(row)"
            >
              解决
            </el-button>
            <el-button
              v-if="row.status === 'firing'"
              link
              type="warning"
              @click="handleSilence(row)"
            >
              抑制
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 告警详情对话框 -->
    <el-dialog
      v-model="detailDialogVisible"
      title="告警详情"
      width="800px"
      :close-on-click-modal="false"
    >
      <div v-if="currentAlert" class="alert-detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="告警名称">
            {{ currentAlert.alert_name }}
          </el-descriptions-item>
          <el-descriptions-item label="规则名称">
            {{ currentAlert.rule_name }}
          </el-descriptions-item>
          <el-descriptions-item label="严重程度">
            <el-tag :type="getSeverityTagType(currentAlert.severity)">
              {{ getSeverityLabel(currentAlert.severity) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusTagType(currentAlert.status)">
              {{ getStatusLabel(currentAlert.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="指纹" :span="2">
            <code class="fingerprint-code">{{ currentAlert.fingerprint }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">
            {{ formatDateTime(currentAlert.starts_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="结束时间">
            {{ currentAlert.ends_at ? formatDateTime(currentAlert.ends_at) : '-' }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="detail-section">
          <h4>标签</h4>
          <div class="tags-container">
            <el-tag
              v-for="(value, key) in currentAlert.labels"
              :key="key"
              class="tag-item"
            >
              {{ key }}: {{ value }}
            </el-tag>
          </div>
        </div>

        <div class="detail-section">
          <h4>注解</h4>
          <div class="annotations-container">
            <div v-for="(value, key) in currentAlert.annotations" :key="key" class="annotation-item">
              <strong>{{ key }}:</strong> {{ value }}
            </div>
          </div>
        </div>

        <div v-if="currentAlert.value" class="detail-section">
          <h4>当前值</h4>
          <code class="value-code">{{ currentAlert.value }}</code>
        </div>

        <div v-if="currentAlert.silenced_by && currentAlert.silenced_by.length > 0" class="detail-section">
          <h4>抑制规则</h4>
          <div class="silenced-by">
            <el-tag
              v-for="silenceId in currentAlert.silenced_by"
              :key="silenceId"
              type="warning"
              class="tag-item"
            >
              {{ silenceId }}
            </el-tag>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="detailDialogVisible = false">关闭</el-button>
        <el-button
          v-if="currentAlert && currentAlert.status === 'firing'"
          type="primary"
          @click="handleResolveFromDialog"
        >
          解决告警
        </el-button>
        <el-button
          v-if="currentAlert && currentAlert.status === 'firing'"
          type="warning"
          @click="handleSilenceFromDialog"
        >
          抑制告警
        </el-button>
      </template>
    </el-dialog>

    <!-- 解决告警对话框 -->
    <el-dialog
      v-model="resolveDialogVisible"
      title="解决告警"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form :model="resolveForm" label-width="100px">
        <el-form-item label="解决原因">
          <el-input
            v-model="resolveForm.reason"
            type="textarea"
            :rows="4"
            placeholder="请输入解决原因（可选）"
          />
        </el-form-item>
        <el-form-item label="解决人">
          <el-input v-model="resolveForm.resolved_by" placeholder="请输入解决人" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resolveDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmResolve">确定</el-button>
      </template>
    </el-dialog>

    <!-- 抑制告警对话框 -->
    <el-dialog
      v-model="silenceDialogVisible"
      title="抑制告警"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form :model="silenceForm" label-width="100px">
        <el-form-item label="抑制时长">
          <el-select v-model="silenceForm.duration" placeholder="请选择时长">
            <el-option label="1 小时" :value="3600" />
            <el-option label="6 小时" :value="21600" />
            <el-option label="12 小时" :value="43200" />
            <el-option label="1 天" :value="86400" />
            <el-option label="3 天" :value="259200" />
            <el-option label="7 天" :value="604800" />
            <el-option label="永久" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item label="结束时间">
          <el-date-picker
            v-model="silenceForm.ends_at"
            type="datetime"
            placeholder="选择结束时间"
            :disabled="silenceForm.duration !== 0"
          />
        </el-form-item>
        <el-form-item label="创建人">
          <el-input v-model="silenceForm.created_by" placeholder="请输入创建人" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="silenceForm.comment"
            type="textarea"
            :rows="4"
            placeholder="请输入备注（可选）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="silenceDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmSilence">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Warning } from '@element-plus/icons-vue'
import {
  listAlerts,
  resolveAlert,
  silenceAlert,
  batchResolveAlerts,
  batchSilenceAlerts,
  getAlertStats,
  subscribeAlertEvents
} from '@/api/alert'
import dayjs from 'dayjs'

// 数据状态
const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const stats = ref({})

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 搜索表单
const searchForm = reactive({
  status: '',
  severity: '',
  rule_name: '',
  alert_name: ''
})

// 对话框状态
const detailDialogVisible = ref(false)
const resolveDialogVisible = ref(false)
const silenceDialogVisible = ref(false)
const currentAlert = ref(null)

// 解决表单
const resolveForm = reactive({
  reason: '',
  resolved_by: ''
})

// 抑制表单
const silenceForm = reactive({
  duration: 3600,
  ends_at: null,
  created_by: '',
  comment: ''
})

// SSE 连接状态
const sseConnected = ref(false)
let sseConnection = null

// 加载告警列表
const loadAlerts = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...searchForm
    }
    const response = await listAlerts(params)
    tableData.value = response.data?.alerts || []
    pagination.total = response.data?.total || 0
  } catch (error) {
    ElMessage.error('加载告警列表失败')
  } finally {
    loading.value = false
  }
}

// 加载统计数据
const loadStats = async () => {
  try {
    const response = await getAlertStats(searchForm)
    stats.value = response.data || {}
  } catch (error) {
    console.error('加载统计数据失败', error)
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadAlerts()
  loadStats()
}

// 重置
const handleReset = () => {
  Object.assign(searchForm, {
    status: '',
    severity: '',
    rule_name: '',
    alert_name: ''
  })
  pagination.page = 1
  loadAlerts()
  loadStats()
}

// 分页变化
const handleSizeChange = () => {
  pagination.page = 1
  loadAlerts()
}

const handlePageChange = () => {
  loadAlerts()
}

// 选择变化
const handleSelectionChange = (selection) => {
  selectedRows.value = selection
}

// 查看详情
const handleViewDetail = async (row) => {
  currentAlert.value = row
  detailDialogVisible.value = true
}

// 解决告警
const handleResolve = (row) => {
  currentAlert.value = row
  resolveForm.reason = ''
  resolveForm.resolved_by = ''
  resolveDialogVisible.value = true
}

const handleResolveFromDialog = () => {
  resolveDialogVisible.value = true
  detailDialogVisible.value = false
}

const confirmResolve = async () => {
  try {
    await resolveAlert(currentAlert.value.id, {
      reason: resolveForm.reason,
      resolved_by: resolveForm.resolved_by || '系统管理员'
    })
    ElMessage.success('告警已解决')
    resolveDialogVisible.value = false
    loadAlerts()
    loadStats()
  } catch (error) {
    ElMessage.error('解决告警失败')
  }
}

// 抑制告警
const handleSilence = (row) => {
  currentAlert.value = row
  silenceForm.duration = 3600
  silenceForm.ends_at = null
  silenceForm.created_by = ''
  silenceForm.comment = ''
  silenceDialogVisible.value = true
}

const handleSilenceFromDialog = () => {
  silenceDialogVisible.value = true
  detailDialogVisible.value = false
}

const confirmSilence = async () => {
  try {
    await silenceAlert(currentAlert.value.id, {
      ends_at: silenceForm.ends_at
        ? dayjs(silenceForm.ends_at).toISOString()
        : dayjs().add(silenceForm.duration, 'second').toISOString(),
      created_by: silenceForm.created_by || '系统管理员',
      comment: silenceForm.comment,
      matchers: [
        {
          name: 'fingerprint',
          value: currentAlert.value.fingerprint,
          is_regex: false
        }
      ]
    })
    ElMessage.success('告警已抑制')
    silenceDialogVisible.value = false
    loadAlerts()
    loadStats()
  } catch (error) {
    ElMessage.error('抑制告警失败')
  }
}

// 批量解决
const handleBatchResolve = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要解决选中的 ${selectedRows.value.length} 个告警吗？`,
      '批量解决',
      {
        type: 'warning'
      }
    )
    await batchResolveAlerts({
      alert_ids: selectedRows.value.map(row => row.id),
      resolved_by: '系统管理员'
    })
    ElMessage.success('批量解决成功')
    loadAlerts()
    loadStats()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('批量解决失败')
    }
  }
}

// 批量抑制
const handleBatchSilence = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要抑制选中的 ${selectedRows.value.length} 个告警吗？`,
      '批量抑制',
      {
        type: 'warning'
      }
    )
    await batchSilenceAlerts({
      alert_ids: selectedRows.value.map(row => row.id),
      ends_at: dayjs().add(1, 'hour').toISOString(),
      created_by: '系统管理员'
    })
    ElMessage.success('批量抑制成功')
    loadAlerts()
    loadStats()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('批量抑制失败')
    }
  }
}

// SSE 实时监控
const toggleSSE = () => {
  if (sseConnected.value) {
    // 关闭连接
    if (sseConnection) {
      sseConnection.close()
      sseConnection = null
    }
    sseConnected.value = false
    ElMessage.info('实时监控已关闭')
  } else {
    // 开启连接
    sseConnection = subscribeAlertEvents(
      // 新告警
      (alert) => {
        tableData.value.unshift(alert)
        ElMessage.warning(`新告警: ${alert.alert_name}`)
        loadStats()
      },
      // 更新
      (alert) => {
        const index = tableData.value.findIndex(a => a.id === alert.id)
        if (index !== -1) {
          tableData.value[index] = alert
        }
      },
      // 解决
      (alert) => {
        const index = tableData.value.findIndex(a => a.id === alert.id)
        if (index !== -1) {
          tableData.value[index] = alert
        }
        ElMessage.success(`告警已解决: ${alert.alert_name}`)
        loadStats()
      },
      // 错误
      (error) => {
        console.error('SSE error:', error)
        sseConnected.value = false
      }
    )
    sseConnected.value = true
    ElMessage.success('实时监控已开启')
  }
}

// 工具函数
const formatDateTime = (dateStr) => {
  return dateStr ? dayjs(dateStr).format('YYYY-MM-DD HH:mm:ss') : '-'
}

const getSeverityLabel = (severity) => {
  const map = {
    critical: '严重',
    warning: '警告',
    info: '信息'
  }
  return map[severity] || severity
}

const getSeverityTagType = (severity) => {
  const map = {
    critical: 'danger',
    warning: 'warning',
    info: 'info'
  }
  return map[severity] || ''
}

const getStatusLabel = (status) => {
  const map = {
    firing: '活跃',
    resolved: '已解决',
    silenced: '已抑制'
  }
  return map[status] || status
}

const getStatusTagType = (status) => {
  const map = {
    firing: 'danger',
    resolved: 'success',
    silenced: 'warning'
  }
  return map[status] || ''
}

// 生命周期
onMounted(() => {
  loadAlerts()
  loadStats()
})

onUnmounted(() => {
  if (sseConnection) {
    sseConnection.close()
  }
})
</script>

<style scoped>
.alert-list-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  margin-bottom: 20px;
}

.stat-card {
  padding: 20px;
  border-radius: 8px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  text-align: center;
}

.stat-card.critical {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
}

.stat-card.warning {
  background: linear-gradient(135deg, #ffecd2 0%, #fcb69f 100%);
  color: #333;
}

.stat-card.info {
  background: linear-gradient(135deg, #a1c4fd 0%, #c2e9fb 100%);
  color: #333;
}

.stat-value {
  font-size: 36px;
  font-weight: bold;
  margin-bottom: 8px;
}

.stat-label {
  font-size: 14px;
  opacity: 0.9;
}

.search-form {
  margin-bottom: 20px;
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
  margin-bottom: 16px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
}

.selection-info {
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.fingerprint-text {
  font-family: monospace;
  font-size: 12px;
}

.tag-item {
  margin-right: 6px;
  margin-bottom: 6px;
}

.alert-detail {
  padding: 10px 0;
}

.detail-section {
  margin-top: 20px;
}

.detail-section h4 {
  margin-bottom: 10px;
  font-size: 16px;
  font-weight: 600;
}

.tags-container {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.annotations-container {
  background-color: var(--el-fill-color-light);
  padding: 12px;
  border-radius: 4px;
}

.annotation-item {
  margin-bottom: 8px;
  font-size: 14px;
}

.fingerprint-code {
  display: block;
  padding: 8px 12px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

.value-code {
  display: block;
  padding: 12px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 14px;
}

.silenced-by {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
