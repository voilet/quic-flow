<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isEdit ? '编辑配置' : '新建配置'"
    width="900px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <el-tabs v-model="activeTab" type="border-card">
      <!-- 基本信息 -->
      <el-tab-pane label="基本信息" name="basic">
        <el-form
          ref="formRef"
          :model="form"
          :rules="formRules"
          label-width="120px"
        >
          <el-form-item label="命名空间" prop="namespace">
            <el-select
              v-model="form.namespace"
              placeholder="请选择命名空间"
              filterable
              allow-create
              @change="handleNamespaceChange"
            >
              <el-option
                v-for="ns in namespaces"
                :key="ns.name"
                :label="ns.name"
                :value="ns.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="分组" prop="group">
            <el-select
              v-model="form.group"
              placeholder="请选择分组"
              filterable
              allow-create
              :disabled="!form.namespace"
            >
              <el-option
                v-for="group in groups"
                :key="group.name"
                :label="group.name"
                :value="group.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="DataID" prop="data_id">
            <el-input
              v-model="form.data_id"
              placeholder="请输入 DataID，如：application.yml"
            />
          </el-form-item>
          <el-form-item label="配置类型" prop="type">
            <el-radio-group v-model="form.type" @change="handleTypeChange">
              <el-radio value="yaml">YAML</el-radio>
              <el-radio value="json">JSON</el-radio>
              <el-radio value="properties">Properties</el-radio>
              <el-radio value="text">Text</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="描述">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="2"
              placeholder="请输入配置描述"
            />
          </el-form-item>
          <el-form-item label="标签">
            <el-select
              v-model="form.tags"
              multiple
              filterable
              allow-create
              placeholder="请选择或输入标签"
            >
              <el-option
                v-for="tag in allTags"
                :key="tag"
                :label="tag"
                :value="tag"
              />
            </el-select>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- 配置内容 -->
      <el-tab-pane label="配置内容" name="content">
        <div class="editor-toolbar">
          <el-button size="small" @click="handleValidate">
            <el-icon><CircleCheck /></el-icon>
            验证格式
          </el-button>
          <el-button size="small" @click="handleFormat">
            <el-icon><Sort /></el-icon>
            格式化
          </el-button>
          <el-button size="small" @click="handlePreview">
            <el-icon><View /></el-icon>
            预览
          </el-button>
        </div>
        <div class="editor-container">
          <MonacoEditor
            v-model="form.content"
            :language="editorLanguage"
            :height="editorHeight"
            :options="editorOptions"
          />
        </div>
        <div v-if="validationError" class="validation-error">
          <el-alert
            :title="validationError"
            type="error"
            :closable="false"
            show-icon
          />
        </div>
      </el-tab-pane>

      <!-- 版本对比 -->
      <el-tab-pane v-if="isEdit" label="版本对比" name="diff">
        <div class="diff-controls">
          <el-select
            v-model="diffFromVersion"
            placeholder="选择起始版本"
            style="width: 200px"
          >
            <el-option
              v-for="ver in versions"
              :key="ver.version"
              :label="`v${ver.version} - ${formatDate(ver.created_at)}`"
              :value="ver.version"
            />
          </el-select>
          <span class="diff-arrow">→</span>
          <span class="diff-current">当前版本</span>
          <el-button type="primary" @click="handleCompare">对比</el-button>
        </div>
        <div v-if="diffResult" class="diff-result">
          <div v-html="diffResult"></div>
        </div>
        <el-empty v-else description="请选择版本进行对比" />
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ isEdit ? '保存' : '创建' }}
      </el-button>
    </template>
  </el-dialog>

  <!-- 预览对话框 -->
  <el-dialog
    v-model="previewVisible"
    title="配置预览"
    width="700px"
  >
    <pre class="preview-content">{{ form.content }}</pre>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { CircleCheck, Sort, View } from '@element-plus/icons-vue'
import { configApi } from '@/api/config'
import MonacoEditor from '@/components/MonacoEditor.vue'
import dayjs from 'dayjs'

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

const isEdit = computed(() => !!props.configId)

const activeTab = ref('basic')
const formRef = ref(null)
const submitting = ref(false)
const validationError = ref('')
const previewVisible = ref(false)

const namespaces = ref([])
const groups = ref([])
const allTags = ref([])
const versions = ref([])

const diffFromVersion = ref('')
const diffResult = ref('')

// 编辑器配置
const editorHeight = '400px'
const editorOptions = {
  minimap: { enabled: false },
  fontSize: 14,
  lineNumbers: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true
}

const editorLanguage = computed(() => {
  switch (form.type) {
    case 'yaml':
      return 'yaml'
    case 'json':
      return 'json'
    case 'properties':
      return 'properties'
    default:
      return 'text'
  }
})

// 表单数据
const form = reactive({
  namespace: '',
  group: '',
  data_id: '',
  type: 'yaml',
  description: '',
  content: '',
  tags: []
})

// 表单验证规则
const formRules = {
  namespace: [
    { required: true, message: '请输入命名空间', trigger: 'blur' }
  ],
  group: [
    { required: true, message: '请输入分组', trigger: 'blur' }
  ],
  data_id: [
    { required: true, message: '请输入 DataID', trigger: 'blur' }
  ],
  type: [
    { required: true, message: '请选择配置类型', trigger: 'change' }
  ]
}

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 加载命名空间
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

