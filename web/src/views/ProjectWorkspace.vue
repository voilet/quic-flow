<template>
  <div class="project-workspace">
    <!-- 页面头部 -->
    <div class="page-header">
      <div>
        <h2>项目工作台</h2>
        <p class="subtitle">选择一个项目进入工作台，或创建新项目</p>
      </div>
      <el-button type="primary" size="large" @click="showCreateDialog = true">
        <el-icon><Plus /></el-icon>
        创建新项目
      </el-button>
    </div>

    <!-- 项目统计 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background: #ecf5ff; color: #409eff">
              <el-icon><FolderOpened /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ projects.length }}</div>
              <div class="stat-label">全部项目</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background: #f0f9ff; color: #67c23a">
              <el-icon><SuccessFilled /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ activeProjectCount }}</div>
              <div class="stat-label">活跃项目</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background: #fef0f0; color: #f56c6c">
              <el-icon><WarningFilled /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ alertCount }}</div>
              <div class="stat-label">活跃告警</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" style="background: #fff7e6; color: #e6a23c">
              <el-icon><Timer /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ executionCount }}</div>
              <div class="stat-label">今日执行</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 项目列表 -->
    <el-card shadow="never" class="projects-card">
      <template #header>
        <div class="card-header">
          <span>项目列表</span>
          <div class="header-actions">
            <el-input
              v-model="searchKeyword"
              placeholder="搜索项目..."
              style="width: 200px"
              clearable
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="typeFilter" placeholder="项目类型" clearable style="width: 120px">
              <el-option label="全部" value="" />
              <el-option label="部署" value="deploy" />
              <el-option label="运维" value="operations" />
              <el-option label="CI/CD" value="cicd" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </div>
        </div>
      </template>

      <el-empty v-if="filteredProjects.length === 0" description="暂无项目">
        <el-button type="primary" @click="showCreateDialog = true">创建第一个项目</el-button>
      </el-empty>

      <el-row :gutter="20" v-else>
        <el-col :span="8" v-for="project in filteredProjects" :key="project.id">
          <el-card shadow="hover" class="project-card" @click="enterProject(project)">
            <template #header>
              <div class="project-card-header">
                <div class="project-info">
                  <el-icon class="project-icon" :color="getProjectColor(project.type)">
                    <component :is="getProjectIcon(project.type)" />
                  </el-icon>
                  <span class="project-name">{{ project.name }}</span>
                </div>
                <el-tag :type="getProjectTypeTag(project.type)" size="small">
                  {{ getProjectTypeLabel(project.type) }}
                </el-tag>
              </div>
            </template>

            <div class="project-description">
              {{ project.description || '暂无描述' }}
            </div>

            <div class="project-meta">
              <div class="meta-item">
                <el-icon><Document /></el-icon>
                <span>{{ project.pipeline_count || 0 }} 条流水线</span>
              </div>
              <div class="meta-item">
                <el-icon><Files /></el-icon>
                <span>{{ project.config_count || 0 }} 项配置</span>
              </div>
              <div class="meta-item">
                <el-icon><Bell /></el-icon>
                <span>{{ project.alert_count || 0 }} 条告警</span>
              </div>
            </div>

            <div class="project-actions" @click.stop>
              <el-button text @click="editProject(project)">
                <el-icon><Edit /></el-icon>
                编辑
              </el-button>
              <el-divider direction="vertical" />
              <el-button text type="danger" @click="deleteProject(project)">
                <el-icon><Delete /></el-icon>
                删除
              </el-button>
            </div>

            <div class="project-status">
              <el-tag v-if="project.has_active_alert" type="danger" size="small">
                <el-icon><Warning /></el-icon>
                有告警
              </el-tag>
              <el-tag v-else type="success" size="small">
                <el-icon><SuccessFilled /></el-icon>
                正常
              </el-tag>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </el-card>

    <!-- 创建/编辑项目对话框 -->
    <el-dialog
      v-model="showCreateDialog"
      :title="editingProject ? '编辑项目' : '创建新项目'"
      width="500px"
      @close="resetForm"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="项目名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入项目名称" />
        </el-form-item>
        <el-form-item label="项目类型" prop="type">
          <el-select v-model="form.type" placeholder="请选择项目类型">
            <el-option label="部署项目" value="deploy" />
            <el-option label="运维项目" value="operations" />
            <el-option label="CI/CD 项目" value="cicd" />
            <el-option label="自定义项目" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item label="项目描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入项目描述"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="saveProject" :loading="saving">
          {{ editingProject ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, FolderOpened, SuccessFilled, WarningFilled, Timer, Search,
  Document, Files, Bell, Warning, Edit, Delete
} from '@element-plus/icons-vue'
import api from '@/api'

const router = useRouter()

// 数据
const projects = ref([])
const searchKeyword = ref('')
const typeFilter = ref('')
const showCreateDialog = ref(false)
const editingProject = ref(null)
const saving = ref(false)
const formRef = ref()

// 统计数据
const activeProjectCount = ref(0)
const alertCount = ref(0)
const executionCount = ref(0)

// 表单数据
const form = ref({
  name: '',
  type: 'custom',
  description: ''
})

// 验证规则
const rules = {
  name: [{ required: true, message: '请输入项目名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择项目类型', trigger: 'change' }]
}

// 过滤后的项目列表
const filteredProjects = computed(() => {
  return projects.value.filter(project => {
    const matchKeyword = !searchKeyword.value ||
      project.name.toLowerCase().includes(searchKeyword.value.toLowerCase())
    const matchType = !typeFilter.value || project.type === typeFilter.value
    return matchKeyword && matchType
  })
})

// 加载项目列表
const loadProjects = async () => {
  try {
    const data = await api.getProjects()
    projects.value = data || []
    // 计算统计数据
    activeProjectCount.value = projects.value.filter(p => p.enabled !== false).length
    alertCount.value = projects.value.reduce((sum, p) => sum + (p.alert_count || 0), 0)
    executionCount.value = projects.value.reduce((sum, p) => sum + (p.today_executions || 0), 0)
  } catch (error) {
    ElMessage.error('加载项目列表失败')
  }
}

// 进入项目
const enterProject = (project) => {
  router.push({
    path: '/project/overview',
    query: { projectId: project.id }
  })
}

// 编辑项目
const editProject = (project) => {
  editingProject.value = project
  form.value = {
    name: project.name,
    type: project.type,
    description: project.description || ''
  }
  showCreateDialog.value = true
}

// 删除项目
const deleteProject = async (project) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除项目"${project.name}"吗？此操作不可恢复。`,
      '删除确认',
      { type: 'warning' }
    )
    await api.deleteProject(project.id)
    ElMessage.success('删除成功')
    loadProjects()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || '删除失败')
    }
  }
}

// 保存项目
const saveProject = async () => {
  await formRef.value.validate()

  try {
    saving.value = true
    if (editingProject.value) {
      await api.updateProject(editingProject.value.id, form.value)
      ElMessage.success('更新成功')
    } else {
      const data = await api.createProject(form.value)
      ElMessage.success('创建成功')
      // 自动进入新项目
      enterProject(data)
    }
    showCreateDialog.value = false
    loadProjects()
  } catch (error) {
    ElMessage.error(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 重置表单
const resetForm = () => {
  formRef.value?.resetFields()
  editingProject.value = null
}

// 获取项目类型标签
const getProjectTypeTag = (type) => {
  const map = {
    deploy: '',
    operations: 'success',
    cicd: 'warning',
    custom: 'info'
  }
  return map[type] || ''
}

const getProjectTypeLabel = (type) => {
  const map = {
    deploy: '部署',
    operations: '运维',
    cicd: 'CI/CD',
    custom: '自定义'
  }
  return map[type] || type
}

const getProjectIcon = (type) => {
  const map = {
    deploy: 'Upload',
    operations: 'Tools',
    cicd: 'Timer',
    custom: 'Folder'
  }
  return map[type] || 'Folder'
}

const getProjectColor = (type) => {
  const map = {
    deploy: '#409eff',
    operations: '#67c23a',
    cicd: '#e6a23c',
    custom: '#909399'
  }
  return map[type] || '#909399'
}

// 初始化
onMounted(() => {
  loadProjects()
})
</script>

<style scoped>
.project-workspace {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  color: #303133;
}

.subtitle {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  cursor: pointer;
  transition: transform 0.2s;
}

.stat-card:hover {
  transform: translateY(-2px);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  line-height: 1;
}

.stat-label {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.projects-card {
  min-height: 400px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.project-card {
  cursor: pointer;
  margin-bottom: 20px;
  transition: all 0.3s;
  position: relative;
}

.project-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
}

.project-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.project-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.project-icon {
  font-size: 20px;
}

.project-name {
  font-weight: 600;
  font-size: 16px;
  color: #303133;
}

.project-description {
  color: #606266;
  font-size: 14px;
  margin: 12px 0;
  min-height: 40px;
  line-height: 1.6;
}

.project-meta {
  display: flex;
  gap: 20px;
  margin: 12px 0;
  padding: 12px 0;
  border-top: 1px solid #ebeef5;
  border-bottom: 1px solid #ebeef5;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #909399;
}

.project-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 12px;
}

.project-status {
  position: absolute;
  top: 12px;
  right: 12px;
}
</style>
