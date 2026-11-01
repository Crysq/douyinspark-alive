<template>
  <div class="register">
    <section class="plan-side">
      <div class="orb" />
      <div class="brand-top">
        <span class="mark">
          <svg viewBox="0 0 48 48"><defs><linearGradient id="rg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ffc542" /><stop offset=".55" stop-color="#ff7a18" /><stop offset="1" stop-color="#ff3d71" /></linearGradient></defs><path d="M24 5c4 7-5 9.5-5 17a5 5 0 0 0 10 0c0-3.4-1.6-5-1.6-7.4C32.8 18.5 36 24 36 30a12 12 0 1 1-24 0C12 20.5 19.5 15.5 24 5Z" fill="url(#rg)" /></svg>
        </span>
        <span>DoyinSpark</span>
      </div>

      <div class="intro">
        <p class="eyebrow">MEMBERSHIP</p>
        <h1>先有卡密<br /><span class="grad-text">再开始守护火花</span></h1>
        <p class="lede">注册必须使用一张未使用的卡密完成开通，之后可在会员中心随时续期。</p>
      </div>

      <div class="plan-list">
        <div v-for="plan in plans" :key="plan.code" class="plan-row">
          <div class="plan-name">
            <strong>{{ plan.name }}</strong>
            <span>{{ plan.desc }}</span>
          </div>
          <div class="plan-price">
            <b>¥{{ plan.price }}</b>
            <small>{{ plan.days ? plan.days + ' 天' : '永久' }}</small>
          </div>
        </div>
      </div>

      <p class="foot">卡密由管理员或经销商提供，一卡一码，兑换后立即生效。</p>
    </section>

    <section class="form-side">
      <div class="form-card">
        <div class="form-head">
          <h2>创建账号</h2>
          <p>填写账号信息并输入卡密，注册即开通会员</p>
        </div>

        <form @submit.prevent="submit">
          <div class="grid cols-2">
            <div class="field">
              <label for="username">用户名</label>
              <input id="username" v-model.trim="form.username" class="input" placeholder="3-32 位字母数字" autocomplete="username" />
            </div>
            <div class="field">
              <label for="nickname">昵称（可选）</label>
              <input id="nickname" v-model.trim="form.nickname" class="input" placeholder="显示名称" />
            </div>
          </div>

          <div class="field">
            <label for="password">密码</label>
            <input id="password" v-model="form.password" type="password" class="input" placeholder="至少 6 位" autocomplete="new-password" />
          </div>

          <div class="field">
            <label for="card">卡密 <span class="req">必填</span></label>
            <div class="card-row">
              <input id="card" v-model.trim="form.cardCode" class="input card-input" placeholder="DYSP-XXXX-XXXX-XXXX" @input="detectPlan" />
              <button type="button" class="btn btn-sm" @click="paste">粘贴</button>
            </div>
            <span v-if="preview" class="preview">
              将开通 <b>{{ preview }}</b>
            </span>
          </div>

          <p v-if="error" class="error">{{ error }}</p>

          <button class="btn btn-primary submit" type="submit" :disabled="auth.loading">
            <span v-if="auth.loading" class="spin loader" />
            <span>{{ auth.loading ? '正在开通…' : '注册并开通会员' }}</span>
          </button>
        </form>

        <p class="login-link">已有账号？<router-link to="/login"><b>直接登录 →</b></router-link></p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { membershipApi } from '@/api'
import type { Plan } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const auth = useAuthStore()
const ui = useUiStore()
const router = useRouter()

const plans = ref<Plan[]>([])
const preview = ref('')
const error = ref('')
const form = reactive({ username: '', nickname: '', password: '', cardCode: '' })

function detectPlan() {
  // 卡密本身不含卡种信息，这里只做格式提示
  const code = form.cardCode.toUpperCase().replace(/\s/g, '')
  preview.value = /^DYSP-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$/.test(code) ? '待校验卡密' : ''
}

async function paste() {
  try {
    const text = await navigator.clipboard.readText()
    form.cardCode = text.trim()
    detectPlan()
  } catch {
    ui.toast('error', '浏览器不允许读取剪贴板，请手动粘贴')
  }
}

