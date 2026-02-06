<template>
  <div class="pipeline-page">
    <!-- 页面头部 -->
    <div class="page-header">
      <h2>流水线管理</h2>
      <div class="header-actions">
        <el-select
          v-model="selectedProjectId"
          placeholder="选择项目"
          @change="loadPipelines"
          style="width: 200px; margin-right: 10px"
        >
          <el-option
            v-for="project in projects"
            :key="project.id"
            :label="project.name"
            :value="project.id"
          />
        </el-select>
        <el-button type="primary" @click="createPipeline" :disabled="!selectedProjectId">
          <el-icon><Plus /></el-icon>
          新建流水线
        </el-button>
        <el-button @click="loadPipelines" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <!-- 流水线列表 -->
    <el-card shadow="never" v-loading="loading">
      <el-empty v-if="!selectedProjectId" description="请先选择项目" :image-size="80" />
      <el-empty v-else-if="pipelines.length === 0" description="暂无流水线" :image-size="80" />
      <div v-else class="pipeline-list">
        <el-row :gutter="20">
          <el-col :span="8" v-for="pipeline in pipelines" :key="pipeline.id">
            <el-card shadow="hover" class="pipeline-card" @click="viewPipeline(pipeline)">
              <template #header>
                <div class="card-header">
                  <span class="pipeline-name">{{ pipeline.name }}</span>
                  <el-dropdown @command="handleAction($event, pipeline)" @click.stop>
                    <el-icon class="more-icon"><MoreFilled /></el-icon>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">编辑</el-dropdown-item>
                        <el-dropdown-item command="execute">执行</el-dropdown-item>
                        <el-dropdown-item command="copy">复制</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>

              <div class="pipeline-info">
                <div class="pipeline-description">{{ pipeline.description || '暂无描述' }}</div>
                <div class="pipeline-meta">
                  <el-tag size="small" :type="getTypeTag(pipeline.type)">
                    {{ getTypeLabel(pipeline.type) }}
                  </el-tag>
                  <span class="task-count">{{ pipeline.stage_count || 0 }} 个阶段</span>
                </div>
                <div class="pipeline-stats">
                  <span class="stat-item">
                    <el-icon><SuccessFilled /></el-icon>
                    {{ pipeline.success_count || 0 }}
                  </span>
                  <span class="stat-item">
                    <el-icon><CircleCloseFilled /></el-icon>
                    {{ pipeline.failed_count || 0 }}
                  </span>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
    </el-card>

    <!-- 创建/编辑流水线对话框 -->
    <el-dialog
      v-model="showEditDialog"
      :title="editingPipeline ? '编辑流水线' : '新建流水线'"
      width="600px"
      @close="resetForm"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="流水线名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入流水线名称" />
        </el-form-item>
        <el-form-item label="流水线类型" prop="type">
          <el-select v-model="form.type" placeholder="请选择类型">
            <el-option label="部署流水线" value="deploy" />
            <el-option label="运维流水线" value="operations" />
            <el-option label="CI/CD 流水线" value="cicd" />
            <el-option label="自定义流水线" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入描述"
          />
        </el-form-item>
        <el-form-item label="启用" prop="enabled">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="savePipeline" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Refresh, MoreFilled, SuccessFilled, CircleCloseFilled
} from '@element-plus/icons-vue'
import api from '@/api'

const router = useRouter()

// 数据
const loading = ref(false)
const saving = ref(false)
const projects = ref([])
const pipelines = ref([])
const selectedProjectId = ref('')
const showEditDialog = ref(false)
const editingPipeline = ref(null)
const formRef = ref()

// 表单数据
const form = ref({
  name: '',
  type: 'deploy',
  description: '',
  enabled: true
})

// 表单验证规则
const rules = {
  name: [{ required: true, message: '请输入流水线名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择流水线类型', trigger: 'change' }]
}

// 加载项目列表
const loadProjects = async () => {
  try {
    loading.value = true
    const data = await api.getProjects()
    projects.value = data || []
  } catch (error) {
    ElMessage.error('加载项目列表失败')
  } finally {
    loading.value = false
  }
}

