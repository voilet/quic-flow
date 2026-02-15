<template>
  <div class="task-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>任务列表</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建任务
          </el-button>
        </div>
      </template>

      <!-- 搜索栏 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="任务名称">
          <el-input
            v-model="searchForm.keyword"
            placeholder="请输入任务名称"
            clearable
            @keyup.enter="handleSearch"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="启用" :value="1" />
            <el-option label="禁用" :value="0" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 表格 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="任务名称" min-width="150" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="executor_type" label="执行器类型" width="120">
          <template #default="{ row }">
            <el-tag :type="row.executor_type === 1 ? 'success' : 'info'">
              {{ row.executor_type === 1 ? 'Shell' : 'HTTP' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="cron_expr" label="Cron 表达式" width="150" />
        <el-table-column label="执行统计" width="140">
          <template #default="{ row }">
            <div class="execution-stats">
              <span class="stats-item">
                <el-tag size="small" type="info">{{ row.execution_count || 0 }} 次</el-tag>
              </span>
              <span class="stats-item">
                <el-tag size="small" type="success">{{ row.success_count || 0 }}</el-tag>
              </span>
              <span class="stats-item">
                <el-tag size="small" type="danger">{{ row.failed_count || 0 }}</el-tag>
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button
              link
              :type="row.status === 1 ? 'warning' : 'success'"
              @click="handleToggleStatus(row)"
            >
              {{ row.status === 1 ? '禁用' : '启用' }}
            </el-button>
            <el-button link type="primary" @click="handleTrigger(row)">触发</el-button>
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

    <!-- 任务表单对话框 -->
    <TaskForm
      v-model="formVisible"
      :task-id="currentTaskId"
      @success="handleFormSuccess"
    />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { taskApi } from '@/api/task'
import { useWebSocket } from '@/composables/useWebSocket'
import TaskForm from './Form.vue'
import dayjs from 'dayjs'

const loading = ref(false)
const tableData = ref([])
const selectedRows = ref([])
const formVisible = ref(false)
const currentTaskId = ref(null)

const searchForm = reactive({
  keyword: '',
  status: ''
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

// 加载任务列表
const loadTasks = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      keyword: searchForm.keyword || undefined,
      status: searchForm.status !== '' ? searchForm.status : undefined
    }
    const res = await taskApi.listTasks(params)
    if (res.success) {
      tableData.value = res.data.tasks || []
      pagination.total = res.data.total || 0
    }
  } catch (error) {
    ElMessage.error('加载任务列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadTasks()
}

// 重置
const handleReset = () => {
  searchForm.keyword = ''
  searchForm.status = ''
  handleSearch()
}

// 分页变化
const handlePageChange = (page) => {
  pagination.page = page
  loadTasks()
}

const handleSizeChange = (size) => {
  pagination.pageSize = size
  pagination.page = 1
  loadTasks()
}

// 选择变化
const handleSelectionChange = (selection) => {
  selectedRows.value = selection
}

// 新建任务
const handleCreate = () => {
  currentTaskId.value = null
  formVisible.value = true
}

// 查看任务
const handleView = (row) => {
  currentTaskId.value = row.id
  formVisible.value = true
}

// 编辑任务
const handleEdit = (row) => {
  currentTaskId.value = row.id
  formVisible.value = true
}

// 切换状态
const handleToggleStatus = async (row) => {
  try {
    if (row.status === 1) {
      await taskApi.disableTask(row.id)
      ElMessage.success('任务已禁用')
    } else {
      await taskApi.enableTask(row.id)
      ElMessage.success('任务已启用')
    }
    loadTasks()
  } catch (error) {
    ElMessage.error('操作失败')
  }
}

// 触发任务
const handleTrigger = async (row) => {
  try {
    await ElMessageBox.confirm('确定要手动触发此任务吗？', '提示', {
      type: 'warning'
    })
    await taskApi.triggerTask(row.id)
    ElMessage.success('任务已触发')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('触发任务失败')
    }
  }
}

// 删除任务
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm('确定要删除此任务吗？删除后无法恢复。', '警告', {
      type: 'warning'
    })
    await taskApi.deleteTask(row.id)
    ElMessage.success('删除成功')
    loadTasks()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 表单成功回调
const handleFormSuccess = () => {
  formVisible.value = false
  loadTasks()
}

// WebSocket 连接（实时更新任务状态）
const { connected } = useWebSocket('/api/ws/tasks', {
  onMessage: (data) => {
    console.log('WebSocket message received:', data)
    if (data.type === 'task_status' || data.type === 'task_created' || data.type === 'task_deleted') {
      loadTasks()
    }
  },
  onError: (err) => {
    console.error('WebSocket error:', err)
  },
  reconnect: true
})

onMounted(() => {
  loadTasks()
})

onUnmounted(() => {
  // WebSocket 会在 composable 中自动断开
})
</script>

<style scoped>
.task-list {
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.execution-stats {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.stats-item {
  margin-right: 2px;
}
</style>
