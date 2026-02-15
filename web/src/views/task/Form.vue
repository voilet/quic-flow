<template>
  <el-dialog
    v-model="dialogVisible"
    :title="taskId ? '编辑任务' : '新建任务'"
    width="800px"
    @close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="120px"
    >
      <el-form-item label="任务名称" prop="name">
        <el-input v-model="form.name" placeholder="请输入任务名称" />
      </el-form-item>

      <el-form-item label="任务描述" prop="description">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="3"
          placeholder="请输入任务描述"
        />
      </el-form-item>

      <el-form-item label="执行器类型" prop="executor_type">
        <el-radio-group v-model="form.executor_type">
          <el-radio :label="1">Shell</el-radio>
          <el-radio :label="2">HTTP</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item
        v-if="form.executor_type === 1"
        label="Shell 命令"
        prop="executor_config"
      >
        <el-input
          v-model="shellConfig.command"
          type="textarea"
          :rows="4"
          placeholder="请输入 Shell 命令"
        />
      </el-form-item>

      <el-form-item
        v-if="form.executor_type === 2"
        label="HTTP 配置"
        prop="executor_config"
      >
        <el-form :model="httpConfig" label-width="100px" size="small">
          <el-form-item label="URL">
            <el-input v-model="httpConfig.url" placeholder="https://example.com/api" />
          </el-form-item>
          <el-form-item label="方法">
            <el-select v-model="httpConfig.method" style="width: 100%">
              <el-option label="GET" value="GET" />
              <el-option label="POST" value="POST" />
              <el-option label="PUT" value="PUT" />
              <el-option label="DELETE" value="DELETE" />
            </el-select>
          </el-form-item>
          <el-form-item label="Headers">
            <el-input
              v-model="httpConfig.headers"
              type="textarea"
              :rows="3"
              placeholder='{"Content-Type": "application/json"}'
            />
          </el-form-item>
          <el-form-item label="Body">
            <el-input
              v-model="httpConfig.body"
              type="textarea"
              :rows="3"
              placeholder='{"key": "value"}'
            />
          </el-form-item>
        </el-form>
      </el-form-item>

      <el-form-item label="Cron 表达式" prop="cron_expr">
        <el-input
          v-model="form.cron_expr"
          placeholder="例如: 0 * * * * (每小时执行)"
        >
          <template #append>
            <el-button @click="showCronHelper = true">帮助</el-button>
          </template>
        </el-input>
        <div v-if="nextRunTime" class="next-run-time">
          下次执行时间: {{ nextRunTime }}
        </div>
      </el-form-item>

      <el-form-item label="时区">
        <el-select v-model="form.timezone" placeholder="选择时区" style="width: 100%">
          <el-option label="本地时区" value="Local" />
          <el-option label="UTC" value="UTC" />
          <el-option label="Asia/Shanghai" value="Asia/Shanghai" />
          <el-option label="America/New_York" value="America/New_York" />
          <el-option label="Europe/London" value="Europe/London" />
        </el-select>
      </el-form-item>

      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item label="开始时间">
            <el-date-picker
              v-model="form.start_time"
              type="datetime"
              placeholder="调度开始时间"
              style="width: 100%"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DDTHH:mm:ss"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="结束时间">
            <el-date-picker
              v-model="form.end_time"
              type="datetime"
              placeholder="调度结束时间"
              style="width: 100%"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DDTHH:mm:ss"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item label="最大执行次数">
        <el-input-number
          v-model="form.max_executions"
          :min="0"
          :max="1000000"
          style="width: 100%"
        />
        <div class="field-hint">设置为 0 表示不限制执行次数</div>
      </el-form-item>

      <el-form-item label="超时时间(秒)" prop="timeout">
        <el-input-number
          v-model="form.timeout"
          :min="1"
          :max="3600"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item label="重试次数" prop="retry_count">
        <el-input-number
          v-model="form.retry_count"
          :min="0"
          :max="10"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item label="重试间隔(秒)" prop="retry_interval">
        <el-input-number
          v-model="form.retry_interval"
          :min="1"
          :max="300"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item label="并发数" prop="concurrency">
        <el-input-number
          v-model="form.concurrency"
          :min="1"
          :max="100"
          style="width: 100%"
        />
      </el-form-item>

      <el-form-item label="任务分组">
        <div style="display: flex; gap: 10px; width: 100%">
          <el-select
            v-model="form.group_ids"
            multiple
            placeholder="请选择分组"
            style="flex: 1"
            filterable
          >
            <el-option
              v-for="group in groups"
              :key="group.id"
              :label="group.name"
              :value="group.id"
            >
              <div>
                <div>{{ group.name }}</div>
                <div style="font-size: 12px; color: #909399">{{ group.description || '无描述' }}</div>
              </div>
            </el-option>
          </el-select>
          <el-button @click="handleCreateGroup" type="primary" link>
            <el-icon><Plus /></el-icon>
            新建分组
          </el-button>
        </div>
        <div v-if="form.group_ids && form.group_ids.length > 0" style="margin-top: 8px; font-size: 12px; color: #909399">
          已选择 {{ form.group_ids.length }} 个分组
        </div>
      </el-form-item>

      <el-form-item label="状态" prop="status">
        <el-radio-group v-model="form.status">
          <el-radio :label="1">启用</el-radio>
          <el-radio :label="0">禁用</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        确定
      </el-button>
    </template>

    <!-- Cron 表达式帮助对话框 -->
    <el-dialog
      v-model="showCronHelper"
      title="Cron 表达式帮助"
      width="600px"
      append-to-body
    >
      <el-table :data="cronExamples" border>
        <el-table-column prop="expr" label="表达式" width="150" />
        <el-table-column prop="desc" label="说明" />
      </el-table>
    </el-dialog>

    <!-- 新建分组对话框 -->
    <el-dialog
      v-model="groupFormVisible"
      title="新建分组"
      width="500px"
      append-to-body
    >
      <el-form :model="newGroupForm" label-width="100px">
        <el-form-item label="分组名称" required>
          <el-input v-model="newGroupForm.name" placeholder="请输入分组名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="newGroupForm.description"
            type="textarea"
            :rows="3"
            placeholder="请输入分组描述"
          />
        </el-form-item>
        <el-form-item label="标签">
          <el-input
            v-model="newGroupForm.tags"
            placeholder="请输入标签，多个标签用逗号分隔"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupFormVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmitGroup">确定</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { taskApi, groupApi } from '@/api/task'
