<template>
  <div class="client-tags-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>客户端标签管理</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建标签
          </el-button>
        </div>
      </template>

      <!-- 标签列表 -->
      <el-table
        v-loading="loading"
        :data="tags"
        stripe
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column label="标签" min-width="150">
          <template #default="{ row }">
            <el-tag :color="row.color" effect="dark" class="tag-badge">
              {{ row.name }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="客户端数量" width="120">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleViewClients(row)">
              {{ row.client_count || 0 }} 台
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="handleManageClients(row)">管理客户端</el-button>
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 标签表单对话框 -->
    <el-dialog
      v-model="formVisible"
      :title="currentTagId ? '编辑标签' : '新建标签'"
      width="500px"
      @close="handleClose"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="80px"
      >
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入标签名称" />
        </el-form-item>

        <el-form-item label="颜色" prop="color">
          <div class="color-picker-row">
            <el-color-picker v-model="form.color" :predefine="predefineColors" />
            <div class="color-preview">
              <el-tag :color="form.color" effect="dark" size="small">
                {{ form.name || '预览' }}
              </el-tag>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入标签描述"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          确定
        </el-button>
      </template>
    </el-dialog>

    <!-- 客户端管理对话框 -->
    <el-dialog
      v-model="clientDialogVisible"
      :title="`管理客户端 - ${currentTag?.name || ''}`"
      width="800px"
    >
      <div class="client-management">
        <!-- 添加客户端 -->
        <div class="add-client-section">
          <div style="margin-bottom: 10px">
            <el-input
              v-model="clientSearchKeyword"
              placeholder="搜索客户端ID或主机名"
              clearable
              @input="handleClientSearch"
              style="width: 100%"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>

          <div class="client-list-container">
            <el-checkbox-group v-model="selectedClientIds" style="width: 100%">
              <div
                v-for="client in filteredAvailableClients"
                :key="client.client_id"
                class="client-item"
              >
                <el-checkbox :label="client.client_id">
                  <div class="client-info">
                    <span class="client-name">{{ client.client_id }}</span>
                    <span class="client-meta">
                      {{ client.hostname || '未知主机' }}
                      <el-tag v-if="client.online" size="small" type="success" style="margin-left: 8px">在线</el-tag>
                      <el-tag v-else size="small" type="info" style="margin-left: 8px">离线</el-tag>
                    </span>
                  </div>
                </el-checkbox>
              </div>
              <div v-if="filteredAvailableClients.length === 0" class="empty-tip">
                没有可用的客户端
              </div>
            </el-checkbox-group>
          </div>

          <div class="add-actions">
            <span class="selected-count">已选择 {{ selectedClientIds.length }} 个客户端</span>
            <div>
              <el-button
                size="small"
                @click="handleSelectAll"
                :disabled="filteredAvailableClients.length === 0"
              >
                全选
              </el-button>
              <el-button
                size="small"
                @click="handleClearSelection"
                :disabled="selectedClientIds.length === 0"
              >
                清空
              </el-button>
              <el-button
                type="primary"
                @click="handleAddClients"
                :disabled="selectedClientIds.length === 0"
              >
                添加到标签
              </el-button>
            </div>
          </div>
        </div>

        <!-- 标签中的客户端列表 -->
        <el-divider>标签中的客户端</el-divider>
        <el-table
          v-loading="clientsLoading"
          :data="tagClients"
          stripe
          max-height="300"
        >
          <el-table-column prop="client_id" label="客户端ID" min-width="150" />
          <el-table-column prop="hostname" label="主机名" width="150" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag v-if="row.online" size="small" type="success">在线</el-tag>
              <el-tag v-else size="small" type="info">离线</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{ row }">
              <el-button link type="danger" @click="handleRemoveClient(row)">
                移除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search } from '@element-plus/icons-vue'
import api from '@/api'
import dayjs from 'dayjs'

const loading = ref(false)
const clientsLoading = ref(false)
const tags = ref([])
const allClients = ref([])
const tagClients = ref([])
const filteredAvailableClients = ref([])
const clientSearchKeyword = ref('')
const formVisible = ref(false)
const clientDialogVisible = ref(false)
const currentTagId = ref(null)
const currentTag = ref(null)
const selectedClientIds = ref([])
const submitting = ref(false)

const formRef = ref(null)

const form = reactive({
  name: '',
  color: '#409EFF',
  description: ''
})

const rules = {
  name: [{ required: true, message: '请输入标签名称', trigger: 'blur' }],
  color: [{ required: true, message: '请选择标签颜色', trigger: 'change' }]
}

// 预定义颜色
const predefineColors = [
  '#409EFF',
  '#67C23A',
  '#E6A23C',
  '#F56C6C',
  '#909399',
  '#00D7E7',
  '#9B59B6',
  '#3498DB',
  '#1ABC9C',
  '#E74C3C'
]

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 加载标签列表
const loadTags = async () => {
  loading.value = true
  try {
    const res = await api.getClientTags(true)
    tags.value = res || []
  } catch (error) {
    ElMessage.error('加载标签列表失败')
  } finally {
    loading.value = false
  }
}

// 加载所有客户端
const loadAllClients = async () => {
  try {
    const res = await api.getClients({ offset: 0, limit: 1000 })
    allClients.value = res.clients || []
  } catch (error) {
    console.error('加载客户端列表失败', error)
  }
}

