<template>
  <div class="config-list-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>配置中心</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建配置
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="命名空间">
          <el-select
            v-model="searchForm.namespace"
            placeholder="请选择命名空间"
            clearable
            filterable
            @change="loadGroups"
          >
            <el-option
              v-for="ns in namespaces"
              :key="ns.name"
              :label="ns.name"
              :value="ns.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="分组">
          <el-select
            v-model="searchForm.group"
            placeholder="请选择分组"
            clearable
            filterable
            :disabled="!searchForm.namespace"
          >
            <el-option
              v-for="group in groups"
              :key="group.name"
              :label="group.name"
              :value="group.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="DataID">
          <el-input
            v-model="searchForm.data_id"
            placeholder="请输入 DataID"
            clearable
          />
        </el-form-item>
        <el-form-item label="配置类型">
          <el-select v-model="searchForm.type" placeholder="请选择类型" clearable>
            <el-option label="全部" value="" />
            <el-option label="YAML" value="yaml" />
            <el-option label="JSON" value="json" />
            <el-option label="Properties" value="properties" />
            <el-option label="Text" value="text" />
          </el-select>
        </el-form-item>
        <el-form-item label="标签">
          <el-select
            v-model="searchForm.tags"
            placeholder="请选择标签"
            clearable
            multiple
            collapse-tags
          >
            <el-option
              v-for="tag in allTags"
              :key="tag"
              :label="tag"
              :value="tag"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 批量操作栏 -->
      <div v-if="selectedRows.length > 0" class="batch-actions">
        <span class="selection-info">已选择 {{ selectedRows.length }} 项</span>
        <el-button size="small" @click="handleBatchPublish">批量发布</el-button>
        <el-button size="small" type="danger" @click="handleBatchDelete">批量删除</el-button>
      </div>

      <!-- 配置列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="namespace" label="命名空间" width="120" />
        <el-table-column prop="group" label="分组" width="120" />
        <el-table-column prop="data_id" label="DataID" min-width="200">
          <template #default="{ row }">
            <div class="data-id-cell">
              <el-icon class="type-icon" :class="`type-${row.type}`">
                <component :is="getTypeIcon(row.type)" />
              </el-icon>
              <span class="data-id-text">{{ row.data_id }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="type" label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="getTypeTagType(row.type)" size="small">
              {{ row.type.toUpperCase() }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="tags" label="标签" width="150">
          <template #default="{ row }">
            <el-tag
              v-for="tag in row.tags"
              :key="tag"
              size="small"
              class="tag-item"
            >
              {{ tag }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="80" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.published ? 'success' : 'info'" size="small">
              {{ row.published ? '已发布' : '未发布' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="handlePublish(row)">发布</el-button>
            <el-button link type="primary" @click="handleHistory(row)">历史</el-button>
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

    <!-- 配置编辑对话框 -->
    <ConfigEditDialog
      v-model="editDialogVisible"
      :config-id="currentConfigId"
      @success="handleEditSuccess"
    />

    <!-- 发布对话框 -->
    <ConfigPublishDialog
      v-model="publishDialogVisible"
      :config-id="currentConfigId"
      @success="handlePublishSuccess"
    />

    <!-- 历史记录对话框 -->
    <ConfigHistoryDialog
      v-model="historyDialogVisible"
      :config-id="currentConfigId"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Document, Ticket, Grid } from '@element-plus/icons-vue'
import { configApi } from '@/api/config'
import dayjs from 'dayjs'
import ConfigEditDialog from './config/ConfigEditDialog.vue'
import ConfigPublishDialog from './config/ConfigPublishDialog.vue'
import ConfigHistoryDialog from './config/ConfigHistoryDialog.vue'

const router = useRouter()

const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const namespaces = ref([])
const groups = ref([])
const allTags = ref([])

// 对话框状态
const editDialogVisible = ref(false)
const publishDialogVisible = ref(false)
const historyDialogVisible = ref(false)
const currentConfigId = ref(null)

const searchForm = reactive({
  namespace: '',
  group: '',
  data_id: '',
  type: '',
  tags: []
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 获取配置类型图标
const getTypeIcon = (type) => {
  switch (type) {
    case 'yaml':
    case 'json':
      return Document
    case 'properties':
      return Ticket
    default:
      return Grid
  }
}

// 获取配置类型标签颜色
const getTypeTagType = (type) => {
  switch (type) {
    case 'yaml':
      return 'success'
    case 'json':
      return 'warning'
    case 'properties':
      return 'danger'
    default:
      return 'info'
  }
}

// 加载命名空间列表
const loadNamespaces = async () => {
  try {
    const res = await configApi.listNamespaces()
    if (res.success) {
      namespaces.value = res.data || []
    }
  } catch (error) {
    console.error('Failed to load namespaces:', error)
  }
}

// 加载分组列表
const loadGroups = async () => {
  if (!searchForm.namespace) {
    groups.value = []
    return
  }
  try {
    const res = await configApi.listGroups(searchForm.namespace)
    if (res.success) {
      groups.value = res.data || []
    }
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

// 加载标签列表
const loadTags = async () => {
  try {
    const res = await configApi.listTags()
    if (res.success) {
      allTags.value = res.data || []
    }
  } catch (error) {
    console.error('Failed to load tags:', error)
  }
}

// 加载配置列表
const loadConfigs = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize
    }
    if (searchForm.namespace) params.namespace = searchForm.namespace
    if (searchForm.group) params.group = searchForm.group
    if (searchForm.data_id) params.data_id = searchForm.data_id
    if (searchForm.type) params.type = searchForm.type
    if (searchForm.tags.length > 0) params.tags = searchForm.tags.join(',')

    const res = await configApi.listConfigs(params)
    if (res.success) {
      tableData.value = res.data.items || []
      pagination.total = res.data.total || 0
    }
  } catch (error) {
    ElMessage.error('加载配置列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadConfigs()
}

// 重置
const handleReset = () => {
  searchForm.namespace = ''
  searchForm.group = ''
  searchForm.data_id = ''
  searchForm.type = ''
  searchForm.tags = []
  groups.value = []
  handleSearch()
}

// 分页变化
const handlePageChange = (page) => {
  pagination.page = page
  loadConfigs()
}

const handleSizeChange = (size) => {
  pagination.pageSize = size
  pagination.page = 1
  loadConfigs()
}

// 选择变化
const handleSelectionChange = (selection) => {
  selectedRows.value = selection
}

// 新建配置
const handleCreate = () => {
  currentConfigId.value = null
  editDialogVisible.value = true
}

// 查看配置
const handleView = (row) => {
  currentConfigId.value = row.id
  editDialogVisible.value = true
}

// 编辑配置
const handleEdit = (row) => {
  currentConfigId.value = row.id
  editDialogVisible.value = true
}

// 发布配置
const handlePublish = (row) => {
  currentConfigId.value = row.id
  publishDialogVisible.value = true
}

// 历史记录
const handleHistory = (row) => {
  currentConfigId.value = row.id
  historyDialogVisible.value = true
}

// 删除配置
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除配置 "${row.data_id}" 吗？删除后无法恢复。`,
      '警告',
      { type: 'warning' }
    )
    await configApi.deleteConfig(row.id)
    ElMessage.success('删除成功')
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 批量发布
const handleBatchPublish = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要批量发布选中的 ${selectedRows.value.length} 个配置吗？`,
      '确认',
      { type: 'warning' }
    )
    const configIds = selectedRows.value.map(row => row.id)
    await configApi.batchAction({
      action: 'publish',
      config_ids: configIds
    })
    ElMessage.success('批量发布成功')
    selectedRows.value = []
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('批量发布失败')
    }
  }
}

// 批量删除
const handleBatchDelete = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要批量删除选中的 ${selectedRows.value.length} 个配置吗？删除后无法恢复。`,
      '警告',
      { type: 'warning' }
    )
    const configIds = selectedRows.value.map(row => row.id)
    await configApi.batchAction({
      action: 'delete',
      config_ids: configIds
    })
    ElMessage.success('批量删除成功')
    selectedRows.value = []
    loadConfigs()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('批量删除失败')
    }
  }
}

// 编辑成功回调
const handleEditSuccess = () => {
  editDialogVisible.value = false
  loadConfigs()
}

// 发布成功回调
const handlePublishSuccess = () => {
  publishDialogVisible.value = false
  loadConfigs()
}

onMounted(() => {
  loadNamespaces()
  loadTags()
  loadConfigs()
})
</script>

<style scoped>
.config-list-page {
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

.batch-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  margin-bottom: 16px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.selection-info {
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.data-id-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.type-icon {
  font-size: 16px;
}

.type-yaml {
  color: var(--el-color-success);
}

.type-json {
  color: var(--el-color-warning);
}

.type-properties {
  color: var(--el-color-danger);
}

.data-id-text {
  font-family: monospace;
}

.tag-item {
  margin-right: 4px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
