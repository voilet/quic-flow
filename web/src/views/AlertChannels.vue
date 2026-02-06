<template>
  <div class="alert-channels-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>通知渠道配置</span>
          <el-button type="primary" @click="handleCreate">
            <el-icon><Plus /></el-icon>
            新建渠道
          </el-button>
        </div>
      </template>

      <!-- 搜索筛选 -->
      <el-form :inline="true" :model="searchForm" class="search-form">
        <el-form-item label="渠道类型">
          <el-select v-model="searchForm.type" placeholder="请选择类型" clearable>
            <el-option label="全部" value="" />
            <el-option label="钉钉" value="dingtalk" />
            <el-option label="企微" value="wework" />
            <el-option label="飞书" value="feishu" />
            <el-option label="Slack" value="slack" />
            <el-option label="邮件" value="email" />
          </el-select>
        </el-form-item>
        <el-form-item label="渠道名称">
          <el-input
            v-model="searchForm.name"
            placeholder="请输入渠道名称"
            clearable
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 渠道列表 -->
      <el-table
        v-loading="loading"
        :data="tableData"
        stripe
      >
        <el-table-column prop="name" label="渠道名称" min-width="200" />
        <el-table-column prop="type" label="类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getTypeTagType(row.type)" size="small">
              {{ getTypeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="配置信息" min-width="300">
          <template #default="{ row }">
            <div class="config-info">
              <span v-if="row.type === 'dingtalk'">
                Webhook: {{ maskUrl(row.config.webhook) }}
              </span>
              <span v-if="row.type === 'wework'">
                Webhook: {{ maskUrl(row.config.webhook) }}
              </span>
              <span v-if="row.type === 'feishu'">
                Webhook: {{ maskUrl(row.config.webhook) }}
              </span>
              <span v-if="row.type === 'slack'">
                Webhook: {{ maskUrl(row.config.webhook) }}
              </span>
              <span v-if="row.type === 'email'">
                SMTP: {{ row.config.smtp_host }}:{{ row.config.smtp_port }}
              </span>
            </div>
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
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
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

    <!-- 渠道编辑对话框 -->
    <el-dialog
      v-model="editDialogVisible"
      :title="dialogTitle"
      width="700px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="channelFormRef"
        :model="channelForm"
        :rules="channelRules"
        label-width="120px"
      >
        <el-form-item label="渠道名称" prop="name">
          <el-input v-model="channelForm.name" placeholder="请输入渠道名称" />
        </el-form-item>

        <el-form-item label="渠道类型" prop="type">
          <el-select
            v-model="channelForm.type"
            placeholder="请选择渠道类型"
            :disabled="isEdit"
            @change="handleTypeChange"
          >
            <el-option label="钉钉" value="dingtalk" />
            <el-option label="企微" value="wework" />
            <el-option label="飞书" value="feishu" />
            <el-option label="Slack" value="slack" />
            <el-option label="邮件" value="email" />
          </el-select>
        </el-form-item>

        <!-- 钉钉配置 -->
        <template v-if="channelForm.type === 'dingtalk'">
          <el-form-item label="Webhook URL" prop="config.webhook">
            <el-input
              v-model="channelForm.config.webhook"
              placeholder="请输入钉钉机器人 Webhook URL"
            />
          </el-form-item>
          <el-form-item label="签名密钥">
            <el-input
              v-model="channelForm.config.secret"
              placeholder="可选，输入加签密钥"
            />
          </el-form-item>
          <el-form-item label="消息类型">
            <el-radio-group v-model="channelForm.config.msg_type">
              <el-radio label="text">文本</el-radio>
              <el-radio label="markdown">Markdown</el-radio>
              <el-radio label="actionCard">行动卡片</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>

        <!-- 企微配置 -->
        <template v-if="channelForm.type === 'wework'">
          <el-form-item label="Webhook URL" prop="config.webhook">
            <el-input
              v-model="channelForm.config.webhook"
              placeholder="请输入企微机器人 Webhook URL"
            />
          </el-form-item>
          <el-form-item label="消息类型">
            <el-radio-group v-model="channelForm.config.msg_type">
              <el-radio label="text">文本</el-radio>
              <el-radio label="markdown">Markdown</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>

        <!-- 飞书配置 -->
        <template v-if="channelForm.type === 'feishu'">
          <el-form-item label="Webhook URL" prop="config.webhook">
            <el-input
              v-model="channelForm.config.webhook"
              placeholder="请输入飞书机器人 Webhook URL"
            />
          </el-form-item>
          <el-form-item label="签名密钥">
            <el-input
              v-model="channelForm.config.secret"
              placeholder="可选，输入签名密钥"
            />
          </el-form-item>
        </template>

        <!-- Slack 配置 -->
        <template v-if="channelForm.type === 'slack'">
          <el-form-item label="Webhook URL" prop="config.webhook">
            <el-input
              v-model="channelForm.config.webhook"
              placeholder="请输入 Slack Incoming Webhook URL"
            />
          </el-form-item>
          <el-form-item label="频道">
            <el-input
              v-model="channelForm.config.channel"
              placeholder="默认频道，如 #alerts"
            />
          </el-form-item>
          <el-form-item label="用户名">
            <el-input
              v-model="channelForm.config.username"
              placeholder="Bot 用户名"
            />
          </el-form-item>
          <el-form-item label="图标图标">
            <el-input
              v-model="channelForm.config.icon_url"
              placeholder="Bot 头像 URL"
            />
          </el-form-item>
        </template>

        <!-- 邮件配置 -->
        <template v-if="channelForm.type === 'email'">
          <el-form-item label="SMTP 服务器" prop="config.smtp_host">
            <el-input
              v-model="channelForm.config.smtp_host"
              placeholder="smtp.example.com"
            />
          </el-form-item>
          <el-form-item label="SMTP 端口" prop="config.smtp_port">
            <el-input-number
              v-model="channelForm.config.smtp_port"
              :min="1"
              :max="65535"
            />
          </el-form-item>
          <el-form-item label="用户名" prop="config.username">
            <el-input
              v-model="channelForm.config.username"
              placeholder="SMTP 用户名"
            />
          </el-form-item>
          <el-form-item label="密码" prop="config.password">
            <el-input
              v-model="channelForm.config.password"
              type="password"
              placeholder="SMTP 密码"
              show-password
            />
          </el-form-item>
          <el-form-item label="发件人">
            <el-input
              v-model="channelForm.config.from"
              placeholder="发件人邮箱地址"
            />
          </el-form-item>
          <el-form-item label="启用 TLS">
            <el-switch v-model="channelForm.config.tls" />
          </el-form-item>
          <el-form-item label="收件人">
            <el-select
              v-model="channelForm.config.to"
              multiple
              filterable
              allow-create
              placeholder="输入邮箱地址后回车添加"
              style="width: 100%"
            >
            </el-select>
          </el-form-item>
        </template>

        <el-form-item label="描述">
          <el-input
            v-model="channelForm.description"
            type="textarea"
            :rows="3"
            placeholder="请输入渠道描述"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>

    <!-- 测试渠道对话框 -->
    <el-dialog
      v-model="testDialogVisible"
      title="测试通知渠道"
      width="600px"
    >
      <el-form :model="testForm" label-width="100px">
        <el-form-item label="测试标题">
          <el-input v-model="testForm.title" placeholder="告警测试" />
        </el-form-item>
        <el-form-item label="测试内容">
          <el-input
            v-model="testForm.content"
            type="textarea"
            :rows="6"
            placeholder="这是一条测试告警通知"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="testDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="testLoading" @click="runTest">
          发送测试
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import {
  listAlertChannels,
  createAlertChannel,
  updateAlertChannel,
  deleteAlertChannel,
  testAlertChannel
} from '@/api/alert'
import dayjs from 'dayjs'

// 数据状态
const loading = ref(false)
const tableData = ref([])
const channelFormRef = ref()
const isEdit = ref(false)
const testLoading = ref(false)

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 搜索表单
const searchForm = reactive({
  type: '',
  name: ''
})

// 对话框状态
const editDialogVisible = ref(false)
const testDialogVisible = ref(false)

// 渠道表单
const channelForm = reactive({
  name: '',
  type: '',
  description: '',
  enabled: true,
  config: {}
})

// 表单验证规则
const channelRules = {
  name: [
    { required: true, message: '请输入渠道名称', trigger: 'blur' }
  ],
  type: [
    { required: true, message: '请选择渠道类型', trigger: 'change' }
  ],
  'config.webhook': [
    { required: true, message: '请输入 Webhook URL', trigger: 'blur' }
  ],
  'config.smtp_host': [
    { required: true, message: '请输入 SMTP 服务器', trigger: 'blur' }
  ],
  'config.smtp_port': [
    { required: true, message: '请输入 SMTP 端口', trigger: 'blur' }
  ]
}

// 测试表单
const testForm = reactive({
  title: '告警测试',
  content: '这是一条测试告警通知，如果您收到此消息，说明通知渠道配置正确。'
})

let currentChannel = null

// 计算属性
const dialogTitle = computed(() => isEdit.value ? '编辑渠道' : '新建渠道')

// 加载渠道列表
const loadChannels = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize,
      ...searchForm
    }
    const response = await listAlertChannels(params)
    tableData.value = response.data?.channels || []
    pagination.total = response.data?.total || 0
  } catch (error) {
    ElMessage.error('加载渠道列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  loadChannels()
}

// 重置
const handleReset = () => {
  Object.assign(searchForm, {
    type: '',
    name: ''
  })
  pagination.page = 1
  loadChannels()
}

// 分页变化
const handleSizeChange = () => {
  pagination.page = 1
  loadChannels()
}

const handlePageChange = () => {
  loadChannels()
}

// 新建渠道
const handleCreate = () => {
  isEdit.value = false
  resetChannelForm()
  editDialogVisible.value = true
}

// 编辑渠道
const handleEdit = (row) => {
  isEdit.value = true
  currentChannel = row
  Object.assign(channelForm, {
    id: row.id,
    name: row.name,
    type: row.type,
    description: row.description || '',
    enabled: row.enabled,
    config: { ...row.config }
  })
  editDialogVisible.value = true
}

// 删除渠道
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除渠道 "${row.name}" 吗？`,
      '删除渠道',
      {
        type: 'warning'
      }
    )
    await deleteAlertChannel(row.id)
    ElMessage.success('删除成功')
    loadChannels()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 切换启用状态
const handleToggle = async (row) => {
  try {
    await updateAlertChannel(row.id, { enabled: row.enabled })
    ElMessage.success(row.enabled ? '渠道已启用' : '渠道已禁用')
  } catch (error) {
    row.enabled = !row.enabled
    ElMessage.error('操作失败')
  }
}

// 测试渠道
const handleTest = (row) => {
  currentChannel = row
  testForm.title = '告警测试'
  testForm.content = '这是一条测试告警通知，如果您收到此消息，说明通知渠道配置正确。'
  testDialogVisible.value = true
}

// 运行测试
const runTest = async () => {
  testLoading.value = true
  try {
    await testAlertChannel(currentChannel.id, {
      title: testForm.title,
      content: testForm.content
    })
    ElMessage.success('测试通知已发送，请检查是否收到')
    testDialogVisible.value = false
  } catch (error) {
    ElMessage.error('发送测试通知失败：' + (error.response?.data?.msg || error.message))
  } finally {
    testLoading.value = false
  }
}

// 渠道类型变化
const handleTypeChange = (type) => {
  // 根据类型初始化默认配置
  if (type === 'dingtalk') {
    channelForm.config = {
      webhook: '',
      secret: '',
      msg_type: 'markdown'
    }
  } else if (type === 'wework') {
    channelForm.config = {
      webhook: '',
      msg_type: 'markdown'
    }
  } else if (type === 'feishu') {
    channelForm.config = {
      webhook: '',
      secret: ''
    }
  } else if (type === 'slack') {
    channelForm.config = {
      webhook: '',
      channel: '#alerts',
      username: 'Alert Bot',
      icon_url: ''
    }
  } else if (type === 'email') {
    channelForm.config = {
      smtp_host: '',
      smtp_port: 587,
      username: '',
      password: '',
      from: '',
      tls: true,
      to: []
    }
  }
}

// 重置表单
const resetChannelForm = () => {
  Object.assign(channelForm, {
    name: '',
    type: '',
    description: '',
    enabled: true,
    config: {}
  })
}

// 保存渠道
const handleSave = async () => {
  try {
    await channelFormRef.value.validate()

    const data = {
      name: channelForm.name,
      type: channelForm.type,
      description: channelForm.description,
      enabled: channelForm.enabled,
      config: channelForm.config
    }

    if (isEdit.value) {
      await updateAlertChannel(channelForm.id, data)
      ElMessage.success('更新成功')
    } else {
      await createAlertChannel(data)
      ElMessage.success('创建成功')
    }

    editDialogVisible.value = false
    loadChannels()
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

const getTypeLabel = (type) => {
  const map = {
    dingtalk: '钉钉',
    wework: '企微',
    feishu: '飞书',
    slack: 'Slack',
    email: '邮件'
  }
  return map[type] || type
}

const getTypeTagType = (type) => {
  const map = {
    dingtalk: 'danger',
    wework: 'success',
    feishu: 'primary',
    slack: 'warning',
    email: 'info'
  }
  return map[type] || ''
}

const maskUrl = (url) => {
  if (!url) return '-'
  try {
    const parsed = new URL(url)
    const hostname = parsed.hostname
    return hostname + '/***'
  } catch {
    return url.slice(0, 30) + '...'
  }
}

// 生命周期
onMounted(() => {
  loadChannels()
})
</script>

<style scoped>
.alert-channels-page {
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

.config-info {
  font-size: 13px;
  color: var(--el-text-color-regular);
  font-family: monospace;
}

.pagination {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}
</style>
