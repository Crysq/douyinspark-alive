/**
 * DoyinSpark 签名服务
 *
 * 抖音 Web 接口需要 a_bogus / X-Bogus 签名，签名由页面里的 byted_acrawler 生成，
 * 设备指纹（ttwid / fpk1 / UIFID）也绑定在同一个浏览器实例上。
 * 因此这里常驻一个 Chromium：登录、签名、发送请求都发生在同一个会话里。
 *
 * 每个账号一个独立 user-data-dir，互不串号。
 */
import express from 'express'
import puppeteer from 'puppeteer-core'
import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'

const PORT = Number(process.env.PORT || 9100)
const PROFILE_ROOT = path.resolve(process.env.PROFILE_DIR || 'profiles')
const CHROME_PATH = process.env.CHROME_PATH ||
  (process.platform === 'darwin'
    ? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
    : process.platform === 'win32'
      ? 'C:/Program Files/Google/Chrome/Application/chrome.exe'
      : '/usr/bin/chromium')
const HEADLESS = (process.env.HEADLESS ?? 'false') !== 'false'
const HOME = 'https://www.douyin.com/'
const sessions = new Map()
const log = (...args) => console.log(new Date().toISOString(), ...args)
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

function accountKey(value) {
  const id = Number(value || 1)
  if (!Number.isSafeInteger(id) || id < 1) throw new Error('accountId 必须是正整数')
  return String(id)
}

async function ensureSession(value) {
  const id = accountKey(value)
  const existing = sessions.get(id)
  if (existing?.browser && existing.page) return existing
  if (existing?.launching) return existing.launching

  const session = { id, browser: null, page: null, launching: null, cookieHash: '' }
  const profileDir = path.join(PROFILE_ROOT, `acct-${id}`)
  session.launching = (async () => {
    fs.mkdirSync(profileDir, { recursive: true })
    // 容器重建后 Chromium 会留下跨主机名单例锁，不清掉会提示“配置文件被占用”
    for (const name of ['SingletonLock', 'SingletonCookie', 'SingletonSocket']) {
      fs.rmSync(path.join(profileDir, name), { force: true })
    }
    log(`启动会话 account=${id} headless=${HEADLESS}`)
    const browser = await puppeteer.launch({
      executablePath: CHROME_PATH,
      headless: HEADLESS,
      userDataDir: profileDir,
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage',
        '--disable-gpu', '--disable-blink-features=AutomationControlled', '--lang=zh-CN'],
      defaultViewport: { width: 1400, height: 900 },
    })
    session.browser = browser
    browser.on('disconnected', () => {
      session.browser = null
      session.page = null
      session.cookieHash = ''
    })
    const pages = await browser.pages()
    session.page = pages[0] || await browser.newPage()
    await session.page.evaluateOnNewDocument(() => {
      Object.defineProperty(navigator, 'webdriver', { get: () => undefined })
    })
    await session.page.setExtraHTTPHeaders({ 'Accept-Language': 'zh-CN,zh;q=0.9' })
    await session.page.goto(HOME, { waitUntil: 'domcontentloaded', timeout: 45000 })
    await session.page
      .waitForFunction(() => typeof window.byted_acrawler?.frontierSign === 'function', { timeout: 20000 })
      .catch(() => log(`账号 ${id} 的签名脚本尚未就绪`))
    return session
  })()
  sessions.set(id, session)
  try {
    return await session.launching
  } catch (error) {
    sessions.delete(id)
    if (session.browser) await session.browser.close().catch(() => {})
    throw error
  } finally {
    session.launching = null
  }
}