import dayjs from 'dayjs'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  taskId: {
    type: Number,
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
const showCronHelper = ref(false)
const groups = ref([])
const nextRunTime = ref('')
const groupFormVisible = ref(false)
const newGroupForm = reactive({
  name: '',
  description: '',
  tags: ''
})

const form = reactive({
  name: '',
  description: '',
  executor_type: 1,
  executor_config: '',
  cron_expr: '',
  // 增强调度字段
  timezone: 'Local',
  start_time: null,
  end_time: null,
  max_executions: 0,
  timeout: 300,
  retry_count: 0,
  retry_interval: 60,
  concurrency: 1,
  group_ids: [],
  status: 1
})

const shellConfig = reactive({
  command: ''
})

const httpConfig = reactive({
  url: '',
  method: 'GET',
  headers: '',
  body: ''
})

const rules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  executor_type: [{ required: true, message: '请选择执行器类型', trigger: 'change' }],
  cron_expr: [{ required: true, message: '请输入 Cron 表达式', trigger: 'blur' }],
  timeout: [{ required: true, message: '请输入超时时间', trigger: 'blur' }]
}

const cronExamples = [
  { expr: '0 * * * *', desc: '每小时执行一次' },
  { expr: '0 0 * * *', desc: '每天 0 点执行' },
  { expr: '0 0 * * 0', desc: '每周日 0 点执行' },
  { expr: '0 0 1 * *', desc: '每月 1 号 0 点执行' },
  { expr: '*/5 * * * *', desc: '每 5 分钟执行一次' },
  { expr: '0 9-18 * * 1-5', desc: '工作日上午 9 点到下午 6 点每小时执行' }
]

// 监听 executor_config 变化，同步到 shellConfig 或 httpConfig
watch(() => form.executor_config, (val) => {
  if (val) {
    try {
      const config = JSON.parse(val)
      if (form.executor_type === 1) {
        shellConfig.command = config.command || ''
      } else {
        Object.assign(httpConfig, config)
      }
    } catch (e) {
      // 忽略解析错误
    }
  }
}, { immediate: true })

// 监听 shellConfig 和 httpConfig 变化，同步到 form.executor_config
watch([shellConfig, httpConfig, () => form.executor_type], () => {
  if (form.executor_type === 1) {
    form.executor_config = JSON.stringify(shellConfig)
  } else {
    form.executor_config = JSON.stringify(httpConfig)
  }
}, { deep: true })

