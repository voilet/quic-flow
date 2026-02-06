<template>
  <div class="releases-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>发布管理</span>
          <el-button type="primary" @click="handleRefresh">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="配置ID">
          <el-input
            v-model="searchForm.config_id"
            placeholder="请输入配置ID"
            clearable
            type="number"
          />
        </el-form-item>
        <el-form-item label="发布类型">
          <el-select v-model="searchForm.release_type" placeholder="请选择类型" clearable>
            <el-option label="全部" value="" />
            <el-option label="全量发布" value="full" />
            <el-option label="灰度发布" value="gray" />
            <el-option label="回滚" value="rollback" />
          </el-select>
        </el-form-item>
        <el-form-item label="发布状态">
          <el-select v-model="searchForm.status" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="待发布" value="pending" />
            <el-option label="发布中" value="publishing" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
        </el-form-item>
        <el-form-item label="发布人">
          <el-input
            v-model="searchForm.released_by"
            placeholder="请输入发布人"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 发布记录列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="id" label="发布ID" width="120" />
        <el-table-column prop="config_id" label="配置ID" width="120" />
        <el-table-column prop="namespace" label="命名空间" width="150" />
        <el-table-column prop="group" label="分组" width="120" />
        <el-table-column prop="data_id" label="DataID" min-width="150" />
        <el-table-column label="发布类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getReleaseTypeTag(row.release_type)" size="small">
              {{ getReleaseTypeLabel(row.release_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发布状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusTag(row.status)" size="small">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发布进度" width="150">
          <template #default="{ row }">
            <el-progress
              :percentage="getProgress(row)"
              :status="row.status === 'failed' ? 'exception' : ''"
            />
          </template>
        </el-table-column>
        <el-table-column prop="released_by" label="发布人" width="120" />
        <el-table-column label="发布时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.released_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button
              v-if="row.status === 'publishing'"
              link
              type="warning"
              @click="handleCancel(row)"
            >
              取消
            </el-button>
            <el-button
              v-if="row.status === 'success'"
              link
              type="success"
              @click="handleRollback(row)"
            >
              回滚
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.size"
          :total="pagination.total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 查看对话框 -->
    <el-dialog
      v-model="viewDialogVisible"
      title="发布详情"
      width="900px"
    >
      <el-descriptions :column="2" border v-if="viewData">
        <el-descriptions-item label="发布ID">{{ viewData.id }}</el-descriptions-item>
        <el-descriptions-item label="配置ID">{{ viewData.config_id }}</el-descriptions-item>
        <el-descriptions-item label="命名空间">{{ viewData.namespace }}</el-descriptions-item>
        <el-descriptions-item label="分组">{{ viewData.group }}</el-descriptions-item>
        <el-descriptions-item label="DataID">{{ viewData.data_id }}</el-descriptions-item>
        <el-descriptions-item label="发布类型">
          <el-tag :type="getReleaseTypeTag(viewData.release_type)" size="small">
            {{ getReleaseTypeLabel(viewData.release_type) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="发布状态">
          <el-tag :type="getStatusTag(viewData.status)" size="small">
            {{ getStatusLabel(viewData.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="发布人">{{ viewData.released_by }}</el-descriptions-item>
        <el-descriptions-item label="发布时间">
          {{ formatDateTime(viewData.released_at) }}
        </el-descriptions-item>
        <el-descriptions-item label="目标总数">{{ viewData.total_targets || 0 }}</el-descriptions-item>
        <el-descriptions-item label="成功数">
          <span style="color: var(--el-color-success)">{{ viewData.success_count || 0 }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="失败数">
          <span style="color: var(--el-color-danger)">{{ viewData.failed_count || 0 }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="是否灰度">
          <el-tag :type="viewData.is_gray ? 'warning' : 'info'" size="small">
            {{ viewData.is_gray ? '是' : '否' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="备注" :span="2">
          {{ viewData.comment || '-' }}
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="viewDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { configApi } from '@/api/config'

const loading = ref(false)
const tableData = ref([])
const viewDialogVisible = ref(false)
const viewData = ref(null)

const searchForm = reactive({
  config_id: '',
  release_type: '',
  status: '',
  released_by: ''
})

const pagination = reactive({
  page: 1,
  size: 20,
  total: 0
})

// 获取发布类型标签
const getReleaseTypeTag = (type) => {
  const map = {
    full: 'success',
    gray: 'warning',
    rollback: 'info'
  }
  return map[type] || ''
}

// 获取发布类型标签文本
const getReleaseTypeLabel = (type) => {
  const map = {
    full: '全量发布',
    gray: '灰度发布',
    rollback: '回滚'
  }
  return map[type] || type
}

// 获取状态标签
const getStatusTag = (status) => {
  const map = {
    pending: 'info',
    publishing: 'warning',
    success: 'success',
    failed: 'danger'
  }
  return map[status] || ''
}

// 获取状态标签文本
const getStatusLabel = (status) => {
  const map = {
    pending: '待发布',
    publishing: '发布中',
    success: '成功',
    failed: '失败'
  }
  return map[status] || status
}

// 获取发布进度
const getProgress = (row) => {
  if (!row.total_targets || row.total_targets === 0) return 0
  const success = row.success_count || 0
  const failed = row.failed_count || 0
  return Math.round(((success + failed) / row.total_targets) * 100)
}

// 格式化时间
const formatDateTime = (dateTime) => {
  if (!dateTime) return '-'
  const date = new Date(dateTime)
  return date.toLocaleString('zh-CN')
}

// 加载数据
const loadData = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.size,
      ...searchForm
    }
    // 清理空值
    Object.keys(params).forEach(key => {
      if (params[key] === '' || params[key] === null || params[key] === undefined) {
        delete params[key]
      }
    })
    
    const res = await configApi.listReleases(params)
    if (res && res.success !== false) {
      // 处理响应数据格式
      tableData.value = res.items || res.data || []
      pagination.total = res.total || 0
    } else {
      ElMessage.error(res?.error || '加载失败')
    }
  } catch (error) {
    ElMessage.error(error.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadData()
}

// 重置
const handleReset = () => {
  Object.assign(searchForm, {
    config_id: '',
    release_type: '',
    status: '',
    released_by: ''
  })
  handleSearch()
}

// 刷新
const handleRefresh = () => {
  loadData()
}

// 查看
const handleView = (row) => {
  viewData.value = { ...row }
  viewDialogVisible.value = true
}

// 取消发布
const handleCancel = async (row) => {
  try {
    await ElMessageBox.confirm('确定要取消该发布吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await configApi.cancelRelease(row.id)
    ElMessage.success('取消成功')
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || error.message || '取消失败')
    }
  }
}

// 回滚
const handleRollback = async (row) => {
  try {
    await ElMessageBox.confirm('确定要回滚该发布吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    // 回滚需要配置ID和版本号
    await configApi.rollbackConfig(row.config_id, {
      to_version: row.previous_version || row.version - 1,
      comment: '从发布管理页面回滚'
    })
    ElMessage.success('回滚成功')
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || error.message || '回滚失败')
    }
  }
}

// 分页
const handleSizeChange = () => {
  loadData()
}

const handlePageChange = () => {
  loadData()
}

onMounted(() => {
  loadData()
  // 定时刷新发布中的记录
  setInterval(() => {
    const hasPublishing = tableData.value.some(row => row.status === 'publishing')
    if (hasPublishing) {
      loadData()
    }
  }, 5000) // 每5秒刷新一次发布中的记录
})
</script>

<style scoped>
.releases-page {
  padding: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.search-form {
  margin-bottom: 16px;
}

.pagination-container {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
