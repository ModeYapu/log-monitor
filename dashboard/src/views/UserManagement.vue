<template>
  <div class="users-page">
    <h1 class="sr-only">用户管理</h1>
    <el-row :gutter="20">
      <el-col :span="24">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>用户管理</span>
              <el-button type="primary" :icon="Plus" @click="showCreateDialog">
                新增用户
              </el-button>
            </div>
          </template>

          <el-table :data="users" v-loading="loading" stripe>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="username" label="用户名" width="150" />
            <el-table-column prop="display_name" label="显示名称" width="150" />
            <el-table-column prop="role" label="角色" width="120">
              <template #default="{ row }">
                <el-tag :type="row.role === 'admin' ? 'danger' : 'primary'" size="small">
                  {{ row.role === 'admin' ? '管理员' : '普通用户' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="enabled" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="last_login_at" label="最后登录" width="180">
              <template #default="{ row }">
                {{ row.last_login_at ? formatTime(row.last_login_at) : '从未登录' }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="240">
              <template #default="{ row }">
                <el-button
                  size="small"
                  :disabled="row.id === currentUserId"
                  @click="showEditDialog(row)"
                >
                  编辑
                </el-button>
                <el-button
                  size="small"
                  :disabled="row.id === currentUserId"
                  @click="showResetPasswordDialog(row)"
                >
                  重置密码
                </el-button>
                <el-button
                  size="small"
                  type="danger"
                  :disabled="row.role === 'admin' || row.id === currentUserId"
                  @click="handleDelete(row)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- Create User Dialog -->
    <el-dialog
      v-model="createDialogVisible"
      title="新增用户"
      width="500px"
    >
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-width="100px"
      >
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="createForm.password"
            type="password"
            show-password
            placeholder="请输入密码"
          />
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input v-model="createForm.display_name" placeholder="请输入显示名称" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-radio-group v-model="createForm.role">
            <el-radio label="user">普通用户</el-radio>
            <el-radio label="admin">管理员</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreate">
          确定
        </el-button>
      </template>
    </el-dialog>

    <!-- Edit User Dialog -->
    <el-dialog
      v-model="editDialogVisible"
      title="编辑用户"
      width="500px"
    >
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="editRules"
        label-width="100px"
      >
        <el-form-item label="用户名">
          <el-input v-model="editForm.username" disabled />
        </el-form-item>
        <el-form-item label="显示名称" prop="display_name">
          <el-input v-model="editForm.display_name" placeholder="请输入显示名称" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-radio-group v-model="editForm.role">
            <el-radio label="user">普通用户</el-radio>
            <el-radio label="admin">管理员</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="editForm.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleUpdate">
          确定
        </el-button>
      </template>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog
      v-model="resetPasswordDialogVisible"
      title="重置密码"
      width="400px"
    >
      <el-form
        ref="resetPasswordFormRef"
        :model="resetPasswordForm"
        :rules="resetPasswordRules"
        label-width="100px"
      >
        <el-form-item label="用户名">
          <el-input v-model="resetPasswordForm.username" disabled />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input
            v-model="resetPasswordForm.new_password"
            type="password"
            show-password
            placeholder="请输入新密码"
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input
            v-model="resetPasswordForm.confirm_password"
            type="password"
            show-password
            placeholder="请再次输入新密码"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleResetPassword">
          确定
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { authApi } from '../api'
import type { User } from '../types'

const loading = ref(false)
const users = ref<User[]>([])
const currentUserId = ref(0)

const createDialogVisible = ref(false)
const editDialogVisible = ref(false)
const resetPasswordDialogVisible = ref(false)
const submitting = ref(false)

const createFormRef = ref<FormInstance>()
const editFormRef = ref<FormInstance>()
const resetPasswordFormRef = ref<FormInstance>()

const createForm = reactive({
  username: '',
  password: '',
  display_name: '',
  role: 'user'
})

const editForm = reactive({
  id: 0,
  username: '',
  display_name: '',
  role: 'user',
  enabled: true
})

const resetPasswordForm = reactive({
  id: 0,
  username: '',
  new_password: '',
  confirm_password: ''
})

const createRules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 32, message: '用户名长度为 3-32 个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 个字符', trigger: 'blur' }
  ],
  display_name: [
    { required: true, message: '请输入显示名称', trigger: 'blur' }
  ],
  role: [
    { required: true, message: '请选择角色', trigger: 'change' }
  ]
}

const editRules: FormRules = {
  display_name: [
    { required: true, message: '请输入显示名称', trigger: 'blur' }
  ],
  role: [
    { required: true, message: '请选择角色', trigger: 'change' }
  ]
}

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
  if (value !== resetPasswordForm.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const resetPasswordRules: FormRules = {
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 个字符', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const formatTime = (timestamp: number) => {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN')
}

const fetchUsers = async () => {
  loading.value = true
  try {
    const { data } = await authApi.listUsers()
    users.value = data.data
  } catch (error) {
    ElMessage.error('获取用户列表失败')
  } finally {
    loading.value = false
  }
}

const getCurrentUser = () => {
  const userStr = localStorage.getItem('logmon_user')
  if (userStr) {
    const user = JSON.parse(userStr)
    currentUserId.value = user.id
  }
}

const showCreateDialog = () => {
  Object.assign(createForm, {
    username: '',
    password: '',
    display_name: '',
    role: 'user'
  })
  createDialogVisible.value = true
}

const showEditDialog = (user: User) => {
  Object.assign(editForm, {
    id: user.id,
    username: user.username,
    display_name: user.display_name,
    role: user.role,
    enabled: user.enabled
  })
  editDialogVisible.value = true
}

const showResetPasswordDialog = (user: User) => {
  Object.assign(resetPasswordForm, {
    id: user.id,
    username: user.username,
    new_password: '',
    confirm_password: ''
  })
  resetPasswordDialogVisible.value = true
}

const handleCreate = async () => {
  if (!createFormRef.value) return

  await createFormRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      await authApi.createUser(createForm)
      ElMessage.success('用户创建成功')
      createDialogVisible.value = false
      fetchUsers()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '创建用户失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleUpdate = async () => {
  if (!editFormRef.value) return

  await editFormRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      await authApi.updateUser(editForm.id, {
        display_name: editForm.display_name,
        role: editForm.role,
        enabled: editForm.enabled
      })
      ElMessage.success('用户更新成功')
      editDialogVisible.value = false
      fetchUsers()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '更新用户失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleDelete = async (user: User) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 "${user.username}" 吗？此操作不可撤销。`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await authApi.deleteUser(user.id)
    ElMessage.success('用户删除成功')
    fetchUsers()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '删除用户失败')
    }
  }
}

const handleResetPassword = async () => {
  if (!resetPasswordFormRef.value) return

  await resetPasswordFormRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      await authApi.resetPassword(resetPasswordForm.id, {
        new_password: resetPasswordForm.new_password
      })
      ElMessage.success('密码重置成功')
      resetPasswordDialogVisible.value = false
    } catch (error: any) {
      ElMessage.error(error.response?.data?.error || '重置密码失败')
    } finally {
      submitting.value = false
    }
  })
}

onMounted(() => {
  getCurrentUser()
  fetchUsers()
})
</script>

<style scoped>
.users-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