// 监听 cron_expr 变化，获取下次执行时间
watch(() => form.cron_expr, async (val) => {
  if (val && props.taskId) {
    try {
      const res = await taskApi.getNextRunTime(props.taskId)
      if (res.success && res.data.next_run_time) {
        nextRunTime.value = dayjs(res.data.next_run_time).format('YYYY-MM-DD HH:mm:ss')
      }
    } catch (e) {
      // 忽略错误
    }
  }
})

// 加载分组列表
const loadGroups = async () => {
  try {
    const res = await groupApi.listGroups()
    if (res.success) {
      groups.value = res.data || []
    }
  } catch (error) {
    console.error('加载分组列表失败:', error)
  }
}

// 加载任务详情
const loadTask = async () => {
  if (!props.taskId) {
    return
  }
  try {
    const res = await taskApi.getTask(props.taskId)
    if (res.success) {
      const task = res.data
      Object.assign(form, {
        name: task.name || '',
        description: task.description || '',
        executor_type: task.executor_type || 1,
        executor_config: task.executor_config || '',
        cron_expr: task.cron_expr || '',
        timezone: task.timezone || 'Local',
        start_time: task.start_time || null,
        end_time: task.end_time || null,
        max_executions: task.max_executions || 0,
        timeout: task.timeout || 300,
        retry_count: task.retry_count || 0,
        retry_interval: task.retry_interval || 60,
        concurrency: task.concurrency || 1,
        group_ids: task.group_ids || [],
        status: task.status !== undefined ? task.status : 1
      })

      // 解析 executor_config
      if (task.executor_config) {
        try {
          const config = JSON.parse(task.executor_config)
          if (task.executor_type === 1) {
            shellConfig.command = config.command || ''
          } else {
            Object.assign(httpConfig, config)
          }
        } catch (e) {
          console.error('解析 executor_config 失败:', e)
        }
      }
    }
  } catch (error) {
    ElMessage.error('加载任务详情失败')
  }
}

// 提交表单
const handleSubmit = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      if (props.taskId) {
        await taskApi.updateTask(props.taskId, form)
        ElMessage.success('更新成功')
      } else {
        await taskApi.createTask(form)
        ElMessage.success('创建成功')
      }
      emit('success')
      handleClose()
    } catch (error) {
      ElMessage.error(props.taskId ? '更新失败' : '创建失败')
    } finally {
      submitting.value = false
    }
  })
}

// 关闭对话框
const handleClose = () => {
  dialogVisible.value = false
  formRef.value?.resetFields()
  nextRunTime.value = ''
}

// 新建分组
const handleCreateGroup = () => {
  groupFormVisible.value = true
  newGroupForm.name = ''
  newGroupForm.description = ''
  newGroupForm.tags = ''
}

// 提交新建分组
const handleSubmitGroup = async () => {
  if (!newGroupForm.name) {
    ElMessage.warning('请输入分组名称')
    return
  }

  try {
    const res = await groupApi.createGroup(newGroupForm)
    if (res.success && res.data) {
      ElMessage.success('分组创建成功')
      groupFormVisible.value = false
      // 重新加载分组列表
      await loadGroups()
      // 自动选中新创建的分组
      if (res.data.id) {
        if (!form.group_ids) {
          form.group_ids = []
        }
        // 避免重复添加
        if (!form.group_ids.includes(res.data.id)) {
          form.group_ids.push(res.data.id)
        }
      }
      // 重置表单
      newGroupForm.name = ''
      newGroupForm.description = ''
      newGroupForm.tags = ''
    }
  } catch (error) {
    ElMessage.error('创建分组失败')
  }
}

// 监听 dialogVisible 变化
watch(dialogVisible, (val) => {
  if (val) {
    loadGroups()
    if (props.taskId) {
      loadTask()
    } else {
      // 重置表单
      Object.assign(form, {
        name: '',
        description: '',
        executor_type: 1,
        executor_config: '',
        cron_expr: '',
        timezone: 'Local',
        start_time: null,
        end_time: null,
        max_executions: 0,
        timeout: 300,
        retry_count: 0,
        retry_interval: 60,
        concurrency: 1,
        group_ids: [],
        status: 1
      })
      shellConfig.command = ''
      Object.assign(httpConfig, {
        url: '',
        method: 'GET',
        headers: '',
        body: ''
      })
    }
  }
})
</script>

<style scoped>
.next-run-time {
  margin-top: 5px;
  font-size: 12px;
  color: #909399;
}

.field-hint {
  margin-top: 5px;
  font-size: 12px;
  color: #909399;
}
</style>
