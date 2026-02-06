<template>
  <div class="gray-rules-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>灰度规则管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建规则
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
        <el-form-item label="规则类型">
          <el-select v-model="searchForm.rule_type" placeholder="请选择类型" clearable>
            <el-option label="全部" value="" />
            <el-option label="标签" value="tag" />
            <el-option label="IP" value="ip" />
            <el-option label="客户端ID" value="client_id" />
            <el-option label="百分比" value="percentage" />
          </el-select>
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

      <!-- 规则列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="id" label="规则ID" width="120" />
        <el-table-column prop="config_id" label="配置ID" width="120" />
        <el-table-column prop="rule_name" label="规则名称" min-width="200" />
        <el-table-column label="规则类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getRuleTypeTag(row.rule_type)" size="small">
              {{ getRuleTypeLabel(row.rule_type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="规则值" min-width="200">
          <template #default="{ row }">
            <el-tooltip :content="row.rule_value" placement="top">
              <span class="rule-value-text">{{ row.rule_value }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="100" />
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
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="success" @click="handlePromote(row)">全量发布</el-button>
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
      width="700px"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
      >
        <el-form-item label="配置ID" prop="config_id">
          <el-input
            v-model.number="form.config_id"
            placeholder="请输入配置ID"
            type="number"
          />
        </el-form-item>
        <el-form-item label="规则名称" prop="rule_name">
          <el-input v-model="form.rule_name" placeholder="请输入规则名称" />
        </el-form-item>
        <el-form-item label="规则类型" prop="rule_type">
          <el-select v-model="form.rule_type" placeholder="请选择规则类型" style="width: 100%">
            <el-option label="标签" value="tag" />
            <el-option label="IP" value="ip" />
            <el-option label="客户端ID" value="client_id" />
            <el-option label="百分比" value="percentage" />
          </el-select>
        </el-form-item>
        <el-form-item label="规则值" prop="rule_value">
          <el-input
            v-model="form.rule_value"
            type="textarea"
            :rows="4"
            placeholder="请输入规则值（JSON格式）"
          />
        </el-form-item>
        <el-form-item label="优先级" prop="priority">
          <el-input-number v-model="form.priority" :min="0" :max="100" />
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
import { configApi } from '@/api/config'

const loading = ref(false)
const tableData = ref([])
const dialogVisible = ref(false)
const dialogTitle = ref('新建灰度规则')
const formRef = ref(null)

const searchForm = reactive({
  config_id: '',
  rule_type: '',
  enabled: null
})

const pagination = reactive({
  page: 1,
  size: 20,
  total: 0
})

const form = reactive({
  id: null,
  config_id: null,
  rule_name: '',
  rule_type: 'tag',
  rule_value: '',
  priority: 0,
  enabled: true
})

const rules = {
  config_id: [
    { required: true, message: '请输入配置ID', trigger: 'blur' },
    { type: 'number', message: '配置ID必须为数字', trigger: 'blur' }
  ],
  rule_name: [
    { required: true, message: '请输入规则名称', trigger: 'blur' }
  ],
  rule_type: [
    { required: true, message: '请选择规则类型', trigger: 'change' }
  ],
  rule_value: [
    { required: true, message: '请输入规则值', trigger: 'blur' }
  ]
}

// 获取规则类型标签
const getRuleTypeTag = (type) => {
  const map = {
    tag: 'success',
    ip: 'warning',
    client_id: 'info',
    percentage: 'primary'
  }
  return map[type] || ''
}

// 获取规则类型标签文本
const getRuleTypeLabel = (type) => {
  const map = {
    tag: '标签',
    ip: 'IP',
    client_id: '客户端ID',
    percentage: '百分比'
  }
  return map[type] || type
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
    // 从URL获取config_id（如果存在）
    const urlParams = new URLSearchParams(window.location.search)
    const configId = urlParams.get('config_id')
    if (configId && !params.config_id) {
      params.config_id = configId
      searchForm.config_id = configId
    }
    const res = await configApi.listAllGrayRules(params)
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
    config_id: '',
    rule_type: '',
    enabled: null
  })
  handleSearch()
}

// 新建
const handleCreate = () => {
  dialogTitle.value = '新建灰度规则'
  Object.assign(form, {
    id: null,
    config_id: null,
    rule_name: '',
    rule_type: 'tag',
    rule_value: '',
    priority: 0,
    enabled: true
  })
  dialogVisible.value = true
}

// 编辑
const handleEdit = (row) => {
  dialogTitle.value = '编辑灰度规则'
  Object.assign(form, {
    id: row.id,
    config_id: row.config_id,
    rule_name: row.rule_name,
    rule_type: row.rule_type,
    rule_value: row.rule_value,
    priority: row.priority,
    enabled: row.enabled
  })
  dialogVisible.value = true
}

// 全量发布
const handlePromote = async (row) => {
  try {
    await ElMessageBox.confirm('确定要将该灰度规则全量发布吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await configApi.promoteGrayRule(row.config_id, row.id)
    ElMessage.success('全量发布成功')
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '全量发布失败')
    }
  }
}

// 删除
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm('确定要删除该灰度规则吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await configApi.deleteGrayRule(row.config_id, row.id)
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
    await configApi.updateGrayRule(row.config_id, row.id, {
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
          await configApi.updateGrayRule(form.config_id, form.id, form)
          ElMessage.success('更新成功')
        } else {
          await configApi.createGrayRule(form.config_id, form)
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
.gray-rules-page {
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

.rule-value-text {
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
