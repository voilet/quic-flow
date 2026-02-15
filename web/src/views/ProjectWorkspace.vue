<template>
  <div class="project-workspace">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-content">
            <div class="stat-icon primary">
              <el-icon :size="30"><FolderOpened /></el-icon>
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
            <div class="stat-icon success">
              <el-icon :size="30"><SuccessFilled /></el-icon>
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
            <div class="stat-icon warning">
              <el-icon :size="30"><WarningFilled /></el-icon>
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
            <div class="stat-icon info">
              <el-icon :size="30"><Timer /></el-icon>
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
    <el-card shadow="never" class="list-card">
      <template #header>
        <div class="card-header">
          <span>项目列表</span>
          <div class="header-actions">
            <!-- 搜索框 -->
            <el-input
              v-model="searchKeyword"
              placeholder="搜索项目名称"
              :prefix-icon="Search"
              clearable
              style="width: 200px"
              @keyup.enter="handleSearch"
              @clear="handleSearchClear"
            />
            <el-select
              v-model="typeFilter"
              placeholder="项目类型"
              clearable
              style="width: 120px"
              @change="handleSearch"
            >
              <el-option label="全部" value="" />
              <el-option label="部署" value="deploy" />
              <el-option label="运维" value="operations" />
              <el-option label="CI/CD" value="cicd" />
              <el-option label="自定义" value="custom" />
            </el-select>
            <el-button type="primary" @click="showCreateDialog = true">
              <el-icon><Plus /></el-icon>
              新建项目
            </el-button>
            <el-button @click="loadProjects" :loading="loading">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>
        </div>
      </template>

      <!-- 项目表格 -->
      <el-table
        v-loading="loading"
        :data="filteredProjects"
        stripe
        style="width: 100%"
      >
        <el-table-column prop="name" label="项目名称" min-width="180">
          <template #default="{ row }">
            <div class="project-name-cell">
              <el-icon class="project-icon" :color="getProjectColor(row.type)">
                <component :is="getProjectIcon(row.type)" />
              </el-icon>
              <span class="project-name">{{ row.name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="getProjectTypeTag(row.type)" size="small">
              {{ getProjectTypeLabel(row.type) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="执行次数" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.execution_count || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="配置数" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.config_count || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="告警" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.alert_count > 0" type="danger" size="small">
              {{ row.alert_count }}
            </el-tag>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.has_active_alert" type="danger" size="small">
              <el-icon><Warning /></el-icon>
              异常
            </el-tag>
            <el-tag v-else type="success" size="small">
              <el-icon><SuccessFilled /></el-icon>
              正常
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="enterProject(row)">
              <el-icon><FolderOpened /></el-icon>
              进入
            </el-button>
            <el-button link type="primary" @click="editProject(row)">
              <el-icon><Edit /></el-icon>
              编辑
            </el-button>
            <el-button link type="danger" @click="deleteProject(row)">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <el-empty v-if="!loading && filteredProjects.length === 0" description="暂无项目">
        <el-button type="primary" @click="showCreateDialog = true">创建第一个项目</el-button>
      </el-empty>
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
          <el-select v-model="form.type" placeholder="请选择项目类型" style="width: 100%">
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
  Plus, Refresh, Search, FolderOpened, SuccessFilled, WarningFilled, Timer,
  Warning, Edit, Delete, Upload, Tools
} from '@element-plus/icons-vue'
import api from '@/api'
import dayjs from 'dayjs'

const router = useRouter()

// 数据
const loading = ref(false)
const projects = ref([])
const searchKeyword = ref('')
const typeFilter = ref('')
const showCreateDialog = ref(false)
const editingProject = ref(null)
const saving = ref(false)
const formRef = ref()

// 统计数据
const activeProjectCount = computed(() => projects.value.filter(p => p.enabled !== false).length)
const alertCount = computed(() => projects.value.reduce((sum, p) => sum + (p.alert_count || 0), 0))
const executionCount = computed(() => projects.value.reduce((sum, p) => sum + (p.today_executions || 0), 0))

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
  loading.value = true
  try {
    const data = await api.getProjects()
    projects.value = data || []
  } catch (error) {
    ElMessage.error('加载项目列表失败')
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  // 搜索已通过 computed 自动处理
}

// 清除搜索
const handleSearchClear = () => {
  searchKeyword.value = ''
}

// 进入项目 - 默认进入发布管理页面
const enterProject = (project) => {
  router.push({
    path: '/project/versions',
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
      await api.createProject(form.value)
      ElMessage.success('创建成功')
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

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
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
    deploy: Upload,
    operations: Tools,
    cicd: Timer,
    custom: FolderOpened
  }
  return map[type] || FolderOpened
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

/* 统计卡片样式 */
.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  cursor: default;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon.primary {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.stat-icon.success {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.stat-icon.warning {
  background: rgba(230, 162, 60, 0.1);
  color: #e6a23c;
}

.stat-icon.info {
  background: rgba(144, 147, 153, 0.1);
  color: #909399;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: #303133;
  line-height: 1;
}

.stat-label {
  font-size: 14px;
  color: #909399;
  margin-top: 8px;
}

/* 列表卡片样式 */
.list-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 项目名称单元格 */
.project-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.project-icon {
  font-size: 18px;
}

.project-name {
  font-weight: 500;
}

.text-muted {
  color: #c0c4cc;
}
</style>
