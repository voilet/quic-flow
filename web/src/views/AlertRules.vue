<template>
  <div class="alert-rules-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>告警规则管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建规则
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="searchForm.enabled" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="已启用" :value="true" />
            <el-option label="已禁用" :value="false" />
          </el-select>
        </el-form-item>
        <el-form-item label="规则名称">
          <el-input
            v-model="searchForm.name"
            placeholder="请输入规则名称"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 规则列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="name" label="规则名称" min-width="200" />
        <el-table-column prop="expression" label="表达式" min-width="300">
          <template #default="{ row }">
            <el-tooltip :content="row.expression" placement="top">
              <code class="expression-code">{{ row.expression }}</code>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="severity" label="严重程度" width="100">
          <template #default="{ row }">
            <el-tag :type="getSeverityTagType(row.severity)" size="small">
              {{ getSeverityLabel(row.severity) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="评估间隔" width="100">
          <template #default="{ row }">
            {{ row.interval }}s
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="handleToggle(row)"
            />
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="handleTest(row)">测试</el-button>
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <!-- 规则编辑对话框 -->
    <el-dialog
      v-model="editDialogVisible"
      :title="dialogTitle"
      width="900px"
      :close-on-click-modal="false"
      @open="handleDialogOpen"
    >
      <el-form
        ref="ruleFormRef"
        :model="ruleForm"
        :rules="ruleRules"
        label-width="120px"
      >
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="规则名称" prop="name">
              <el-input v-model="ruleForm.name" placeholder="请输入规则名称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="严重程度" prop="severity">
              <el-select v-model="ruleForm.severity" placeholder="请选择严重程度">
                <el-option label="严重" value="critical" />
                <el-option label="警告" value="warning" />
                <el-option label="信息" value="info" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="规则表达式" prop="expression">
          <div class="expression-editor-wrapper">
            <monaco-editor
              v-model="ruleForm.expression"
              language="javascript"
              height="150px"
              :options="{
                minimap: { enabled: false },
                fontSize: 13,
                lineNumbers: 'on',
                scrollBeyondLastLine: false
              }"
            />
          </div>
          <div class="expression-help">
            <el-text type="info" size="small">
              支持 CEL 表达式语法，例如: metric.value > 100
            </el-text>
          </div>
        </el-form-item>

        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="评估间隔" prop="interval">
              <el-input-number
                v-model="ruleForm.interval"
                :min="10"
                :max="86400"
                :step="10"
              />
              <span style="margin-left: 8px">秒</span>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="持续时间" prop="for">
              <el-input-number
                v-model="ruleForm.for"
                :min="0"
                :max="86400"
                :step="10"
              />
              <span style="margin-left: 8px">秒（0 表示立即触发）</span>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="描述" prop="description">
          <el-input
            v-model="ruleForm.description"
            type="textarea"
            :rows="3"
            placeholder="请输入规则描述"
          />
        </el-form-item>

        <el-form-item label="告警名称">
          <el-input v-model="ruleForm.alert_name" placeholder="留空则使用规则名称" />
        </el-form-item>

        <el-form-item label="告警摘要">
          <el-input
            v-model="ruleForm.summary"
            placeholder="告警摘要模板，可使用 {{ .labels.xxx }}"
          />
        </el-form-item>

        <el-form-item label="告警描述">
          <el-input
            v-model="ruleForm.message"
            type="textarea"
            :rows="3"
            placeholder="告警详细描述模板"
          />
        </el-form-item>

        <el-form-item label="标签">
          <div class="labels-editor">
            <div
              v-for="(label, index) in ruleForm.labels"
              :key="index"
              class="label-row"
            >
              <el-input
                v-model="label.name"
                placeholder="标签名"
                style="width: 45%"
              />
              <span style="padding: 0 8px">=</span>
              <el-input
                v-model="label.value"
                placeholder="标签值"
                style="width: 45%"
              />
              <el-button
                type="danger"
                :icon="Delete"
                circle
                size="small"
                style="margin-left: 8px"
                @click="removeLabel(index)"
              />
            </div>
            <el-button
              type="primary"
              :icon="Plus"
              size="small"
              plain
              @click="addLabel"
            >
              添加标签
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="注解">
          <div class="annotations-editor">
            <div
              v-for="(annotation, index) in ruleForm.annotations"
              :key="index"
              class="annotation-row"
            >
              <el-input
                v-model="annotation.name"
                placeholder="注解名"
                style="width: 45%"
              />
              <span style="padding: 0 8px">=</span>
              <el-input
                v-model="annotation.value"
                placeholder="注解值"
                style="width: 45%"
              />
              <el-button
                type="danger"
                :icon="Delete"
                circle
                size="small"
                style="margin-left: 8px"
                @click="removeAnnotation(index)"
              />
            </div>
            <el-button
              type="primary"
              :icon="Plus"
              size="small"
              plain
              @click="addAnnotation"
            >
              添加注解
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="通知渠道">
          <el-select
            v-model="ruleForm.channels"
            multiple
            placeholder="请选择通知渠道"
            style="width: 100%"
          >
            <el-option
              v-for="channel in channels"
              :key="channel.id"
              :label="channel.name"
              :value="channel.id"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>

    <!-- 规则查看对话框 -->
    <el-dialog
      v-model="viewDialogVisible"
      title="规则详情"
      width="800px"
    >
      <div v-if="currentRule" class="rule-detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="规则名称">
            {{ currentRule.name }}
          </el-descriptions-item>
          <el-descriptions-item label="严重程度">
            <el-tag :type="getSeverityTagType(currentRule.severity)">
              {{ getSeverityLabel(currentRule.severity) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="评估间隔">
            {{ currentRule.interval }} 秒
          </el-descriptions-item>
          <el-descriptions-item label="持续时间">
            {{ currentRule.for || 0 }} 秒
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="currentRule.enabled ? 'success' : 'info'">
              {{ currentRule.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ formatDateTime(currentRule.created_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="表达式" :span="2">
            <code class="expression-code-full">{{ currentRule.expression }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">
            {{ currentRule.description || '-' }}
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="currentRule.labels && Object.keys(currentRule.labels).length > 0" class="detail-section">
          <h4>标签</h4>
          <div class="tags-container">
            <el-tag
              v-for="(value, key) in currentRule.labels"
              :key="key"
              class="tag-item"
            >
              {{ key }}: {{ value }}
            </el-tag>
          </div>
        </div>

        <div v-if="currentRule.annotations && Object.keys(currentRule.annotations).length > 0" class="detail-section">
          <h4>注解</h4>
          <div class="annotations-container">
            <div v-for="(value, key) in currentRule.annotations" :key="key" class="annotation-item">
              <strong>{{ key }}:</strong> {{ value }}
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 测试规则对话框 -->
    <el-dialog
      v-model="testDialogVisible"
      title="测试规则"
      width="700px"
    >
      <el-form :model="testForm" label-width="100px">
        <el-form-item label="测试数据">
          <div class="test-data-editor">
            <monaco-editor
              v-model="testForm.data"
              language="json"
              height="200px"
              :options="{
                minimap: { enabled: false },
                fontSize: 13,
                lineNumbers: 'on'
              }"
            />
          </div>
          <el-text type="info" size="small">
            输入测试用的指标数据（JSON 格式）
          </el-text>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="runTest">运行测试</el-button>
        </el-form-item>
        <el-form-item v-if="testResult" label="测试结果">
          <div :class="['test-result', testResult.match ? 'success' : 'error']">
            <el-tag :type="testResult.match ? 'success' : 'danger'">
              {{ testResult.match ? '匹配' : '不匹配' }}
            </el-tag>
            <pre v-if="testResult.evaluation">{{ testResult.evaluation }}</pre>
          </div>
        </el-form-item>
      </el-form>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import {
  listAlertRules,
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
  toggleAlertRule,
  testAlertRule
} from '@/api/alert'
import { listAlertChannels } from '@/api/alert'
import MonacoEditor from '@/components/MonacoEditor.vue'
import dayjs from 'dayjs'

// 数据状态
const loading = ref(false)
const tableData = ref([])
const channels = ref([])
const currentRule = ref(null)
const isEdit = ref(false)
const ruleFormRef = ref()

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 搜索表单
const searchForm = reactive({
  enabled: '',
  name: ''
})

// 对话框状态
const editDialogVisible = ref(false)
const viewDialogVisible = ref(false)
const testDialogVisible = ref(false)

// 规则表单
const ruleForm = reactive({
  name: '',
  expression: '',
  severity: 'warning',
  interval: 60,
  for: 0,
  description: '',
  alert_name: '',
  summary: '',
  message: '',
  labels: [],
  annotations: [],
  channels: []
})

// 表单验证规则
const ruleRules = {
  name: [
    { required: true, message: '请输入规则名称', trigger: 'blur' }
  ],
  expression: [
    { required: true, message: '请输入规则表达式', trigger: 'blur' }
  ],
  severity: [
    { required: true, message: '请选择严重程度', trigger: 'change' }
  ],
  interval: [
    { required: true, message: '请输入评估间隔', trigger: 'blur' }
  ]
}

// 测试表单
const testForm = reactive({
  data: JSON.stringify({
    metric: {
      name: 'cpu_usage',
      labels: {
        host: 'server1',
        region: 'cn-north'
      },
      value: 85.5
    },
    timestamp: new Date().toISOString()
  }, null, 2)
})

const testResult = ref(null)

// 计算属性
const dialogTitle = computed(() => isEdit.value ? '编辑规则' : '新建规则')

// 加载规则列表
const loadRules = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...searchForm
    }
    const response = await listAlertRules(params)
    tableData.value = response.data?.rules || []
    pagination.total = response.data?.total || 0
  } catch (error) {
    ElMessage.error('加载规则列表失败')
  } finally {
    loading.value = false
  }
}

// 加载通知渠道列表
const loadChannels = async () => {
  try {
    const response = await listAlertChannels()
    channels.value = response.data?.channels || []
  } catch (error) {
    console.error('加载渠道列表失败', error)
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadRules()
}

// 重置
const handleReset = () => {
  Object.assign(searchForm, {
    enabled: '',
    name: ''
  })
  pagination.page = 1
  loadRules()
}

// 分页变化
const handleSizeChange = () => {
  pagination.page = 1
  loadRules()
}

const handlePageChange = () => {
  loadRules()
}

// 新建规则
const handleCreate = () => {
  isEdit.value = false
  resetRuleForm()
  editDialogVisible.value = true
}

// 编辑规则
const handleEdit = (row) => {
  isEdit.value = true
  currentRule.value = row
  Object.assign(ruleForm, {
    id: row.id,
    name: row.name,
    expression: row.expression,
    severity: row.severity,
    interval: row.interval,
    for: row.for || 0,
    description: row.description || '',
    alert_name: row.alert_name || '',
    summary: row.summary || '',
    message: row.message || '',
    labels: Object.entries(row.labels || {}).map(([name, value]) => ({ name, value })),
    annotations: Object.entries(row.annotations || {}).map(([name, value]) => ({ name, value })),
    channels: row.channels || []
  })
  editDialogVisible.value = true
}

// 查看规则
const handleView = (row) => {
  currentRule.value = row
  viewDialogVisible.value = true
}

// 删除规则
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除规则 "${row.name}" 吗？`,
      '删除规则',
      {
        type: 'warning'
      }
    )
    await deleteAlertRule(row.id)
    ElMessage.success('删除成功')
    loadRules()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 切换启用状态
const handleToggle = async (row) => {
  try {
    await toggleAlertRule(row.id, row.enabled)
    ElMessage.success(row.enabled ? '规则已启用' : '规则已禁用')
  } catch (error) {
    row.enabled = !row.enabled
    ElMessage.error('操作失败')
  }
}

// 测试规则
const handleTest = (row) => {
  currentRule.value = row
  testResult.value = null
  testDialogVisible.value = true
}

// 运行测试
const runTest = async () => {
  try {
    const response = await testAlertRule({
      rule: currentRule.value,
      data: JSON.parse(testForm.data)
    })
    testResult.value = response.data
  } catch (error) {
    ElMessage.error('测试失败：' + (error.response?.data?.msg || error.message))
  }
}

// 添加标签
const addLabel = () => {
  ruleForm.labels.push({ name: '', value: '' })
}

// 移除标签
const removeLabel = (index) => {
  ruleForm.labels.splice(index, 1)
}

// 添加注解
const addAnnotation = () => {
  ruleForm.annotations.push({ name: '', value: '' })
}

// 移除注解
const removeAnnotation = (index) => {
  ruleForm.annotations.splice(index, 1)
}

// 重置表单
const resetRuleForm = () => {
  Object.assign(ruleForm, {
    name: '',
    expression: '',
    severity: 'warning',
    interval: 60,
    for: 0,
    description: '',
    alert_name: '',
    summary: '',
    message: '',
    labels: [],
    annotations: [],
    channels: []
  })
}

// 对话框打开时
const handleDialogOpen = () => {
  if (!isEdit.value) {
    resetRuleForm()
  }
}

// 保存规则
const handleSave = async () => {
  try {
    await ruleFormRef.value.validate()

    // 转换标签和注解
    const labels = {}
    ruleForm.labels.forEach(({ name, value }) => {
      if (name && value) {
        labels[name] = value
      }
    })

    const annotations = {}
    ruleForm.annotations.forEach(({ name, value }) => {
      if (name && value) {
        annotations[name] = value
      }
    })

    const data = {
      ...ruleForm,
      labels,
      annotations
    }

    if (isEdit.value) {
      await updateAlertRule(ruleForm.id, data)
      ElMessage.success('更新成功')
    } else {
      await createAlertRule(data)
      ElMessage.success('创建成功')
    }

    editDialogVisible.value = false
    loadRules()
  } catch (error) {
    if (error !== false) {
      ElMessage.error('保存失败')
    }
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

// 生命周期
onMounted(() => {
  loadRules()
  loadChannels()
})
</script>

<style scoped>
.alert-rules-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-form {
  margin-bottom: 20px;
}

.expression-code {
  display: block;
  padding: 4px 8px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.expression-code-full {
  display: block;
  padding: 12px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.expression-editor-wrapper {
  width: 100%;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.expression-help {
  margin-top: 8px;
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.label-row,
.annotation-row {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}

.rule-detail {
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

.tag-item {
  margin-right: 6px;
  margin-bottom: 6px;
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

.test-data-editor {
  width: 100%;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 8px;
}

.test-result {
  padding: 16px;
  border-radius: 4px;
}

.test-result.success {
  background-color: var(--el-color-success-light-9);
}

.test-result.error {
  background-color: var(--el-color-danger-light-9);
}

.test-result pre {
  margin-top: 12px;
  padding: 8px;
  background-color: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
}
</style>