// 加载分组
const loadGroups = async () => {
  if (!form.namespace) return
  try {
    const res = await configApi.listGroups(form.namespace)
    if (res.success) {
      groups.value = res.data || []
    }
  } catch (error) {
    console.error('Failed to load groups:', error)
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

// 加载配置详情
const loadConfig = async () => {
  if (!props.configId) return
  try {
    const res = await configApi.getConfig(props.configId)
    if (res.success) {
      const config = res.data
      form.namespace = config.namespace
      form.group = config.group
      form.data_id = config.data_id
      // 后端返回的是 format，前端使用的是 type
      form.type = config.format || config.type || 'yaml'
      form.description = config.description || ''
      form.content = config.content || ''
      form.tags = config.tags || []
      await loadGroups()
      await loadVersions()
    }
  } catch (error) {
    ElMessage.error('加载配置详情失败')
  }
}

// 加载版本列表
const loadVersions = async () => {
  if (!props.configId) return
  try {
    const res = await configApi.listConfigHistory(props.configId, { page: 1, page_size: 20 })
    if (res.success) {
      // 后端返回格式: {success: true, items: [...], total: ...}
      versions.value = res.items || []
    }
  } catch (error) {
    console.error('Failed to load versions:', error)
  }
}

// 命名空间变化
const handleNamespaceChange = () => {
  form.group = ''
  loadGroups()
}

// 配置类型变化
const handleTypeChange = () => {
  validationError.value = ''
}

// 验证配置格式
const handleValidate = async () => {
  if (!form.content) {
    ElMessage.warning('请输入配置内容')
    return
  }
  try {
    const res = await configApi.validateConfig({
      content: form.content,
      type: form.type
    })
    if (res.success) {
      if (res.data.valid) {
        validationError.value = ''
        ElMessage.success('配置格式验证通过')
      } else {
        validationError.value = res.data.error || '配置格式错误'
      }
    }
  } catch (error) {
    validationError.value = error.message || '验证失败'
  }
}

// 格式化配置
const handleFormat = () => {
  if (!form.content) {
    ElMessage.warning('请输入配置内容')
    return
  }
  try {
    if (form.type === 'json') {
      const parsed = JSON.parse(form.content)
      form.content = JSON.stringify(parsed, null, 2)
    } else if (form.type === 'yaml') {
      // YAML 格式化需要引入 js-yaml 库，这里简化处理
      ElMessage.info('YAML 格式化功能需要额外支持')
    }
  } catch (error) {
    ElMessage.error('格式化失败：' + error.message)
  }
}

// 预览配置
const handlePreview = () => {
  previewVisible.value = true
}

// 版本对比
const handleCompare = async () => {
  if (!diffFromVersion.value) {
    ElMessage.warning('请选择要对比的版本')
    return
  }
  try {
    const res = await configApi.compareConfig(props.configId, {
      from_version: diffFromVersion.value
    })
    if (res.success) {
      const { old_content, new_content } = res.data
      // 简单的行级别对比
      const oldLines = (old_content || '').split('\n')
      const newLines = (new_content || '').split('\n')
      const maxLines = Math.max(oldLines.length, newLines.length)

      let html = ''
      for (let i = 0; i < maxLines; i++) {
        const oldLine = oldLines[i] || ''
        const newLine = newLines[i] || ''

        if (oldLine === newLine) {
          html += `<div style="white-space: pre-wrap;">${escapeHtml(oldLine)}</div>`
        } else {
          if (oldLine) {
            html += `<div style="background-color: #f8d7da;white-space: pre-wrap;">${escapeHtml(oldLine)}</div>`
          }
          if (newLine) {
            html += `<div style="background-color: #d4edda;white-space: pre-wrap;">${escapeHtml(newLine)}</div>`
          }
        }
      }
      diffResult.value = html
    }
  } catch (error) {
    ElMessage.error('对比失败')
  }
}

// HTML 转义
const escapeHtml = (text) => {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

// 提交表单
const handleSubmit = async () => {
  await formRef.value?.validate()

  if (!form.content) {
    ElMessage.warning('请输入配置内容')
    activeTab.value = 'content'
    return
  }

  submitting.value = true
  try {
    const data = {
      namespace: form.namespace,
      group: form.group,
      data_id: form.data_id,
      type: form.type,
      description: form.description,
      content: form.content,
      tags: form.tags
    }

    if (isEdit.value) {
      await configApi.updateConfig(props.configId, data)
      ElMessage.success('更新成功')
    } else {
      await configApi.createConfig(data)
      ElMessage.success('创建成功')
    }

    emit('success')
  } catch (error) {
    ElMessage.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

// 关闭对话框
const handleClose = () => {
  emit('update:modelValue', false)
}

// 监听对话框打开
watch(() => props.modelValue, (val) => {
  if (val) {
    activeTab.value = 'basic'
    validationError.value = ''
    diffResult.value = ''
    if (!isEdit.value) {
      // 新建模式，重置表单
      form.namespace = ''
      form.group = ''
      form.data_id = ''
      form.type = 'yaml'
      form.description = ''
      form.content = ''
      form.tags = []
    }
    loadNamespaces()
    loadTags()
    if (isEdit.value) {
      loadConfig()
    }
  }
})
</script>

<style scoped>
.editor-toolbar {
  display: flex;
  gap: 8px;
  padding: 8px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  margin-bottom: 8px;
}

.editor-container {
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
}

.validation-error {
  margin-top: 12px;
}

.diff-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  margin-bottom: 16px;
}

.diff-arrow {
  font-size: 18px;
  color: var(--el-text-color-secondary);
}

.diff-current {
  color: var(--el-color-primary);
  font-weight: 500;
}

.diff-result {
  padding: 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  max-height: 400px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.6;
}

.preview-content {
  margin: 0;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: monospace;
  font-size: 13px;
  max-height: 500px;
  overflow-y: auto;
}
</style>