// 加载流水线列表
const loadPipelines = async () => {
  if (!selectedProjectId.value) return

  try {
    loading.value = true
    const data = await api.getPipelines(selectedProjectId.value)
    pipelines.value = data || []
  } catch (error) {
    ElMessage.error('加载流水线列表失败')
  } finally {
    loading.value = false
  }
}

// 创建流水线
const createPipeline = () => {
  editingPipeline.value = null
  form.value = {
    name: '',
    type: 'deploy',
    description: '',
    enabled: true
  }
  showEditDialog.value = true
}

// 编辑流水线
const editPipeline = (pipeline) => {
  editingPipeline.value = pipeline
  form.value = {
    name: pipeline.name,
    type: pipeline.type,
    description: pipeline.description || '',
    enabled: pipeline.enabled ?? true
  }
  showEditDialog.value = true
}

// 保存流水线
const savePipeline = async () => {
  await formRef.value.validate()

  try {
    saving.value = true
    if (editingPipeline.value) {
      await api.updatePipeline(editingPipeline.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      await api.createPipeline(selectedProjectId.value, form.value)
      ElMessage.success('创建成功')
    }
    showEditDialog.value = false
    loadPipelines()
  } catch (error) {
    ElMessage.error(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 删除流水线
const deletePipeline = async (pipeline) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除流水线"${pipeline.name}"吗？`,
      '删除确认',
      { type: 'warning' }
    )
    await api.deletePipeline(pipeline.id)
    ElMessage.success('删除成功')
    loadPipelines()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '删除失败')
    }
  }
}

// 复制流水线
const copyPipeline = async (pipeline) => {
  try {
    const data = {
      name: `${pipeline.name} (副本)`,
      type: pipeline.type,
      description: pipeline.description,
      stages: pipeline.stages
    }
    await api.createPipeline(selectedProjectId.value, data)
    ElMessage.success('复制成功')
    loadPipelines()
  } catch (error) {
    ElMessage.error(error.message || '复制失败')
  }
}

// 查看流水线
const viewPipeline = (pipeline) => {
  router.push({
    path: '/pipeline/editor',
    query: { id: pipeline.id, projectId: selectedProjectId.value }
  })
}

// 执行流水线
const executePipeline = (pipeline) => {
  router.push({
    path: '/pipeline/execute',
    query: { id: pipeline.id, projectId: selectedProjectId.value }
  })
}

// 处理操作
const handleAction = (command, pipeline) => {
  switch (command) {
    case 'edit':
      editPipeline(pipeline)
      break
    case 'delete':
      deletePipeline(pipeline)
      break
    case 'copy':
      copyPipeline(pipeline)
      break
    case 'execute':
      executePipeline(pipeline)
      break
  }
}

// 重置表单
const resetForm = () => {
  formRef.value?.resetFields()
}

// 获取类型标签
const getTypeTag = (type) => {
  const map = {
    deploy: '',
    operations: 'success',
    cicd: 'warning',
    custom: 'info'
  }
  return map[type] || ''
}

// 获取类型文本
const getTypeLabel = (type) => {
  const map = {
    deploy: '部署',
    operations: '运维',
    cicd: 'CI/CD',
    custom: '自定义'
  }
  return map[type] || type
}

// 初始化
onMounted(() => {
  loadProjects()
})
</script>

<style scoped>
.pipeline-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.pipeline-list {
  margin-top: 20px;
}

.pipeline-card {
  cursor: pointer;
  margin-bottom: 20px;
  transition: transform 0.2s;
}

.pipeline-card:hover {
  transform: translateY(-2px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pipeline-name {
  font-weight: 600;
  font-size: 16px;
}

.more-icon {
  cursor: pointer;
  opacity: 0.6;
}

.more-icon:hover {
  opacity: 1;
}

.pipeline-info {
  padding: 10px 0;
}

.pipeline-description {
  color: #666;
  margin-bottom: 10px;
  min-height: 40px;
}

.pipeline-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.task-count {
  font-size: 12px;
  color: #999;
}

.pipeline-stats {
  display: flex;
  gap: 20px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 14px;
}

.stat-item.success {
  color: #67c23a;
}

.stat-item.danger {
  color: #f56c6c;
}
</style>
