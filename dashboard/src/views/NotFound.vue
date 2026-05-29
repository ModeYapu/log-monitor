<template>
  <div class="not-found-page">
    <div class="not-found-content">
      <h1>404</h1>
      <p>抱歉，页面未找到</p>
      <el-button type="primary" @click="goHome">返回首页</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()

const goHome = () => {
  router.push('/')
}
</script>

<style scoped>
.not-found-page {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1a1f35 0%, #0f1419 100%);
}

.not-found-content {
  text-align: center;
  color: #e0e6ed;
}

.not-found-content h1 {
  font-size: 120px;
  font-weight: 700;
  margin: 0 0 20px 0;
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.not-found-content p {
  font-size: 18px;
  margin: 0 0 30px 0;
  color: #94a3b8;
}
</style>
