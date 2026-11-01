<template>
  <div class="login">
    <section class="brand-side">
      <div class="orb orb-a" />
      <div class="orb orb-b" />
      <div class="grid-lines" />
      <div class="brand-top">
        <span class="mark">
          <svg viewBox="0 0 48 48"><defs><linearGradient id="lg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ffc542" /><stop offset=".55" stop-color="#ff7a18" /><stop offset="1" stop-color="#ff3d71" /></linearGradient></defs><path d="M24 5c4 7-5 9.5-5 17a5 5 0 0 0 10 0c0-3.4-1.6-5-1.6-7.4C32.8 18.5 36 24 36 30a12 12 0 1 1-24 0C12 20.5 19.5 15.5 24 5Z" fill="url(#lg)" /></svg>
        </span>
        <span>DoyinSpark</span>
      </div>

      <div class="hero">
        <p class="eyebrow">DOUYIN SPARK KEEPER</p>
        <h1>把火花<br /><span class="grad-text">交给系统守护</span></h1>
        <p class="lede">
          多账号托管 · 24 小时窗口倒计时 · 定时自动续火 · 断火实时预警。<br />
          浏览器级真实会话，每一次发送都写入可追溯的记录。
        </p>
        <ul class="points">
          <li><span class="tick">✓</span> 真实登录态，扫码或 Cookie 导入</li>
          <li><span class="tick">✓</span> 时间窗口 + 轮询间隔，像人一样发送</li>
          <li><span class="tick">✓</span> 日卡 / 周卡 / 月卡 / 季卡 / 年卡 / 永久卡</li>
        </ul>
      </div>

      <div class="brand-foot">
        <span>Go · Gin · MySQL · Redis · Vue 3 · TypeScript</span>
      </div>
    </section>

    <section class="form-side">
      <div class="form-card">
        <template v-if="!auth.pending">
          <div class="form-head">
            <h2>登录控制台</h2>
            <p>使用你的账号进入火花守护系统</p>
          </div>

          <form @submit.prevent="submit">
            <div class="field">
              <label for="username">用户名</label>
              <div class="input-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="12" cy="8" r="3.6" /><path d="M4.5 20a7.5 7.5 0 0 1 15 0" /></svg>
                <input id="username" v-model.trim="username" class="input" placeholder="用户名" autocomplete="username" />
              </div>
            </div>

            <div class="field">
              <label for="password">密码</label>
              <div class="input-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><rect x="4" y="10" width="16" height="10" rx="2.5" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></svg>
                <input id="password" v-model="password" type="password" class="input" placeholder="••••••••" autocomplete="current-password" />
              </div>
            </div>

            <p v-if="error" class="error">{{ error }}</p>

            <button class="btn btn-primary submit" type="submit" :disabled="auth.loading">
              <span v-if="auth.loading" class="spin loader" />
              <span>{{ auth.loading ? '正在验证…' : '登录' }}</span>
            </button>
          </form>

          <router-link to="/register" class="register-link">
            还没有账号？<b>用卡密开通 →</b>
          </router-link>

          <div class="hint">
            <span>管理员 <code>admin</code> / <code>admin123</code></span>
          </div>
        </template>

        <template v-else>
          <div class="form-head">
            <span class="badge urgent">需要激活</span>
            <h2 class="mt-12">兑换卡密后登录</h2>
            <p>{{ auth.pending.user.nickname }}，你的会员未开通或已过期，输入卡密即可立即恢复。</p>
          </div>

          <div class="pending-card">
            <div><span>当前状态</span><b>{{ auth.pending.user.planLabel }}</b></div>
            <div><span>到期时间</span><b>{{ auth.pending.user.memberExpireAt ? formatDay(auth.pending.user.memberExpireAt) : '未开通' }}</b></div>
          </div>

          <form @submit.prevent="redeem">
            <div class="field">
              <label for="card">卡密</label>
              <input id="card" v-model="cardCode" class="input card-input" placeholder="DYSP-XXXX-XXXX-XXXX" />
            </div>
            <p v-if="error" class="error">{{ error }}</p>
            <button class="btn btn-primary submit" type="submit" :disabled="redeeming">
              {{ redeeming ? '兑换中…' : '兑换并进入' }}
            </button>
          </form>

          <button class="btn btn-ghost back" @click="cancelActivation">换个账号登录</button>
        </template>
      </div>

      <p class="copyright">© {{ year }} DoyinSpark · 仅供个人学习与自用</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'
import { formatDay } from '@/utils/format'

const auth = useAuthStore()
const ui = useUiStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const cardCode = ref('')
const error = ref('')
const redeeming = ref(false)
const year = new Date().getFullYear()

function go() {
  router.push((route.query.redirect as string) || '/dashboard')
}

async function submit() {
  error.value = ''
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }
  try {
    const result = await auth.login(username.value, password.value)
    if (result.needActivation) {
      ui.toast('info', '会员未激活，请兑换卡密')
      return
    }
    ui.toast('success', '欢迎回来，火花已就位')
    go()
  } catch (err) {
    error.value = (err as Error).message
  }
}