// 加载标签下的客户端
const loadTagClients = async (tagId) => {
  clientsLoading.value = true
  try {
    const clientIds = await api.getTagClients(tagId)
    // 根据 client_id 查找完整信息
    const clients = (clientIds || []).map(clientId => {
      const client = allClients.value.find(c => c.client_id === clientId)
      return client || { client_id: clientId, hostname: '未知', online: false }
    })
    tagClients.value = clients
    return clientIds || []
  } catch (error) {
    ElMessage.error('加载标签客户端失败')
    return []
  } finally {
    clientsLoading.value = false
  }
}

// 过滤可用客户端
const filterAvailableClients = () => {
  const tagClientIds = new Set(tagClients.value.map(c => c.client_id))
  let available = allClients.value.filter(c => !tagClientIds.has(c.client_id))

  if (clientSearchKeyword.value.trim()) {
    const keyword = clientSearchKeyword.value.toLowerCase()
    available = available.filter(c =>
      c.client_id.toLowerCase().includes(keyword) ||
      (c.hostname && c.hostname.toLowerCase().includes(keyword))
    )
  }

  filteredAvailableClients.value = available
}

// 搜索客户端
const handleClientSearch = () => {
  filterAvailableClients()
}

// 新建标签
const handleCreate = () => {
  currentTagId.value = null
  form.name = ''
  form.color = '#409EFF'
  form.description = ''
  formVisible.value = true
}

// 编辑标签
const handleEdit = (row) => {
  currentTagId.value = row.id
  form.name = row.name || ''
  form.color = row.color || '#409EFF'
  form.description = row.description || ''
  formVisible.value = true
}

// 删除标签
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm('确定要删除此标签吗？标签与客户端的关联将被解除。', '警告', {
      type: 'warning'
    })
    await api.deleteClientTag(row.id)
    ElMessage.success('删除成功')
    loadTags()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 查看客户端
const handleViewClients = (row) => {
  handleManageClients(row)
}

// 管理客户端
const handleManageClients = async (row) => {
  currentTag.value = row
  clientDialogVisible.value = true
  selectedClientIds.value = []
  clientSearchKeyword.value = ''

  // 确保客户端列表已加载
  if (allClients.value.length === 0) {
    await loadAllClients()
  }

  await loadTagClients(row.id)
  filterAvailableClients()
}

// 全选
const handleSelectAll = () => {
  selectedClientIds.value = filteredAvailableClients.value.map(c => c.client_id)
}

// 清空选择
const handleClearSelection = () => {
  selectedClientIds.value = []
}

// 添加客户端到标签
const handleAddClients = async () => {
  if (selectedClientIds.value.length === 0) {
    ElMessage.warning('请选择要添加的客户端')
    return
  }

  try {
    await api.addTagClients(currentTag.value.id, selectedClientIds.value)
    ElMessage.success('客户端添加成功')
    await loadTagClients(currentTag.value.id)
    filterAvailableClients()
    selectedClientIds.value = []
    clientSearchKeyword.value = ''
    // 刷新标签列表以更新客户端数量
    loadTags()
  } catch (error) {
    ElMessage.error('添加客户端失败')
  }
}

// 从标签移除客户端
const handleRemoveClient = async (row) => {
  try {
    await ElMessageBox.confirm('确定要从标签中移除此客户端吗？', '提示', {
      type: 'warning'
    })
    await api.removeTagClient(currentTag.value.id, row.client_id)
    ElMessage.success('客户端移除成功')
    await loadTagClients(currentTag.value.id)
    filterAvailableClients()
    // 刷新标签列表以更新客户端数量
    loadTags()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('移除客户端失败')
    }
  }
}

// 提交表单
const handleSubmit = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      if (currentTagId.value) {
        await api.updateClientTag(currentTagId.value, form)
        ElMessage.success('更新成功')
      } else {
        await api.createClientTag(form)
        ElMessage.success('创建成功')
      }
      handleClose()
      loadTags()
    } catch (error) {
      ElMessage.error(currentTagId.value ? '更新失败' : '创建失败')
    } finally {
      submitting.value = false
    }
  })
}

// 关闭对话框
const handleClose = () => {
  formVisible.value = false
  formRef.value?.resetFields()
}

onMounted(() => {
  loadTags()
  loadAllClients()
})
</script>

<style scoped>
.client-tags-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tag-badge {
  color: white;
}

.color-picker-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.color-preview {
  display: flex;
  align-items: center;
}

.client-management {
  padding: 10px 0;
}

.add-client-section {
  margin-bottom: 20px;
}

.client-list-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  max-height: 300px;
  overflow-y: auto;
  padding: 8px;
}

.client-item {
  padding: 8px;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
}

.client-item:last-child {
  border-bottom: none;
}

.client-info {
  display: flex;
  flex-direction: column;
  margin-left: 8px;
}

.client-name {
  font-weight: 500;
}

.client-meta {
  font-size: 12px;
  color: #909399;
}

.empty-tip {
  padding: 20px;
  text-align: center;
  color: #909399;
}

.add-actions {
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.selected-count {
  font-size: 12px;
  color: #909399;
}
</style>