async function submit() {
  error.value = ''
  if (!form.username || !form.password || !form.cardCode) {
    error.value = '请填写用户名、密码与卡密'
    return
  }
  try {
    const data = await auth.register({ ...form })
    ui.toast('success', `开通成功，${data.user.planLabel} 已生效`)
    router.push('/dashboard')
  } catch (err) {
    error.value = (err as Error).message
  }
}

onMounted(async () => {
  try {
    const data = await membershipApi.plans()
    plans.value = data.plans
  } catch { /* 忽略 */ }
})
</script>

<style scoped>
.register { display: grid; grid-template-columns: 1fr 1fr; min-height: 100vh; }

.plan-side {
  position: relative; overflow: hidden;
  display: flex; flex-direction: column; gap: 26px;
  padding: 42px 52px; border-right: 1px solid var(--line);
}
.orb { position: absolute; width: 520px; height: 520px; left: -160px; top: -180px; border-radius: 50%; filter: blur(90px); background: radial-gradient(circle, rgba(255, 122, 24, .5), transparent 65%); opacity: .5; }
.brand-top { display: flex; align-items: center; gap: 10px; font-weight: 680; position: relative; }
.mark { width: 34px; height: 34px; border-radius: 11px; display: grid; place-items: center; background: var(--flame-soft); box-shadow: inset 0 0 0 1px rgba(255, 138, 60, .25); }
.mark svg { width: 22px; height: 22px; }
.intro { position: relative; margin-top: 18px; }
.eyebrow { font-size: 11px; letter-spacing: .32em; color: var(--flame-2); font-weight: 650; margin-bottom: 14px; }
.intro h1 { font-size: clamp(30px, 3.4vw, 42px); line-height: 1.15; font-weight: 740; letter-spacing: -.03em; }
.lede { margin-top: 16px; font-size: 13px; line-height: 1.8; color: var(--muted); max-width: 420px; }

.plan-list { position: relative; display: flex; flex-direction: column; gap: 8px; }
.plan-row { display: flex; align-items: center; justify-content: space-between; padding: 11px 14px; border-radius: 13px; border: 1px solid var(--line); background: var(--surface); }
.plan-name { display: flex; flex-direction: column; gap: 2px; }
.plan-name strong { font-size: 13px; font-weight: 620; }
.plan-name span { font-size: 11px; color: var(--muted-2); }
.plan-price { text-align: right; display: flex; flex-direction: column; gap: 1px; }
.plan-price b { font-size: 15px; color: var(--flame-1); font-variant-numeric: tabular-nums; }
.plan-price small { font-size: 10px; color: var(--muted-2); }
.foot { position: relative; margin-top: auto; font-size: 11px; color: var(--muted-2); }

.form-side { display: grid; place-items: center; padding: 40px 28px; }
.form-card { width: min(430px, 100%); }
.form-head h2 { font-size: 24px; font-weight: 700; letter-spacing: -.03em; }
.form-head p { margin-top: 8px; font-size: 13px; color: var(--muted); }
form { margin-top: 24px; display: flex; flex-direction: column; gap: 15px; }
.req { color: var(--flame-2); font-size: 10px; margin-left: 4px; }
.card-row { display: flex; gap: 8px; }
.card-input { font-family: var(--font-mono); letter-spacing: .06em; text-transform: uppercase; }
.preview { font-size: 11px; color: var(--muted); }
.preview b { color: var(--safe); }
.submit { height: 46px; border-radius: 13px; margin-top: 6px; width: 100%; }
.loader { width: 15px; height: 15px; border-radius: 50%; border: 2px solid rgba(255, 255, 255, .35); border-top-color: #fff; }
.error { font-size: 12.5px; color: #ff8fa3; background: rgba(255, 90, 120, .1); border: 1px solid rgba(255, 90, 120, .22); padding: 9px 12px; border-radius: 10px; }
.login-link { margin-top: 20px; font-size: 12.5px; color: var(--muted); }
.login-link b { color: var(--flame-2); }

@media (max-width: 980px) {
  .register { grid-template-columns: 1fr; }
  .plan-side { padding: 24px 20px 6px; border-right: none; }
  .plan-list { display: none; }
  .foot { display: none; }
  .form-side { padding: 16px 20px 48px; }
}
</style>