async function redeem() {
  error.value = ''
  if (!cardCode.value) {
    error.value = '请输入卡密'
    return
  }
  redeeming.value = true
  try {
    const data = await auth.redeem(cardCode.value)
    ui.toast('success', `${data.card.name}兑换成功`)
    go()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    redeeming.value = false
  }
}

function cancelActivation() {
  auth.pending = null
  error.value = ''
  cardCode.value = ''
}
</script>

<style scoped>
.login { display: grid; grid-template-columns: 1.05fr 1fr; min-height: 100vh; }

.brand-side {
  position: relative; overflow: hidden;
  display: flex; flex-direction: column; justify-content: space-between;
  padding: 42px 56px; border-right: 1px solid var(--line);
}
.orb { position: absolute; border-radius: 50%; filter: blur(80px); opacity: .55; pointer-events: none; }
.orb-a { width: 460px; height: 460px; left: -120px; top: -140px; background: radial-gradient(circle, rgba(255, 122, 24, .55), transparent 65%); }
.orb-b { width: 420px; height: 420px; right: -100px; bottom: -160px; background: radial-gradient(circle, rgba(255, 61, 113, .42), transparent 65%); }
.grid-lines {
  position: absolute; inset: 0; pointer-events: none; opacity: .5;
  background-image: linear-gradient(rgba(255, 255, 255, .04) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, .04) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: radial-gradient(ellipse at 40% 40%, #000 20%, transparent 72%);
}
.brand-top { display: flex; align-items: center; gap: 10px; font-weight: 680; letter-spacing: -.01em; position: relative; }
.mark { width: 34px; height: 34px; border-radius: 11px; display: grid; place-items: center; background: var(--flame-soft); box-shadow: inset 0 0 0 1px rgba(255, 138, 60, .25); }
.mark svg { width: 22px; height: 22px; }

.hero { position: relative; margin-top: auto; margin-bottom: auto; max-width: 520px; }
.eyebrow { font-size: 11px; letter-spacing: .34em; color: var(--flame-2); font-weight: 650; margin-bottom: 18px; }
.hero h1 { font-size: clamp(34px, 4.2vw, 52px); line-height: 1.1; font-weight: 750; letter-spacing: -.035em; }
.lede { margin-top: 22px; font-size: 14px; line-height: 1.85; color: var(--muted); }
.points { margin-top: 28px; display: flex; flex-direction: column; gap: 12px; font-size: 13px; color: var(--text-2); }
.points li { display: flex; align-items: center; gap: 10px; }
.tick { width: 20px; height: 20px; display: grid; place-items: center; border-radius: 50%; background: rgba(255, 122, 24, .14); color: var(--flame-2); font-size: 11px; }
.brand-foot { position: relative; font-size: 11px; color: var(--muted-2); letter-spacing: .04em; }

.form-side { display: grid; place-items: center; padding: 40px 28px; }
.form-card { width: min(392px, 100%); }
.form-head h2 { font-size: 24px; font-weight: 700; letter-spacing: -.03em; }
.form-head p { margin-top: 8px; font-size: 13px; color: var(--muted); line-height: 1.6; }
form { margin-top: 26px; display: flex; flex-direction: column; gap: 16px; }
.submit { height: 46px; border-radius: 13px; font-size: 14px; margin-top: 4px; width: 100%; }
.loader { width: 15px; height: 15px; border-radius: 50%; border: 2px solid rgba(255, 255, 255, .35); border-top-color: #fff; }
.error { font-size: 12.5px; color: #ff8fa3; background: rgba(255, 90, 120, .1); border: 1px solid rgba(255, 90, 120, .22); padding: 9px 12px; border-radius: 10px; }
.hint { margin-top: 22px; display: flex; align-items: center; gap: 8px; font-size: 11.5px; color: var(--muted-2); flex-wrap: wrap; }
.hint code { font-family: var(--font-mono); font-size: 11px; padding: 1px 5px; border-radius: 5px; background: var(--surface-2); color: var(--text-2); }
.register-link { display: inline-block; margin-top: 20px; font-size: 12.5px; color: var(--muted); }
.register-link b { color: var(--flame-2); font-weight: 600; }
.register-link:hover b { text-decoration: underline; }

.pending-card { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 20px; padding: 14px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface); }
.pending-card div { display: flex; flex-direction: column; gap: 3px; }
.pending-card span { font-size: 10.5px; color: var(--muted-2); }
.pending-card b { font-size: 13px; font-weight: 620; color: var(--text-2); }
.card-input { font-family: var(--font-mono); letter-spacing: .08em; text-transform: uppercase; }
.back { width: 100%; margin-top: 12px; font-size: 12px; }
.copyright { margin-top: 40px; font-size: 11px; color: var(--muted-2); text-align: center; }

@media (max-width: 980px) {
  .login { grid-template-columns: 1fr; }
  .brand-side { padding: 26px 24px 8px; min-height: auto; border-right: none; }
  .hero { margin: 30px 0 10px; }
  .hero h1 { font-size: 32px; }
  .lede { font-size: 13px; }
  .points, .brand-foot { display: none; }
  .form-side { padding: 18px 20px 48px; }
}
</style>
