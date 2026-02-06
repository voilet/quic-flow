<template>
  <div class="oncall-schedule-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>值班管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建值班表
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="值班表名称">
          <el-input
            v-model="searchForm.name"
            placeholder="请输入值班表名称"
            clearable
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="searchForm.enabled" placeholder="请选择状态" clearable>
            <el-option label="全部" :value="null" />
            <el-option label="启用" :value="true" />
            <el-option label="禁用" :value="false" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 值班表列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="name" label="值班表名称" min-width="200" />
        <el-table-column prop="description" label="描述" min-width="250" />
        <el-table-column prop="timezone" label="时区" width="150" />
        <el-table-column label="当前值班" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.current_oncall" type="success" size="small">
              {{ row.current_oncall }}
            </el-tag>
            <span v-else class="text-gray">暂无</span>
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
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleView(row)">查看</el-button>
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
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

    <!-- 编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="800px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="值班表名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入值班表名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入描述"
          />
        </el-form-item>
        <el-form-item label="时区" prop="timezone">
          <el-select v-model="form.timezone" placeholder="请选择时区" style="width: 100%">
            <el-option label="UTC" value="UTC" />
            <el-option label="Asia/Shanghai" value="Asia/Shanghai" />
            <el-option label="America/New_York" value="America/New_York" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '@/api'

const loading = ref(false)
const tableData = ref([])
const dialogVisible = ref(false)
const dialogTitle = ref('新建值班表')
const formRef = ref(null)

const searchForm = reactive({
  name: '',
  enabled: null
})

const pagination = reactive({
  page: 1,
  size: 20,
  total: 0
})

const form = reactive({
  id: null,
  name: '',
  description: '',
  timezone: 'Asia/Shanghai',
  enabled: true
})

const rules = {
  name: [
    { required: true, message: '请输入值班表名称', trigger: 'blur' }
  ],
  timezone: [
    { required: true, message: '请选择时区', trigger: 'change' }
  ]
}

// 加载数据
const loadData = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      size: pagination.size,
      ...searchForm
    }
    const res = await api.request.get('/alert/oncall-schedules', { params })
    if (res.success) {
      tableData.value = res.data?.items || []
      pagination.total = res.data?.total || 0
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
    name: '',
    enabled: null
  })
  handleSearch()
}

// 新建
const handleCreate = () => {
  dialogTitle.value = '新建值班表'
  Object.assign(form, {
    id: null,
    name: '',
    description: '',
    timezone: 'Asia/Shanghai',
    enabled: true
  })
  dialogVisible.value = true
}

// 查看
const handleView = (row) => {
  ElMessage.info('查看功能开发中')
}

// 编辑
const handleEdit = (row) => {
  dialogTitle.value = '编辑值班表'
  Object.assign(form, {
    id: row.id,
    name: row.name,
    description: row.description || '',
    timezone: row.timezone || 'Asia/Shanghai',
    enabled: row.enabled
  })
  dialogVisible.value = true
}

// 删除
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm('确定要删除该值班表吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await api.request.delete(`/alert/oncall-schedules/${row.id}`)
    ElMessage.success('删除成功')
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '删除失败')
    }
  }
}

// 切换状态
const handleToggle = async (row) => {
  try {
    await api.request.put(`/alert/oncall-schedules/${row.id}/toggle`, {
      enabled: row.enabled
    })
    ElMessage.success('操作成功')
  } catch (error) {
    row.enabled = !row.enabled
    ElMessage.error(error.message || '操作失败')
  }
}

// 提交表单
const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (valid) {
      try {
        if (form.id) {
          await api.request.put(`/alert/oncall-schedules/${form.id}`, form)
          ElMessage.success('更新成功')
        } else {
          await api.request.post('/alert/oncall-schedules', form)
          ElMessage.success('创建成功')
        }
        dialogVisible.value = false
        loadData()
      } catch (error) {
        ElMessage.error(error.message || '操作失败')
      }
    }
  })
}

// 对话框关闭
const handleDialogClose = () => {
  formRef.value?.resetFields()
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
})
</script>

<style scoped>
.oncall-schedule-page {
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

.text-gray {
  color: var(--el-text-color-secondary);
}
</style>
