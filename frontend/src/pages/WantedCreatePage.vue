<template>
  <el-card class="wrap">
    <h2>发布求购需求</h2>
    <el-form :model="form" label-width="90px">
      <el-form-item label="求购标题" required>
        <el-input v-model="form.title" maxlength="128" show-word-limit placeholder="例如：求购 iPhone 13 128G" />
      </el-form-item>
      <el-form-item label="分类" required>
        <el-select v-model="form.category" placeholder="选择分类">
          <el-option v-for="(text, key) in ProductCategoryText" :key="key" :label="text" :value="key" />
        </el-select>
      </el-form-item>
      <el-form-item label="期望成色" required>
        <el-select v-model="form.condition" placeholder="选择可接受的成色">
          <el-option v-for="(text, key) in ProductConditionText" :key="key" :label="text" :value="key" />
        </el-select>
      </el-form-item>
      <el-form-item label="预算下限" required>
        <el-input-number v-model="form.budget_min" :min="0" :precision="2" />
      </el-form-item>
      <el-form-item label="预算上限" required>
        <el-input-number v-model="form.budget_max" :min="0.01" :precision="2" />
      </el-form-item>
      <el-form-item label="所在城市" required>
        <el-input v-model="form.city" maxlength="64" placeholder="例如：北京" />
      </el-form-item>
      <el-form-item label="期望说明" required>
        <el-input v-model="form.description" type="textarea" :rows="4" placeholder="描述期望的物品、成色要求、交易方式等" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="loading" @click="submit">立即发布</el-button>
        <el-button @click="$router.push('/wanteds')">取消</el-button>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as wantedApi from '../api/wanted'
import { ProductCategoryText, ProductConditionText } from '../constants'

const router = useRouter()
const loading = ref(false)
const form = reactive({
  title: '',
  category: '',
  condition: '',
  budget_min: 0,
  budget_max: 0,
  city: '',
  description: ''
})

async function submit() {
  if (!form.title || !form.category || !form.condition || !form.city || !form.description) {
    ElMessage.warning('请填写完整求购信息')
    return
  }
  if (form.budget_max <= 0) {
    ElMessage.warning('预算上限必须大于 0')
    return
  }
  if (form.budget_min > form.budget_max) {
    ElMessage.warning('预算下限不能大于预算上限')
    return
  }
  loading.value = true
  try {
    const res: any = await wantedApi.createWanted({ ...form })
    ElMessage.success('发布成功')
    router.push(`/wanteds/${res.data.id}`)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.wrap {
  max-width: 760px;
  margin: 0 auto;
}
</style>
