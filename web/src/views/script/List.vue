<template>
  <div class="script-list-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>脚本管理</span>
          <div class="header-actions">
            <el-select v-model="filterCategory" placeholder="分类筛选" clearable style="width: 150px; margin-right: 10px">
              <el-option label="部署脚本" value="deploy" />
              <el-option label="监控脚本" value="monitor" />
              <el-option label="运维脚本" value="operation" />
              <el-option label="其他" value="other" />
            </el-select>
            <el-button type="primary" @click="handleCreate">
              <el-icon><Plus /></el-icon>
              新建脚本
            </el-button>
          </div>
        </div>
      </template>

      <!-- 脚本列表 -->
      <el-table
        v-loading="loading"
        :data="scripts"
        stripe
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="脚本名称" min-width="150" />
        <el-table-column label="分类" width="100">
          <template #default="{ row }">
            <el-tag :type="getCategoryType(row.category)" size="small">
              {{ getCategoryLabel(row.category) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="解释器" width="100">
          <template #default="{ row }">
            <el-tag size="small" effect="plain">{{ row.interpreter }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="版本" width="80">
          <template #default="{ row }">
            {{ row.version_count || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="执行次数" width="100">
          <template #default="{ row }">
            {{ row.execution_count || 0 }}
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="handleVersions(row)">版本</el-button>
            <el-button link type="success" @click="handleExecute(row)">执行</el-button>
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 脚本编辑对话框 -->
    <el-dialog
      v-model="formVisible"
      :title="currentScriptId ? '编辑脚本' : '新建脚本'"
      width="900px"
      @close="handleClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="80px"
      >
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="form.name" placeholder="请输入脚本名称" />
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="分类" prop="category">
              <el-select v-model="form.category" style="width: 100%">
                <el-option label="部署脚本" value="deploy" />
                <el-option label="监控脚本" value="monitor" />
                <el-option label="运维脚本" value="operation" />
                <el-option label="其他" value="other" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="6">
            <el-form-item label="解释器" prop="interpreter">
              <el-select v-model="form.interpreter" style="width: 100%">
                <el-option label="Bash" value="bash" />
                <el-option label="Python" value="python" />
                <el-option label="PowerShell" value="powershell" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="描述">
          <el-input v-model="form.description" placeholder="请输入脚本描述" />
        </el-form-item>

        <el-form-item label="脚本内容" prop="content">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="15"
            placeholder="请输入脚本内容"
            style="font-family: monospace"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          保存
        </el-button>
        <el-button type="success" :loading="submitting" @click="handleSubmitAndPublish">
          保存并发布
        </el-button>
      </template>
    </el-dialog>

    <!-- 版本历史对话框 -->
    <el-dialog
      v-model="versionDialogVisible"
      :title="`版本历史 - ${currentScript?.name || ''}`"
      width="800px"
    >
      <el-table v-loading="versionsLoading" :data="versions" stripe>
        <el-table-column prop="version" label="版本号" width="120" />
        <el-table-column prop="change_log" label="变更说明" min-width="200" />
        <el-table-column prop="created_by" label="创建人" width="100" />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleViewVersion(row)">查看</el-button>
            <el-button link type="primary" @click="handleRollback(row)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 执行对话框 -->
    <el-dialog
      v-model="executeDialogVisible"
      :title="`执行脚本 - ${currentScript?.name || ''}`"
      width="700px"
    >
      <el-form :model="executeForm" label-width="100px">
        <el-form-item label="目标客户端">
          <el-select
            v-model="executeForm.client_ids"
            multiple
            filterable
            placeholder="选择目标客户端"
            style="width: 100%"
          >
            <el-option
              v-for="client in clients"
              :key="client.client_id"
              :label="client.client_id"
              :value="client.client_id"
            >
              <div style="display: flex; justify-content: space-between">
                <span>{{ client.client_id }}</span>
                <el-tag v-if="client.online" size="small" type="success">在线</el-tag>
                <el-tag v-else size="small" type="info">离线</el-tag>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="超时时间">
          <el-input-number v-model="executeForm.timeout" :min="10" :max="3600" :step="30" />
          <span style="margin-left: 10px; color: #909399">秒</span>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="executeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="doExecute">
          开始执行
        </el-button>
      </template>
    </el-dialog>

    <!-- 版本内容查看对话框 -->
    <el-dialog
      v-model="versionContentVisible"
      title="版本内容"
      width="800px"
    >
      <pre class="version-content">{{ versionContent }}</pre>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import api from '@/api'
import dayjs from 'dayjs'

const loading = ref(false)
const scripts = ref([])
const clients = ref([])
const filterCategory = ref('')

// 表单相关
const formVisible = ref(false)
const currentScriptId = ref(null)
const submitting = ref(false)
const formRef = ref(null)

const form = reactive({
  name: '',
  description: '',
  category: 'other',
  interpreter: 'bash',
  content: ''
})

const rules = {
  name: [{ required: true, message: '请输入脚本名称', trigger: 'blur' }],
  category: [{ required: true, message: '请选择分类', trigger: 'change' }],
  interpreter: [{ required: true, message: '请选择解释器', trigger: 'change' }],
  content: [{ required: true, message: '请输入脚本内容', trigger: 'blur' }]
}

// 版本相关
const versionDialogVisible = ref(false)
const versionsLoading = ref(false)
const versions = ref([])
const currentScript = ref(null)
const versionContentVisible = ref(false)
const versionContent = ref('')

// 执行相关
const executeDialogVisible = ref(false)
const executing = ref(false)
const executeForm = reactive({
  client_ids: [],
  timeout: 300
})

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 获取分类标签类型
const getCategoryType = (category) => {
  const types = {
    deploy: 'primary',
    monitor: 'success',
    operation: 'warning',
    other: 'info'
  }
  return types[category] || 'info'
}

// 获取分类标签文本
const getCategoryLabel = (category) => {
  const labels = {
    deploy: '部署',
    monitor: '监控',
    operation: '运维',
    other: '其他'
  }
  return labels[category] || category
}

// 加载脚本列表
const loadScripts = async () => {
  loading.value = true
  try {
    const res = await api.getScripts(filterCategory.value, '', true)
    scripts.value = res || []
  } catch (error) {
    ElMessage.error('加载脚本列表失败')
  } finally {
    loading.value = false
  }
}

// 加载客户端列表
const loadClients = async () => {
  try {
    const res = await api.getClients({ offset: 0, limit: 1000 })
    clients.value = res.clients || []
  } catch (error) {
    console.error('加载客户端列表失败', error)
  }
}

// 监听分类筛选变化
watch(filterCategory, () => {
  loadScripts()
})

// 新建脚本
const handleCreate = () => {
  currentScriptId.value = null
  form.name = ''
  form.description = ''
  form.category = 'other'
  form.interpreter = 'bash'
  form.content = ''
  formVisible.value = true
}

// 编辑脚本
const handleEdit = async (row) => {
  try {
    const script = await api.getScript(row.id)
    currentScriptId.value = script.id
    form.name = script.name || ''
    form.description = script.description || ''
    form.category = script.category || 'other'
    form.interpreter = script.interpreter || 'bash'
    form.content = script.content || ''
    formVisible.value = true
  } catch (error) {
    ElMessage.error('获取脚本详情失败')
  }
}

// 删除脚本
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm('确定要删除此脚本吗？', '警告', { type: 'warning' })
    await api.deleteScript(row.id)
    ElMessage.success('删除成功')
    loadScripts()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 查看版本历史
const handleVersions = async (row) => {
  currentScript.value = row
  versionDialogVisible.value = true
  versionsLoading.value = true
  try {
    versions.value = await api.getScriptVersions(row.id) || []
  } catch (error) {
    ElMessage.error('获取版本历史失败')
  } finally {
    versionsLoading.value = false
  }
}

// 查看版本内容
const handleViewVersion = (row) => {
  versionContent.value = row.content
  versionContentVisible.value = true
}

// 回滚到指定版本
const handleRollback = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要回滚到版本 ${row.version} 吗？`, '提示', { type: 'warning' })
    await api.updateScript(currentScript.value.id, { content: row.content })
    ElMessage.success('回滚成功')
    versionDialogVisible.value = false
    loadScripts()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('回滚失败')
    }
  }
}

// 执行脚本
const handleExecute = async (row) => {
  currentScript.value = row
  executeForm.client_ids = []
  executeForm.timeout = 300
  executeDialogVisible.value = true

  if (clients.value.length === 0) {
    await loadClients()
  }
}

// 执行脚本
const doExecute = async () => {
  if (executeForm.client_ids.length === 0) {
    ElMessage.warning('请选择目标客户端')
    return
  }

  executing.value = true
  try {
    await api.executeScript(currentScript.value.id, {
      client_ids: executeForm.client_ids,
      timeout: executeForm.timeout
    })
    ElMessage.success('脚本执行已开始')
    executeDialogVisible.value = false
    loadScripts()
  } catch (error) {
    ElMessage.error('执行失败: ' + (error.msg || error.message))
  } finally {
    executing.value = false
  }
}

// 提交表单
const handleSubmit = async (publish = false) => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      const data = { ...form }
      if (publish) {
        data.status = 'published'
      }

      if (currentScriptId.value) {
        await api.updateScript(currentScriptId.value, data)
        ElMessage.success('更新成功')
      } else {
        await api.createScript(data)
        ElMessage.success('创建成功')
      }
      handleClose()
      loadScripts()
    } catch (error) {
      ElMessage.error(currentScriptId.value ? '更新失败' : '创建失败')
    } finally {
      submitting.value = false
    }
  })
}

// 保存并发布
const handleSubmitAndPublish = async () => {
  await handleSubmit(true)
}

// 关闭对话框
const handleClose = () => {
  formVisible.value = false
  formRef.value?.resetFields()
}

onMounted(() => {
  loadScripts()
})
</script>

<style scoped>
.script-list-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
}

.version-content {
  background: #f5f7fa;
  padding: 16px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 500px;
  overflow-y: auto;
}
</style>
