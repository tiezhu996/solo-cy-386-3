<template>
  <div v-loading="loading">
    <el-card v-if="wanted" class="wrap">
      <div class="head">
        <h2>{{ wanted.title }}</h2>
        <StatusBadge type="wanted" :value="wanted.status" />
      </div>
      <div class="budget">预算 ¥{{ formatPrice(wanted.budget_min) }} ~ ¥{{ formatPrice(wanted.budget_max) }}</div>
      <el-descriptions :column="1" border class="desc">
        <el-descriptions-item label="分类">{{ formatCategory(wanted.category) }}</el-descriptions-item>
        <el-descriptions-item label="期望成色">{{ formatCondition(wanted.condition) }}</el-descriptions-item>
        <el-descriptions-item label="所在城市">{{ wanted.city }}</el-descriptions-item>
        <el-descriptions-item label="发布时间">{{ wanted.created_at }}</el-descriptions-item>
      </el-descriptions>
      <div class="publisher">
        <el-avatar :size="40">{{ (wanted.user?.nickname || 'U').slice(0, 1) }}</el-avatar>
        <div class="publisher-info">
          <div>{{ wanted.user?.nickname || `用户${wanted.user_id}` }}</div>
          <div class="credit">信用分：{{ wanted.user?.credit_score ?? '-' }}</div>
        </div>
      </div>
      <el-divider content-position="left">期望说明</el-divider>
      <p class="desc-text">{{ wanted.description }}</p>
      <div class="actions">
        <template v-if="isOwner">
          <el-button v-if="wanted.status === 'open'" type="warning" size="large" :loading="closing" @click="closeWanted">关闭求购</el-button>
        </template>
        <el-button v-else type="primary" size="large" @click="contactPublisher">联系发布者</el-button>
        <el-button size="large" @click="$router.push('/wanteds')">返回大厅</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as wantedApi from '../api/wanted'
import { useUserStore } from '../stores/userStore'
import StatusBadge from '../components/StatusBadge.vue'
import { formatPrice, formatCondition, formatCategory } from '../utils/format'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const wanted = ref<any>(null)
const loading = ref(false)
const closing = ref(false)

const isOwner = computed(() => !!userStore.user && wanted.value && userStore.user.id === wanted.value.user_id)

onMounted(load)

async function load() {
  loading.value = true
  try {
    const res: any = await wantedApi.getWanted(Number(route.params.id))
    wanted.value = res.data
  } finally {
    loading.value = false
  }
}

function contactPublisher() {
  if (!userStore.isLoggedIn) {
    router.push('/login')
    return
  }
  router.push({ path: '/messages', query: { peer_id: wanted.value.user_id } })
}

async function closeWanted() {
  try {
    await ElMessageBox.confirm('关闭后求购将不再出现在大厅，确定关闭吗？', '关闭求购', { type: 'warning' })
  } catch {
    return
  }
  closing.value = true
  try {
    await wantedApi.closeWanted(wanted.value.id)
    ElMessage.success('求购已关闭')
    await load()
  } finally {
    closing.value = false
  }
}
</script>

<style scoped>
.wrap {
  max-width: 860px;
  margin: 0 auto;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.budget {
  color: #f56c6c;
  font-size: 24px;
  font-weight: 700;
  margin: 8px 0 16px;
}
.desc {
  margin-top: 8px;
}
.publisher {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 16px 0;
}
.credit {
  font-size: 12px;
  color: #909399;
}
.desc-text {
  color: #606266;
  line-height: 1.8;
  white-space: pre-wrap;
}
.actions {
  margin-top: 16px;
}
</style>
