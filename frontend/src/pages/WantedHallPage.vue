<template>
  <div>
    <div class="filter-bar">
      <el-input v-model="params.keyword" placeholder="搜索求购需求" clearable class="kw" @keyup.enter="load(1)" />
      <el-select v-model="params.category" placeholder="全部分类" clearable class="sel" @change="load(1)">
        <el-option v-for="(text, key) in ProductCategoryText" :key="key" :label="text" :value="key" />
      </el-select>
      <el-input v-model="params.city" placeholder="城市，如：北京" clearable class="city" @keyup.enter="load(1)" />
      <el-button type="primary" @click="load(1)">搜索</el-button>
      <el-button v-if="userStore.isLoggedIn" type="success" @click="$router.push('/wanteds/create')">发布求购</el-button>
    </div>
    <div v-loading="loading">
      <div class="list" v-if="wanteds.length">
        <el-card v-for="w in wanteds" :key="w.id" class="wanted-card" shadow="hover" @click="$router.push(`/wanteds/${w.id}`)">
          <div class="title-row">
            <span class="title">{{ w.title }}</span>
            <StatusBadge type="wanted" :value="w.status" />
          </div>
          <div class="budget">预算 ¥{{ formatPrice(w.budget_min) }} ~ ¥{{ formatPrice(w.budget_max) }}</div>
          <div class="meta">
            <el-tag size="small" effect="plain">{{ formatCategory(w.category) }}</el-tag>
            <el-tag size="small" effect="plain" type="info">{{ formatCondition(w.condition) }}</el-tag>
            <span class="city">{{ w.city }}</span>
          </div>
          <div class="desc">{{ w.description }}</div>
          <div class="foot">
            <span>{{ w.user?.nickname || `用户${w.user_id}` }}</span>
            <span>{{ w.created_at }}</span>
          </div>
        </el-card>
      </div>
      <EmptyState v-if="!loading && wanteds.length === 0" description="暂无求购需求" />
    </div>
    <div class="pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="load"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import * as wantedApi from '../api/wanted'
import { useUserStore } from '../stores/userStore'
import StatusBadge from '../components/StatusBadge.vue'
import EmptyState from '../components/EmptyState.vue'
import { ProductCategoryText } from '../constants'
import { formatPrice, formatCategory, formatCondition } from '../utils/format'

const userStore = useUserStore()
const wanteds = ref<any[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = 12
const params = reactive<Record<string, unknown>>({ keyword: '', category: '', city: '' })

onMounted(() => load(1))

async function load(p: number) {
  loading.value = true
  try {
    const clean: Record<string, unknown> = {}
    Object.entries(params).forEach(([k, v]) => {
      if (v !== '' && v !== undefined && v !== null) clean[k] = v
    })
    const res: any = await wantedApi.listWanteds({ ...clean, page: p, page_size: pageSize })
    wanteds.value = res.data.list || []
    total.value = Number(res.data.total || 0)
    page.value = p
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.filter-bar {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
  padding: 12px 0;
}
.kw {
  width: 240px;
}
.sel {
  width: 140px;
}
.city {
  width: 150px;
}
.list {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}
.wanted-card {
  cursor: pointer;
}
.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.title {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.budget {
  color: #f56c6c;
  font-weight: 700;
  margin: 8px 0;
}
.meta {
  display: flex;
  align-items: center;
  gap: 8px;
}
.meta .city {
  color: #909399;
  font-size: 13px;
}
.desc {
  color: #606266;
  font-size: 13px;
  margin: 8px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.foot {
  display: flex;
  justify-content: space-between;
  color: #c0c4cc;
  font-size: 12px;
}
.pager {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}
</style>
