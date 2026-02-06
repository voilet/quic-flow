<template>
  <div class="subscribers-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>订阅者管理</span>
          <el-button type="primary" @click="handleRefresh">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="客户端ID">
          <el-input
            v-model="searchForm.client_id"
            placeholder="请输入客户端ID"
            clearable
          />
        </el-form-item>
        <el-form-item label="SDK类型">
          <el-select v-model="searchForm.sdk_type" placeholder="请选择类型" clearable>
            <el-option label="全部" value="" />
            <el-option label="Go" value="go" />
            <el-option label="Python" value="python" />
            <el-option label="Java" value="java" />
            <el-option label="JavaScript" value="javascript" />
          </el-select>
        </el-form-item>
        <el-form-item label="命名空间">
          <el-input
            v-model="searchForm.namespace"
            placeholder="请输入命名空间"
            clearable
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.status" placeholder="请选择状态" clearable>
            <el-option label="全部" value="" />
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 订阅者列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="client_id" label="客户端ID" min-width="200" />
        <el-table-column label="SDK类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getSdkTypeTag(row.sdk_type)" size="small">
              {{ row.sdk_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="namespace" label="命名空间" width="150" />
        <el-table-column label="订阅配置" min-width="300">
          <template #default="{ row }">
            <div class="subscriptions-display">
              <el-tag
                v-for="(sub, index) in row.subscriptions"
                :key="index"
                size="small"
                class="subscription-tag"
              >
                {{ sub }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="client_ip" label="客户端IP" width="150" />
        <el-table-column label="标签" min-width="200">
          <template #default="{ row }">
            <el-tag
              v-for="(tag, index) in row.client_tags"
              :key="index"
              size="small"
              class="tag-item"
            >
              {{ tag }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="最后活跃" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.last_active) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">
              {{ row.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button link type="danger" @click="handleDisconnect(row)">断开</el-button>
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
      title="订阅者详情"
      width="800px"
    >
      <el-descriptions :column="2" border v-if="viewData">
        <el-descriptions-item label="客户端ID">{{ viewData.client_id }}</el-descriptions-item>
        <el-descriptions-item label="SDK类型">
          <el-tag :type="getSdkTypeTag(viewData.sdk_type)" size="small">
            {{ viewData.sdk_type }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="命名空间">{{ viewData.namespace }}</el-descriptions-item>
        <el-descriptions-item label="客户端IP">{{ viewData.client_ip }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="viewData.status === 'online' ? 'success' : 'info'" size="small">
            {{ viewData.status === 'online' ? '在线' : '离线' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="最后活跃">
          {{ formatDateTime(viewData.last_active) }}
        </el-descriptions-item>
        <el-descriptions-item label="订阅配置" :span="2">
          <div class="subscriptions-display">
            <el-tag
              v-for="(sub, index) in viewData.subscriptions"
              :key="index"
              size="small"
              class="subscription-tag"
            >
              {{ sub }}
            </el-tag>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="标签" :span="2">
          <el-tag
            v-for="(tag, index) in viewData.client_tags"
            :key="index"
            size="small"
            class="tag-item"
          >
            {{ tag }}
          </el-tag>
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
  client_id: '',
  sdk_type: '',
  namespace: '',
  status: ''
})

const pagination = reactive({
  page: 1,
  size: 20,
  total: 0
})

// 获取SDK类型标签
const getSdkTypeTag = (type) => {
  const map = {
    go: 'success',
    python: 'warning',
    java: 'info',
    javascript: 'primary'
  }
  return map[type] || ''
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
    const res = await configApi.listSubscribers(params)
    if (res.success) {
      tableData.value = res.items || []
      pagination.total = res.total || 0
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
    client_id: '',
    sdk_type: '',
    namespace: '',
    status: ''
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

// 断开连接
const handleDisconnect = async (row) => {
  try {
    await ElMessageBox.confirm('确定要断开该客户端连接吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await configApi.disconnectSubscriber(row.client_id)
    ElMessage.success('断开成功')
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '断开失败')
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
  // 定时刷新
  setInterval(() => {
    loadData()
  }, 30000) // 每30秒刷新一次
})
</script>

<style scoped>
.subscribers-page {
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

.subscriptions-display {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.subscription-tag {
  margin: 2px;
}

.tag-item {
  margin-right: 4px;
  margin-bottom: 4px;
}
</style>
