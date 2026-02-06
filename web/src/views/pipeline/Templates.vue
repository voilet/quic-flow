<template>
  <div class="templates-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <h3>流水线模板</h3>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            创建模板
          </el-button>
        </div>
      </template>

      <!-- 搜索和筛选 -->
      <div class="search-bar">
        <el-input
          v-model="searchForm.keyword"
          placeholder="搜索模板名称或描述"
          style="width: 300px; margin-right: 10px"
          clearable
          @clear="loadTemplates"
          @keyup.enter="loadTemplates"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="searchForm.category"
          placeholder="分类"
          style="width: 150px; margin-right: 10px"
          clearable
          @change="loadTemplates"
        >
          <el-option label="全部" value="" />
          <el-option label="运维场景" value="ops" />
          <el-option label="CI/CD场景" value="cicd" />
        </el-select>
        <el-button @click="loadTemplates" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <!-- 模板列表 -->
      <el-table v-loading="loading" :data="templates" stripe style="margin-top: 20px">
        <el-table-column prop="name" label="模板名称" min-width="200" />
        <el-table-column prop="category" label="分类" width="120">
          <template #default="{ row }">
            <el-tag :type="row.category === 'ops' ? 'success' : 'warning'">
              {{ row.category === 'ops' ? '运维场景' : 'CI/CD场景' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="300" show-overflow-tooltip />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="useTemplate(row)">使用模板</el-button>
            <el-button link type="primary" @click="editTemplate(row)">编辑</el-button>
            <el-button link type="danger" @click="deleteTemplate(row)">删除</el-button>
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

    <!-- 创建/编辑模板对话框 -->
    <el-dialog
      v-model="showEditDialog"
      :title="editingTemplate ? '编辑模板' : '创建模板'"
      width="800px"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item label="模板名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入模板名称" />
        </el-form-item>
        <el-form-item label="分类" prop="category">
          <el-select v-model="form.category" placeholder="请选择分类">
            <el-option label="运维场景" value="ops" />
            <el-option label="CI/CD场景" value="cicd" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入模板描述"
          />
        </el-form-item>
        <el-form-item label="模板定义" prop="definition">
          <el-input
            v-model="form.definition"
            type="textarea"
            :rows="10"
            placeholder="请输入YAML格式的流水线定义"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="saveTemplate" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <!-- 使用模板对话框 -->
    <el-dialog
      v-model="showUseDialog"
      title="从模板创建流水线"
      width="600px"
    >
      <el-form :model="useForm" :rules="useRules" ref="useFormRef" label-width="100px">
        <el-form-item label="流水线名称" prop="name">
          <el-input v-model="useForm.name" placeholder="请输入流水线名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="useForm.description"
            type="textarea"
            :rows="3"
            placeholder="请输入描述"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUseDialog = false">取消</el-button>
        <el-button type="primary" @click="createFromTemplate" :loading="creating">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import { request } from '@/api/index.js'
import dayjs from 'dayjs'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const saving = ref(false)
const creating = ref(false)
const templates = ref([])
const showEditDialog = ref(false)
const showUseDialog = ref(false)
const editingTemplate = ref(null)
const selectedTemplate = ref(null)
const formRef = ref()
const useFormRef = ref()

const searchForm = reactive({
  keyword: '',
  category: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const form = reactive({
  name: '',
  category: '',
  description: '',
  definition: ''
})

const useForm = reactive({
  name: '',
  description: ''
})

const rules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  category: [{ required: true, message: '请选择分类', trigger: 'change' }],
  definition: [{ required: true, message: '请输入模板定义', trigger: 'blur' }]
}

const useRules = {
  name: [{ required: true, message: '请输入流水线名称', trigger: 'blur' }]
}

// 格式化日期
const formatDate = (date) => {
  return date ? dayjs(date).format('YYYY-MM-DD HH:mm:ss') : '-'
}

// 加载模板列表
const loadTemplates = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize
    }
    if (searchForm.keyword) params.keyword = searchForm.keyword
    if (searchForm.category) params.category = searchForm.category

    const res = await request.get('/pipeline/templates', { params })
    if (res.success) {
      templates.value = res.items || res.data || []
      pagination.total = res.total || 0
    } else {
      ElMessage.error(res.error || '加载模板列表失败')
    }
  } catch (error) {
    ElMessage.error('加载模板列表失败')
  } finally {
    loading.value = false
  }
}

// 显示创建对话框
const showCreateDialog = () => {
  editingTemplate.value = null
  form.name = ''
  form.category = ''
  form.description = ''
  form.definition = ''
  showEditDialog.value = true
}

// 编辑模板
const editTemplate = (template) => {
  editingTemplate.value = template
  form.name = template.name
  form.category = template.category
  form.description = template.description || ''
  form.definition = template.definition || ''
  showEditDialog.value = true
}

// 保存模板
const saveTemplate = async () => {
  await formRef.value.validate()

  try {
    saving.value = true
    if (editingTemplate.value) {
      await request.put(`/pipeline/templates/${editingTemplate.value.id}`, form)
      ElMessage.success('更新成功')
    } else {
      await request.post('/pipeline/templates', form)
      ElMessage.success('创建成功')
    }
    showEditDialog.value = false
    loadTemplates()
  } catch (error) {
    ElMessage.error(error.response?.data?.error || error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 删除模板
const deleteTemplate = async (template) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除模板 "${template.name}" 吗？`,
      '删除确认',
      { type: 'warning' }
    )
    await request.delete(`/pipeline/templates/${template.id}`)
    ElMessage.success('删除成功')
    loadTemplates()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || error.message || '删除失败')
    }
  }
}

// 使用模板
const useTemplate = (template) => {
  selectedTemplate.value = template
  useForm.name = template.name
  useForm.description = template.description || ''
  showUseDialog.value = true
}

// 从模板创建流水线
const createFromTemplate = async () => {
  await useFormRef.value.validate()

  try {
    creating.value = true
    const res = await request.post(`/pipeline/templates/${selectedTemplate.value.id}/instantiate`, {
      name: useForm.name,
      description: useForm.description,
      parameters: {}
    })
    
    if (res.success) {
      ElMessage.success('流水线创建成功')
      showUseDialog.value = false
      // 跳转到流水线编辑器
      router.push({
        path: '/project/pipeline/editor',
        query: { id: res.data?.id || res.data?.pipeline_id }
      })
    } else {
      ElMessage.error(res.error || '创建失败')
    }
  } catch (error) {
    ElMessage.error(error.response?.data?.error || error.message || '创建失败')
  } finally {
    creating.value = false
  }
}

// 分页处理
const handleSizeChange = () => {
  loadTemplates()
}

const handlePageChange = () => {
  loadTemplates()
}

// 初始化
onMounted(() => {
  loadTemplates()
})
</script>

<style scoped>
.templates-page {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 18px;
}

.search-bar {
  display: flex;
  align-items: center;
  margin-bottom: 20px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