async function syncCookie(session, raw) {
  if (!raw?.trim()) return
  const hash = createHash('sha256').update(raw).digest('hex')
  if (session.cookieHash === hash) return
  const client = await session.page.createCDPSession()
  try {
    const pairs = raw.split(';').map((part) => part.trim()).filter(Boolean)
    const cookies = []
    for (const pair of pairs) {
      const index = pair.indexOf('=')
      if (index < 1) continue
      cookies.push({ name: pair.slice(0, index), value: pair.slice(index + 1), domain: '.douyin.com', path: '/', url: HOME })
    }
    if (!cookies.length) throw new Error('Cookie 格式应为 name=value; name=value')
    await client.send('Network.setCookies', { cookies })
    session.cookieHash = hash
    await session.page.reload({ waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(1200)
  } finally {
    await client.detach().catch(() => {})
  }
}

async function status(accountID) {
  const session = await ensureSession(accountID)
  const client = await session.page.createCDPSession()
  try {
    const { cookies } = await client.send('Network.getAllCookies')
    const names = new Set(cookies.map((cookie) => cookie.name))
    return {
      logged: names.has('sessionid') || names.has('sessionid_ss'),
      cookieCount: cookies.length,
      nickname: '',
    }
  } finally {
    await client.detach().catch(() => {})
  }
}

async function openLogin(accountID) {
  const current = await status(accountID)
  if (current.logged) return { logged: true, ...current }
  const { page } = await ensureSession(accountID)
  await page.goto(HOME, { waitUntil: 'domcontentloaded', timeout: 45000 })
  await page.evaluate(() => {
    const button = [...document.querySelectorAll('button')]
      .filter((el) => el.textContent?.trim() === '登录')
      .find((el) => {
        const rect = el.getBoundingClientRect()
        return rect.width > 0 && rect.left > window.innerWidth * 0.65 && rect.top < 120
      })
    button?.click()
  })
  await page.waitForFunction(() => {
    return [...document.querySelectorAll('img[src^="data:image"]')].some((el) => {
      const rect = el.getBoundingClientRect()
      return rect.width >= 120 && rect.width <= 300 && Math.abs(rect.width - rect.height) < 40
    })
  }, { timeout: 15000 }).catch(() => {})
  await sleep(500)
  const qr = await page.evaluate(() => {
    const image = [...document.querySelectorAll('img[src^="data:image"]')].find((el) => {
      const rect = el.getBoundingClientRect()
      return rect.width >= 120 && rect.width <= 300 && Math.abs(rect.width - rect.height) < 40
    })
    if (!image) return null
    return { src: image.currentSrc || image.src, width: Math.round(image.getBoundingClientRect().width) }
  })
  if (qr?.src) return { logged: false, dataUrl: qr.src, width: qr.width, fallback: false }
  const shot = await page.screenshot({ type: 'png' })
  return { logged: false, dataUrl: `data:image/png;base64,${shot.toString('base64')}`, fallback: true }
}

async function accountProfile(accountId) {
  const url = 'https://www.douyin.com/aweme/v1/web/user/profile/self/?device_platform=webapp&aid=6383&channel=channel_pc_web&pc_client_type=1&version_code=190500&version_name=19.5.0&cookie_enabled=true&platform=PC'
  const result = await siteFetch({ accountId, url, method: 'GET' })
  if (result.error) throw new Error(result.error)
  if (result.status !== 200) throw new Error(`资料接口返回 ${result.status}`)
  let data
  try { data = JSON.parse(result.body) } catch { throw new Error('资料接口返回非 JSON') }
  const user = data.user || data.data?.user || data.user_info
  if (!user) throw new Error('登录态无效：资料接口没有返回用户信息')
  const avatar = user.avatar_larger?.url_list?.[0] || user.avatar_thumb?.url_list?.[0] || ''
  return {
    uid: String(user.uid || user.user_id || ''),
    nickname: String(user.nickname || ''),
    uniqueId: String(user.unique_id || user.short_id || ''),
    secUid: String(user.sec_uid || ''),
    avatarUrl: avatar,
  }
}

async function siteFetch({ accountId, cookie, url, method = 'POST', body = '', headers = {} }) {
  const session = await ensureSession(accountId)
  await syncCookie(session, cookie)
  const target = new URL(url, HOME)
  if (target.protocol !== 'https:' || !/^(?:[a-z0-9-]+\.)*douyin\.com$/i.test(target.hostname)) {
    throw new Error('仅支持抖音 HTTPS 接口')
  }
  return session.page.evaluate(async ({ url, method, body, headers }) => {
    const ac = window.byted_acrawler
    if (!ac || typeof ac.frontierSign !== 'function') {
      return { status: 503, body: '', error: '页面签名脚本未就绪，请刷新会话后重试' }
    }
    try {
      const target = new URL(url)
      let payload = body
      if (method !== 'GET' && !/[?&]csrf_token=/.test(payload)) {
        const token = document.cookie.match(/(?:^|;\s*)passport_csrf_token=([^;]+)/)?.[1]
        if (!token) return { status: 401, body: '', error: '登录 Cookie 缺少 passport_csrf_token' }
        payload += `${payload ? '&' : ''}csrf_token=${encodeURIComponent(token)}`
      }
      const signature = ac.frontierSign({ url: target.toString(), data: payload, method }) || {}
      for (const [key, value] of Object.entries(signature)) target.searchParams.set(key, value)
      const response = await fetch(target.toString(), {
        method,
        headers: { 'content-type': 'application/x-www-form-urlencoded', ...headers },
        body: method === 'GET' ? undefined : payload,
        credentials: 'include',
      })
      return { status: response.status, body: (await response.text()).slice(0, 20000) }
    } catch (error) {
      return { status: 500, body: '', error: String(error?.message || error) }
    }
  }, { url: target.toString(), method: method.toUpperCase(), body, headers })
}

const app = express()
app.disable('x-powered-by')
app.use(express.json({ limit: '2mb' }))

const handle = (fn) => async (req, res) => {
  try {
    res.json(await fn(req))
  } catch (error) {
    log(error.message)
    res.status(500).json({ error: error.message })
  }
}
const idOf = (req) => req.body?.accountId ?? req.query.accountId ?? 1

const health = async () => ({ ok: true, ready: true, headless: HEADLESS })
app.get('/health', handle(health))
app.post('/health', handle(health))
app.get('/login/status', handle(async (req) => status(idOf(req))))
app.post('/login/status', handle(async (req) => status(idOf(req))))
app.get('/login/qr', handle(async (req) => openLogin(idOf(req))))
app.post('/login/open', handle(async (req) => openLogin(idOf(req))))
app.post('/fetch', handle(async (req) => {
  if (!req.body?.url) return { status: 400, body: '', error: 'url 必填' }
  return siteFetch(req.body)
}))
app.post('/account/profile', handle(async (req) => accountProfile(idOf(req))))
app.post('/cookie/import', handle(async (req) => {
  if (!req.body?.cookie) throw new Error('cookie 必填')
  const session = await ensureSession(idOf(req))
  await syncCookie(session, req.body.cookie)
  return status(idOf(req))
}))
app.post('/logout', handle(async (req) => {
  const id = accountKey(idOf(req))
  const session = sessions.get(id)
  if (session) {
    await (session.launching || Promise.resolve())
    if (session.browser) await session.browser.close()
    sessions.delete(id)
  }
  await fs.promises.rm(path.join(PROFILE_ROOT, `acct-${id}`), { recursive: true, force: true })
  return { logged: false }
}))

const server = app.listen(PORT, '0.0.0.0', () => log(`签名服务已启动 http://0.0.0.0:${PORT}`))
async function shutdown() {
  server.close()
  await Promise.all([...sessions.values()].map((session) => session.browser?.close().catch(() => {})))
  process.exit(0)
}
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, shutdown)
