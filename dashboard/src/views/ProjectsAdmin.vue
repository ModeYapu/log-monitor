<template>
  <div class="projects-admin">
    <div class="page-header">
      <h2>项目管理</h2>
      <el-button type="primary" @click="showCreateDialog = true" :icon="Plus">
        创建项目
      </el-button>
    </div>

    <el-table :data="projects" stripe>
      <el-table-column prop="name" label="项目名称" width="200" />
      <el-table-column prop="slug" label="标识符" width="150" />
      <el-table-column prop="description" label="描述" show-overflow-tooltip />
      <el-table-column prop="api_key" label="API密钥" width="200">
        <template #default="{ row }">
          <el-input :value="maskedApiKey(row.api_key)" readonly size="small">
            <template #append>
              <el-button @click="copyApiKey(row.api_key)" :icon="CopyDocument" />
            </template>
          </el-input>
        </template>
      </el-table-column>
      <el-table-column prop="member_count" label="成员数" width="100" />
      <el-table-column prop="event_count" label="事件数" width="100" />
      <el-table-column prop="retention_days" label="保留天数" width="100" />
      <el-table-column label="操作" width="250" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="editProject(row)">编辑</el-button>
          <el-button size="small" @click="manageMembers(row)">成员</el-button>
          <el-button size="small" type="warning" @click="regenerateKey(row)">重置密钥</el-button>
          <el-button size="small" type="danger" @click="deleteProject(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create/Edit Project Dialog -->
    <el-dialog
      v-model="showCreateDialog"
      :title="editingProject ? '编辑项目' : '创建项目'"
      width="600px"
    >
      <el-form :model="projectForm" label-width="120px">
        <el-form-item label="项目名称">
          <el-input v-model="projectForm.name" placeholder="请输入项目名称" />
        </el-form-item>
        <el-form-item label="标识符">
          <el-input v-model="projectForm.slug" placeholder="请输入标识符（英文）" :disabled="editingProject" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="projectForm.description" type="textarea" placeholder="请输入项目描述" />
        </el-form-item>
        <el-form-item label="数据保留天数">
          <el-input-number v-model="projectForm.retention_days" :min="1" :max="365" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="saveProject">保存</el-button>
      </template>
    </el-dialog>

    <!-- Members Management Dialog -->
    <el-dialog v-model="showMembersDialog" title="成员管理" width="800px">
      <div class="members-header">
        <h3>项目成员</h3>
        <el-button type="primary" size="small" @click="showAddMemberDialog = true">添加成员</el-button>
      </div>
      <el-table :data="members" stripe>
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column prop="display_name" label="显示名称" width="200" />
        <el-table-column prop="role" label="角色" width="150">
          <template #default="{ row }">
            <el-tag :type="getRoleType(row.role)">{{ getRoleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" @click="updateMemberRole(row)">修改角色</el-button>
            <el-button size="small" type="danger" @click="removeMember(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Add Member Dialog -->
    <el-dialog v-model="showAddMemberDialog" title="添加成员" width="500px">
      <el-form :model="memberForm" label-width="120px">
        <el-form-item label="用户ID">
          <el-input-number v-model="memberForm.user_id" :min="1" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="memberForm.role">
            <el-option label="查看者" value="viewer" />
            <el-option label="开发者" value="developer" />
            <el-option label="所有者" value="owner" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddMemberDialog = false">取消</el-button>
        <el-button type="primary" @click="addMember">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, CopyDocument } from '@element-plus/icons-vue'
import { projectApi } from '../api'
import type { Project, ProjectMember } from '../types'

const projects = ref<Project[]>([])
const members = ref<ProjectMember[]>([])
const showCreateDialog = ref(false)
const showMembersDialog = ref(false)
const showAddMemberDialog = ref(false)
const editingProject = ref<Project | null>(null)
const currentProject = ref<Project | null>(null)

const projectForm = ref({
  name: '',
  slug: '',
  description: '',
  retention_days: 30
})

const memberForm = ref({
  user_id: 0,
  role: 'viewer'
})

const fetchProjects = async () => {
  try {
    const { data } = await projectApi.listProjects()
    projects.value = data
  } catch (error) {
    ElMessage.error('获取项目列表失败')
  }
}

const maskedApiKey = (apiKey: string) => {
  if (!apiKey) return ''
  return apiKey.substring(0, 8) + '...'
}

const copyApiKey = async (apiKey: string) => {
  try {
    await navigator.clipboard.writeText(apiKey)
    ElMessage.success('API密钥已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const editProject = (project: Project) => {
  editingProject.value = project
  projectForm.value = {
    name: project.name,
    slug: project.slug,
    description: project.description,
    retention_days: project.retention_days
  }
  showCreateDialog.value = true
}

const saveProject = async () => {
  try {
    if (editingProject.value) {
      // Update existing project
      await projectApi.updateProject(editingProject.value.id, {
        name: projectForm.value.name,
        description: projectForm.value.description,
        retention_days: projectForm.value.retention_days
      })
      ElMessage.success('项目更新成功')
    } else {
      // Create new project
      await projectApi.createProject({
        name: projectForm.value.name,
        slug: projectForm.value.slug,
        description: projectForm.value.description
      })
      ElMessage.success('项目创建成功')
    }
    showCreateDialog.value = false
    fetchProjects()
  } catch (error) {
    ElMessage.error('保存项目失败')
  }
}

const deleteProject = async (project: Project) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除项目 "${project.name}" 吗？此操作不可恢复。`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await projectApi.deleteProject(project.id)
    ElMessage.success('项目删除成功')
    fetchProjects()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除项目失败')
    }
  }
}

const regenerateKey = async (project: Project) => {
  try {
    await ElMessageBox.confirm(
      `确定要重置项目 "${project.name}" 的API密钥吗？旧的密钥将立即失效。`,
      '确认重置',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await projectApi.regenerateApiKey(project.id)
    ElMessage.success('API密钥重置成功')
    fetchProjects()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('重置密钥失败')
    }
  }
}

const manageMembers = async (project: Project) => {
  currentProject.value = project
  try {
    const { data } = await projectApi.listMembers(project.id)
    members.value = data
    showMembersDialog.value = true
  } catch (error) {
    ElMessage.error('获取成员列表失败')
  }
}

const addMember = async () => {
  try {
    if (!currentProject.value) return
    await projectApi.addMember(currentProject.value.id, {
      user_id: memberForm.value.user_id,
      role: memberForm.value.role
    })
    ElMessage.success('成员添加成功')
    showAddMemberDialog.value = false
    manageMembers(currentProject.value)
  } catch (error) {
    ElMessage.error('添加成员失败')
  }
}

const updateMemberRole = async (member: ProjectMember) => {
  try {
    if (!currentProject.value) return
    const newRole = member.role === 'owner' ? 'developer' : member.role === 'developer' ? 'viewer' : 'owner'
    await projectApi.updateMemberRole(currentProject.value.id, member.user_id, { role: newRole })
    ElMessage.success('角色更新成功')
    manageMembers(currentProject.value)
  } catch (error) {
    ElMessage.error('更新角色失败')
  }
}

const removeMember = async (member: ProjectMember) => {
  try {
    if (!currentProject.value) return
    await ElMessageBox.confirm(
      `确定要移除成员 "${member.username || member.user_id}" 吗？`,
      '确认移除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await projectApi.removeMember(currentProject.value.id, member.user_id)
    ElMessage.success('成员移除成功')
    manageMembers(currentProject.value)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('移除成员失败')
    }
  }
}

const getRoleType = (role: string) => {
  switch (role) {
    case 'owner': return 'danger'
    case 'developer': return 'warning'
    case 'viewer': return 'info'
    default: return ''
  }
}

const getRoleLabel = (role: string) => {
  switch (role) {
    case 'owner': return '所有者'
    case 'developer': return '开发者'
    case 'viewer': return '查看者'
    default: return role
  }
}

onMounted(() => {
  fetchProjects()
})
</script>

<style scoped>
.projects-admin {
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
  color: var(--color-text);
}

.members-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.members-header h3 {
  margin: 0;
  color: var(--color-text);
}
</style>