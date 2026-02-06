<template>
  <el-dialog
    v-model="dialogVisible"
    title="发布配置"
    width="700px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="form"
      label-width="120px"
    >
      <el-form-item label="配置信息">
        <div class="config-info">
          <div class="info-item">
            <span class="label">DataID:</span>
            <span class="value">{{ configInfo.data_id || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="label">命名空间:</span>
            <span class="value">{{ configInfo.namespace || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="label">分组:</span>
            <span class="value">{{ configInfo.group || '-' }}</span>
          </div>
          <div class="info-item">
            <span class="label">当前版本:</span>
            <span class="value">v{{ configInfo.version || '-' }}</span>
          </div>
        </div>
      </el-form-item>

      <el-form-item label="发布方式">
        <el-radio-group v-model="publishMode">
          <el-radio value="direct">直接发布</el-radio>
          <el-radio value="gray">灰度发布</el-radio>
        </el-radio-group>
      </el-form-item>

      <!-- 灰度配置 -->
      <template v-if="publishMode === 'gray'">
        <el-divider content-position="left">灰度规则配置</el-divider>
        <el-form-item label="规则名称">
          <el-input v-model="form.gray_rule.name" placeholder="请输入规则名称" />
        </el-form-item>
        <el-form-item label="灰度类型">
          <el-select v-model="form.gray_rule.rule_type" placeholder="请选择灰度类型">
            <el-option label="按百分比" value="percent" />
            <el-option label="按 IP 列表" value="ip_list" />
            <el-option label="按标签" value="tags" />
          </el-select>
        </el-form-item>

        <template v-if="form.gray_rule.rule_type === 'percent'">
          <el-form-item label="灰度比例">
            <el-slider
              v-model="form.gray_rule.percent"
              :marks="percentMarks"
              :step="5"
              show-input
            />
          </el-form-item>
        </template>

        <template v-if="form.gray_rule.rule_type === 'ip_list'">
          <el-form-item label="IP 列表">
            <el-input
              v-model="grayIpList"
              type="textarea"
              :rows="4"
              placeholder="每行一个 IP，支持 IP 段，如：&#10;192.168.1.100&#10;192.168.1.0/24"
            />
          </el-form-item>
        </template>

        <template v-if="form.gray_rule.rule_type === 'tags'">
          <el-form-item label="标签匹配">
            <el-select
              v-model="form.gray_rule.tags"
              multiple
              filterable
              allow-create
              placeholder="请选择标签"
            >
              <el-option
                v-for="tag in allTags"
                :key="tag"
                :label="tag"
                :value="tag"
              />
            </el-select>
          </el-form-item>
        </template>
      </template>

      <el-form-item label="发布备注">
        <el-input
          v-model="form.comment"
          type="textarea"
          :rows="3"
          placeholder="请输入发布备注"
        />
      </el-form-item>
    </el-form>

    <!-- 预览发布内容 -->
    <el-collapse v-model="previewCollapse" class="preview-collapse">
      <el-collapse-item title="发布内容预览" name="preview">
        <div class="content-preview">
          <pre>{{ configPreview }}</pre>
        </div>
      </el-collapse-item>
    </el-collapse>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ publishMode === 'gray' ? '创建灰度发布' : '立即发布' }}
      </el-button>
    </template>
  </el-dialog>

  <!-- 发布进度对话框 -->
  <el-dialog
    v-model="progressVisible"
    title="发布进度"
    width="600px"
    :close-on-click-modal="false"
    :show-close="false"
  >
    <div class="publish-progress">
      <el-steps :active="currentStep" align-center>
        <el-step title="准备中" description="验证配置" />
        <el-step title="发布中" description="推送到客户端" />
        <el-step title="完成" description="发布成功" />
      </el-steps>

      <div class="progress-stats">
        <div class="stat-item">
          <span class="label">目标实例:</span>
          <span class="value">{{ stats.total || 0 }}</span>
        </div>
        <div class="stat-item">
          <span class="label">已推送:</span>
          <span class="value success">{{ stats.pushed || 0 }}</span>
        </div>
        <div class="stat-item">
          <span class="label">成功:</span>
          <span class="value success">{{ stats.success || 0 }}</span>
        </div>
        <div class="stat-item">
          <span class="label">失败:</span>
          <span class="value danger">{{ stats.failed || 0 }}</span>
        </div>
      </div>

      <el-progress
        :percentage="progressPercent"
        :status="progressStatus"
      />

      <div v-if="currentLogs.length > 0" class="progress-logs">
        <div
          v-for="(log, idx) in currentLogs"
          :key="idx"
          class="log-item"
          :class="log.level"
        >
          <span class="log-time">{{ log.time }}</span>
          <span class="log-message">{{ log.message }}</span>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button
        v-if="!isCompleted"
        type="danger"
        @click="handleCancel"
      >
        取消发布
      </el-button>
      <el-button v-else type="primary" @click="handleCloseProgress">
        关闭
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { configApi } from '@/api/config'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  configId: {
    type: String,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'success'])

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const formRef = ref(null)
const submitting = ref(false)
const progressVisible = ref(false)
const isCompleted = ref(false)

const publishMode = ref('direct')
const previewCollapse = ref([])
const allTags = ref([])

const configInfo = ref({})
const grayIpList = ref('')

const percentMarks = {
  0: '0%',
  25: '25%',
  50: '50%',
  75: '75%',
  100: '100%'
}

// 发布表单
const form = reactive({
  comment: '',
  gray_rule: {
    name: '',
    rule_type: 'percent',
    percent: 10,
    ips: [],
    tags: []
  }
})

// 发布进度状态
const currentStep = ref(0)
const stats = reactive({
  total: 0,
  pushed: 0,
  success: 0,
  failed: 0
})
const currentLogs = ref([])
let eventSource = null

const progressPercent = computed(() => {
  if (stats.total === 0) return 0
  return Math.round((stats.success + stats.failed) / stats.total * 100)
})

const progressStatus = computed(() => {
  if (isCompleted.value) {
    return stats.failed > 0 ? 'exception' : 'success'
  }
  return undefined
})

const configPreview = computed(() => {
  return configInfo.value.content || '暂无内容'
})

// 加载配置信息
const loadConfig = async () => {
  if (!props.configId) return
  try {
    const res = await configApi.getConfig(props.configId)
    if (res.success) {
      configInfo.value = res.data
    }
  } catch (error) {
    ElMessage.error('加载配置信息失败')
  }
}

// 加载标签
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

// 提交发布
const handleSubmit = async () => {
  if (!props.configId) {
    ElMessage.warning('配置 ID 不能为空')
    return
  }

  submitting.value = true
  try {
    const data = {
      comment: form.comment
    }

    if (publishMode.value === 'gray') {
      if (!form.gray_rule.name) {
        ElMessage.warning('请输入灰度规则名称')
        return
      }
      // 处理 IP 列表
      if (form.gray_rule.rule_type === 'ip_list' && grayIpList.value) {
        form.gray_rule.ips = grayIpList.value.split('\n').map(ip => ip.trim()).filter(ip => ip)
      }
      data.gray_rule = form.gray_rule
    }

    const res = await configApi.publishConfig(props.configId, data)
    if (res.success) {
      ElMessage.success('发布任务已创建')
      dialogVisible.value = false
      // 打开进度对话框
      openProgress(res.data.release_id)
      emit('success')
    }
  } catch (error) {
    ElMessage.error(error.message || '发布失败')
  } finally {
    submitting.value = false
  }
}

// 打开发布进度
const openProgress = (releaseId) => {
  progressVisible.value = true
  currentStep.value = 0
  stats.total = 0
  stats.pushed = 0
  stats.success = 0
  stats.failed = 0
  currentLogs.value = []
  isCompleted.value = false

  // 订阅 SSE 事件
  const url = configApi.getReleaseEventsUrl(releaseId)
  eventSource = new EventSource(url)

  eventSource.addEventListener('start', (event) => {
    const data = JSON.parse(event.data)
    currentStep.value = 1
    stats.total = data.total || 0
    addLog('info', '开始推送配置到客户端')
  })

  eventSource.addEventListener('progress', (event) => {
    const data = JSON.parse(event.data)
    stats.pushed = data.pushed || 0
    stats.success = data.success || 0
    stats.failed = data.failed || 0
    if (data.client_id) {
      const status = data.success ? '成功' : '失败'
      addLog(data.success ? 'success' : 'error', `客户端 ${data.client_id}: ${status}`)
    }
  })

  eventSource.addEventListener('complete', (event) => {
    const data = JSON.parse(event.data)
    currentStep.value = 2
    isCompleted.value = true
    addLog('info', `发布完成，成功: ${data.success}，失败: ${data.failed}`)
    eventSource.close()
  })

  eventSource.addEventListener('error', (event) => {
    const data = JSON.parse(event.data)
    addLog('error', `发布错误: ${data.message}`)
    currentStep.value = 2
    isCompleted.value = true
    eventSource.close()
  })

  eventSource.onerror = (error) => {
    console.error('SSE error:', error)
    addLog('error', '连接中断')
    eventSource.close()
  }
}

// 添加日志
const addLog = (level, message) => {
  const now = new Date()
  const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`
  currentLogs.value.push({ time, level, message })
  // 保持最新日志在顶部，限制显示数量
  if (currentLogs.value.length > 50) {
    currentLogs.value = currentLogs.value.slice(-50)
  }
}

// 取消发布
const handleCancel = async () => {
  try {
    // 需要从某个地方获取当前 release_id，这里简化处理
    ElMessage.info('取消发布功能待实现')
  } catch (error) {
    ElMessage.error('取消失败')
  }
}

// 关闭进度对话框
const handleCloseProgress = () => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  progressVisible.value = false
}

// 关闭对话框
const handleClose = () => {
  emit('update:modelValue', false)
}

// 监听对话框打开
watch(() => props.modelValue, (val) => {
  if (val) {
    publishMode.value = 'direct'
    form.comment = ''
    form.gray_rule = {
      name: '',
      rule_type: 'percent',
      percent: 10,
      ips: [],
      tags: []
    }
    grayIpList.value = ''
    loadConfig()
    loadTags()
  }
})
</script>

<style scoped>
.config-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.info-item {
  display: flex;
}

.info-item .label {
  width: 80px;
  color: var(--el-text-color-secondary);
}

.info-item .value {
  font-weight: 500;
}

.preview-collapse {
  margin-top: 16px;
}

.content-preview {
  padding: 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 4px;
}

.content-preview pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: monospace;
  font-size: 13px;
}

.publish-progress {
  padding: 20px 0;
}

.progress-stats {
  display: flex;
  justify-content: center;
  gap: 24px;
  margin: 24px 0;
}

.stat-item {
  text-align: center;
}

.stat-item .label {
  display: block;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-bottom: 4px;
}

.stat-item .value {
  display: block;
  font-size: 20px;
  font-weight: 600;
}

.stat-item .value.success {
  color: var(--el-color-success);
}

.stat-item .value.danger {
  color: var(--el-color-danger);
}

.progress-logs {
  margin-top: 20px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  max-height: 200px;
  overflow-y: auto;
}

.log-item {
  display: flex;
  gap: 8px;
  font-size: 12px;
  line-height: 1.6;
}

.log-time {
  color: var(--el-text-color-secondary);
  font-family: monospace;
}

.log-message {
  flex: 1;
}

.log-item.error .log-message {
  color: var(--el-color-danger);
}

.log-item.success .log-message {
  color: var(--el-color-success);
}
</style>
