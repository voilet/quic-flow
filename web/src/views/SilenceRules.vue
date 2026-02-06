<template>
  <div class="silence-rules-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>抑制规则管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建规则
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="活跃" value="active" />
            <el-option label="已过期" value="expired" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
        </el-form-item>
        <el-form-item label="创建人">
          <el-input
            v-model="searchForm.created_by"
            placeholder="请输入创建人"
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
        <el-table-column prop="id" label="规则 ID" width="120">
          <template #default="{ row }">
            <el-tooltip :content="row.id" placement="top">
              <span class="rule-id-text">{{ row.id.slice(0, 8) }}...</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="comment" label="备注" min-width="200" />
        <el-table-column label="匹配条件" min-width="300">
          <template #default="{ row }">
            <div class="matchers-display">
              <el-tag
                v-for="(matcher, index) in row.matchers"
                :key="index"
                size="small"
                :type="matcher.is_regex ? 'warning' : 'info'"
              >
                {{ matcher.name }} {{ matcher.is_regex ? '=~' : '=' }} {{ matcher.value }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="时间范围" width="300">
          <template #default="{ row }">
            <div class="time-range">
              <div class="time-start">{{ formatDateTime(row.starts_at) }}</div>
              <div class="time-end">{{ formatDateTime(row.ends_at) }}</div>
              <div v-if="isExpired(row)" class="expired-badge">
                <el-tag type="info" size="small">已过期</el-tag>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="created_by" label="创建人" width="120" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              :disabled="isExpired(row)"
              @change="handleToggle(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button
              v-if="!isExpired(row)"
              link
              type="primary"
              @click="handleEdit(row)"
            >
              编辑
            </el-button>
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
      width="800px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="ruleFormRef"
        :model="ruleForm"
        :rules="ruleRules"
        label-width="120px"
      >
        <el-form-item label="备注" prop="comment">
          <el-input
            v-model="ruleForm.comment"
            placeholder="请输入抑制原因或备注"
          />
        </el-form-item>

        <el-form-item label="创建人" prop="created_by">
          <el-input v-model="ruleForm.created_by" placeholder="请输入创建人" />
        </el-form-item>

        <el-form-item label="开始时间" prop="starts_at">
          <el-date-picker
            v-model="ruleForm.starts_at"
            type="datetime"
            placeholder="选择开始时间"
            :disabled-date="disableStartDate"
          />
        </el-form-item>

        <el-form-item label="结束时间" prop="ends_at">
          <el-date-picker
            v-model="ruleForm.ends_at"
            type="datetime"
            placeholder="选择结束时间"
            :disabled-date="disableEndDate"
          />
        </el-form-item>

        <el-divider content-position="left">匹配条件</el-divider>

        <div class="matchers-editor">
          <div
            v-for="(matcher, index) in ruleForm.matchers"
            :key="index"
            class="matcher-row"
          >
            <el-input
              v-model="matcher.name"
              placeholder="标签名"
              style="width: 30%"
            />
            <el-select
              v-model="matcher.is_regex"
              placeholder="匹配类型"
              style="width: 25%"
            >
              <el-option label="等于" :value="false" />
              <el-option label="正则" :value="true" />
            </el-select>
            <el-input
              v-model="matcher.value"
              placeholder="标签值"
              style="width: 35%"
            />
            <el-button
              type="danger"
              :icon="Delete"
              circle
              size="small"
              style="margin-left: 8px"
              @click="removeMatcher(index)"
            />
          </div>
          <el-button
            type="primary"
            :icon="Plus"
            size="small"
            plain
            @click="addMatcher"
          >
            添加条件
          </el-button>
        </div>

        <el-alert
          type="info"
          :closable="false"
          style="margin-top: 16px"
        >
          <template #title>
            <div style="font-size: 13px;">
              所有条件必须同时满足才会抑制告警。支持常见的标签如：alert_name、severity、host、region 等。
            </div>
          </template>
        </el-alert>

        <!-- 快捷模板 -->
        <el-divider content-position="left">快捷模板</el-divider>
        <div class="quick-templates">
          <el-button
            size="small"
            @click="applyTemplate('all_critical')"
          >
            抑制所有严重告警
          </el-button>
          <el-button
            size="small"
            @click="applyTemplate('by_host')"
          >
            按主机抑制
          </el-button>
          <el-button
            size="small"
            @click="applyTemplate('by_alert_name')"
          >
            按告警名称抑制
          </el-button>
          <el-button
            size="small"
            @click="applyTemplate('maintenance')"
          >
            维护窗口
          </el-button>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>

    <!-- 规则查看对话框 -->
    <el-dialog
      v-model="viewDialogVisible"
      title="抑制规则详情"
      width="700px"
    >
      <div v-if="currentRule" class="rule-detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="规则 ID" :span="2">
            <code class="rule-id-code">{{ currentRule.id }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="备注" :span="2">
            {{ currentRule.comment || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="创建人">
            {{ currentRule.created_by }}
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag
              :type="isExpired(currentRule) ? 'info' : (currentRule.enabled ? 'success' : 'warning')"
            >
              {{ getStatusText(currentRule) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">
            {{ formatDateTime(currentRule.starts_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="结束时间">
            {{ formatDateTime(currentRule.ends_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ formatDateTime(currentRule.created_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="更新时间">
            {{ formatDateTime(currentRule.updated_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="detail-section">
          <h4>匹配条件</h4>
          <div class="matchers-detail">
            <div
              v-for="(matcher, index) in currentRule.matchers"
              :key="index"
              class="matcher-item"
            >
              <el-tag :type="matcher.is_regex ? 'warning' : 'info'" size="small">
                {{ matcher.name }} {{ matcher.is_regex ? '=~' : '=' }} {{ matcher.value }}
              </el-tag>
            </div>
            <el-empty
              v-if="!currentRule.matchers || currentRule.matchers.length === 0"
              description="无匹配条件（抑制所有告警）"
              :image-size="80"
            />
          </div>
        </div>

        <div v-if="currentRule.matchers && currentRule.matchers.length > 0" class="detail-section">
          <h4>匹配逻辑</h4>
          <el-alert type="info" :closable="false">
            <template #title>
              <div style="font-size: 13px;">
                当告警的标签同时满足以上所有条件时，该告警将被抑制。
              </div>
            </template>
          </el-alert>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import {
  listSilenceRules,
  createSilenceRule,
  updateSilenceRule,
  deleteSilenceRule,
  toggleSilenceRule
} from '@/api/alert'
import dayjs from 'dayjs'

// 数据状态
const loading = ref(false)
const tableData = ref([])
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
  status: '',
  created_by: ''
})

// 对话框状态
const editDialogVisible = ref(false)
const viewDialogVisible = ref(false)

// 规则表单
const ruleForm = reactive({
  comment: '',
  created_by: '',
  starts_at: null,
  ends_at: null,
  matchers: []
})

// 表单验证规则
const ruleRules = {
  comment: [
    { required: true, message: '请输入备注', trigger: 'blur' }
  ],
  created_by: [
    { required: true, message: '请输入创建人', trigger: 'blur' }
  ],
  starts_at: [
    { required: true, message: '请选择开始时间', trigger: 'change' }
  ],
  ends_at: [
    { required: true, message: '请选择结束时间', trigger: 'change' }
  ]
}

// 计算属性
const dialogTitle = computed(() => isEdit.value ? '编辑抑制规则' : '新建抑制规则')

// 加载规则列表
const loadRules = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...searchForm
    }
    const response = await listSilenceRules(params)
    tableData.value = response.data?.silences || []
    pagination.total = response.data?.total || 0
  } catch (error) {
    ElMessage.error('加载规则列表失败')
  } finally {
    loading.value = false
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
    status: '',
    created_by: ''
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
    comment: row.comment || '',
    created_by: row.created_by,
    starts_at: new Date(row.starts_at),
    ends_at: new Date(row.ends_at),
    matchers: row.matchers ? [...row.matchers] : []
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
      `确定要删除抑制规则吗？`,
      '删除规则',
      {
        type: 'warning'
      }
    )
    await deleteSilenceRule(row.id)
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
    await toggleSilenceRule(row.id, row.enabled)
    ElMessage.success(row.enabled ? '规则已启用' : '规则已禁用')
  } catch (error) {
    row.enabled = !row.enabled
    ElMessage.error('操作失败')
  }
}

// 添加匹配条件
const addMatcher = () => {
  ruleForm.matchers.push({
    name: '',
    is_regex: false,
    value: ''
  })
}

// 移除匹配条件
const removeMatcher = (index) => {
  ruleForm.matchers.splice(index, 1)
}

// 重置表单
const resetRuleForm = () => {
  Object.assign(ruleForm, {
    comment: '',
    created_by: '',
    starts_at: new Date(),
    ends_at: dayjs().add(1, 'hour').toDate(),
    matchers: []
  })
}

// 禁用开始日期
const disableStartDate = (time) => {
  return false // 可以选择任何日期
}

// 禁用结束日期
const disableEndDate = (time) => {
  if (!ruleForm.starts_at) return false
  return time.getTime() < ruleForm.starts_at.getTime()
}

// 应用模板
const applyTemplate = (template) => {
  switch (template) {
    case 'all_critical':
      ruleForm.comment = '抑制所有严重告警'
      ruleForm.matchers = [
        { name: 'severity', is_regex: false, value: 'critical' }
      ]
      break
    case 'by_host':
      ruleForm.comment = '按主机抑制告警'
      ruleForm.matchers = [
        { name: 'host', is_regex: false, value: '' }
      ]
      break
    case 'by_alert_name':
      ruleForm.comment = '按告警名称抑制'
      ruleForm.matchers = [
        { name: 'alert_name', is_regex: false, value: '' }
      ]
      break
    case 'maintenance':
      ruleForm.comment = '维护窗口'
      ruleForm.starts_at = new Date()
      ruleForm.ends_at = dayjs().add(2, 'hour').toDate()
      ruleForm.matchers = []
      break
  }
}

// 保存规则
const handleSave = async () => {
  try {
    await ruleFormRef.value.validate()

    // 过滤空的条件
    const matchers = ruleForm.matchers.filter(m => m.name && m.value)

    const data = {
      comment: ruleForm.comment,
      created_by: ruleForm.created_by,
      starts_at: dayjs(ruleForm.starts_at).toISOString(),
      ends_at: dayjs(ruleForm.ends_at).toISOString(),
      matchers
    }

    if (isEdit.value) {
      await updateSilenceRule(ruleForm.id, data)
      ElMessage.success('更新成功')
    } else {
      await createSilenceRule(data)
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

const isExpired = (rule) => {
  return dayjs().isAfter(dayjs(rule.ends_at))
}

const getStatusText = (rule) => {
  if (isExpired(rule)) return '已过期'
  return rule.enabled ? '活跃' : '已禁用'
}

// 生命周期
onMounted(() => {
  loadRules()
})
</script>

<style scoped>
.silence-rules-page {
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

.rule-id-text {
  font-family: monospace;
  font-size: 12px;
}

.matchers-display {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.time-range {
  font-size: 13px;
}

.time-start,
.time-end {
  margin-bottom: 4px;
}

.time-end {
  color: var(--el-text-color-secondary);
}

.expired-badge {
  margin-top: 4px;
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.matcher-row {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.matchers-editor {
  padding: 16px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
}

.quick-templates {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
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

.rule-id-code {
  display: block;
  padding: 8px 12px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}

.matchers-detail {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px;
  background-color: var(--el-fill-color-light);
  border-radius: 4px;
}

.matcher-item {
  margin-bottom: 8px;
}
</style>
