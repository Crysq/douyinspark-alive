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
        '--disable-gpu', '--disable-blink-features=AutomationControlled', '--lang=zh-CN',
        ],
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
    session.capture = []
    session.wsFrames = []
    const attachCapture = (page) => {
      if (!page || page.__capturing) return
      page.__capturing = true
      page.on('request', (request) => {
        try {
          const url = request.url()
          if (!url.includes('imapi.douyin.com')) return
          const post = request.postData() || ''
          if (session.capture.length > 60) session.capture.shift()
          session.capture.push({
            at: new Date().toISOString(), url: url.slice(0, 300), method: request.method(),
            ct: request.headers()['content-type'] || '',
            bodyB64: post ? Buffer.from(post, 'latin1').toString('base64') : '',
            bodyText: post ? post.slice(0, 200) : '',
          })
        } catch (e) { /* 捕获失败不影响主流程 */ }
      })
      page.on('websocket', (ws) => {
        try {
          ws.on('framesent', (f) => {
            if (session.wsFrames.length > 80) session.wsFrames.shift()
            session.wsFrames.push({ at: new Date().toISOString(), url: ws.url().slice(0, 200), payload: typeof f.payload === 'string' ? f.payload.slice(0, 500) : '[binary]' })
          })
        } catch (e) { }
      })
    }
    attachCapture(session.page)
    browser.on('targetcreated', async (target) => {
      try { const p = await target.page(); if (p) attachCapture(p) } catch (e) { }
    })
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
      return { status: response.status, body: (await response.text()).slice(0, 300000) }
    } catch (error) {
      return { status: 500, body: '', error: String(error?.message || error) }
    }
  }, { url: target.toString(), method: method.toUpperCase(), body, headers })
}

/**
 * 页面内发送：真实键盘事件 + 回执确认。
 * 参考 DouYinSparkFlow 的做法——不构造任何请求，让页面自己完成签名与设备指纹。
 *
 * deliveryState:
 *   confirmed  消息出现在会话里（已送达）
 *   uncertain  已尝试但未看到回执（调用方不要重试，避免重复消息）
 *   not_sent   明确未发出（可重试）
 */
async function sendChatMessage({ accountId, nickname, text, debug: req_debug }) {
  const session = await ensureSession(accountId)
  const page = await session.browser.newPage()
  const result = { ok: false, deliveryState: 'not_sent', via: 'page', reason: '' }
  log('发送开始', JSON.stringify({ accountId, nickname, text: String(text).slice(0, 60) }))
  const finish = (state, reason) => { result.deliveryState = state; result.ok = state === 'confirmed'; result.reason = reason; return result }
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await page.waitForSelector('[data-e2e="conversation-item"]', { timeout: 30000 })
    await sleep(2000)

    // 1) 按昵称在会话列表里定位，并用真实鼠标点击打开会话
    const pos = await page.evaluate((name) => {
      const items = [...document.querySelectorAll('[data-e2e="conversation-item"]')]
      const el = items.find((it) => name && (it.textContent || '').includes(name))
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width * 0.4, y: r.y + r.height / 2 }
    }, nickname)
    if (!pos) return finish('not_sent', `会话列表里找不到「${nickname}」，请先同步会话`)
    await page.mouse.click(pos.x, pos.y)
    await sleep(2500)

    // 2) 等输入框出现（Draft.js 的 contenteditable）
    const editorPos = await page.evaluate(() => {
      const el = document.querySelector('div[contenteditable="true"], textarea')
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
    })
    if (!editorPos) return finish('not_sent', '聊天面板没有打开，输入框未出现')
    await page.mouse.click(editorPos.x, editorPos.y)
    await sleep(300)

    // 3) 真实键盘输入；多行用 Shift+Enter 换行，最后再按 Enter 发送
    const lines = String(text).split(/\r?\n/)
    for (let i = 0; i < lines.length; i++) {
      if (lines[i]) await page.keyboard.type(lines[i], { delay: 18 })
      if (i < lines.length - 1) {
        await page.keyboard.down('Shift')
        await page.keyboard.press('Enter')
        await page.keyboard.up('Shift')
        await sleep(200)
      }
    }
    await sleep(400)

    // 4) 优先点「发送」按钮，没有就回车
    const sentByButton = await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button, [role="button"]')]
        .find((e) => /发送/.test((e.textContent || '').trim()))
      if (!btn) return false
      const r = btn.getBoundingClientRect()
      if (r.width < 4) return false
      btn.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      return true
    })
    if (!sentByButton) await page.keyboard.press('Enter')
    await sleep(2500)

    // 5) 回执：去掉空白后，在最近 2000 字里找我们刚发的文本
    const seen = await page.evaluate((t) => {
      const norm = (v) => (v || '').replace(/\s+/g, '')
      const panel = norm(document.body.innerText).slice(-2000)
      return panel.includes(norm(t))
    }, text.slice(0, 30))
    if (req_debug) {
      result.debug = await page.evaluate(() => ({
        url: location.href,
        editor: (() => { const e = document.querySelector('div[contenteditable="true"], textarea'); return e ? (e.textContent || e.value || '').slice(0, 60) : null })(),
        text: (document.body.innerText || '').slice(-400),
      }))
      result.image = (await page.screenshot({ type: 'jpeg', quality: 45, encoding: 'base64' })).slice(0, 200000)
    }
    if (seen) { log('发送确认', nickname); return finish('confirmed', '') }
    log('回执未确认', nickname, 'tail=', await page.evaluate(() => (document.body.innerText || '').slice(-120)))
    return finish('uncertain', '已尝试发送，但未在会话里看到回执')
  } catch (error) {
    return finish('not_sent', String(error && error.message || error))
  } finally {
    await page.close().catch(() => {})
  }
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
app.post('/im/send', handle(async (req) => {
  if (!req.body?.text) throw new Error('text 必填')
  return sendChatMessage({ accountId: idOf(req), nickname: req.body.nickname, text: String(req.body.text).slice(0, 800), debug: req.body.debug })
}))

app.post('/account/profile', handle(async (req) => accountProfile(idOf(req))))

// 记录聊天页建立的 WebSocket（地址 + 前几帧）
app.post('/ws-record', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  const sockets = []
  page.on('websocket', (ws) => {
    const rec = { url: ws.url().slice(0, 300), sent: [], received: 0 }
    sockets.push(rec)
    ws.on('framesent', (f) => { if (rec.sent.length < 8) rec.sent.push(typeof f.payload === 'string' ? f.payload.slice(0, 300) : '[bin]') })
    ws.on('framereceived', () => { rec.received += 1 })
  })
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(Number(req.body?.seconds || 12) * 1000)
  } finally {
    await page.close().catch(() => {})
  }
  return { sockets: sockets.slice(0, 6) }
}))

// 录制一次真实发送请求（人工在 noVNC 里点发送时触发，监听所有页面）
app.get('/captured', handle(async (req) => {
  const session = await ensureSession(idOf(req))
  return { http: (session.capture || []).slice(-20), ws: (session.wsFrames || []).slice(-20) }
}))

app.post('/record-send', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const hits = []
  const wsHits = []
  const attach = (page) => {
    page.on('request', (request) => {
      const url = request.url()
      if (!/imapi\.douyin\.com/.test(url)) return
      const post = request.postData() || ''
      hits.push({
        url: url.slice(0, 300), method: request.method(),
        ct: request.headers()['content-type'] || '',
        bodyB64: post ? Buffer.from(post, 'latin1').toString('base64') : '',
      })
    })
    page.on('websocket', (ws) => {
      ws.on('framesent', (f) => {
        if (wsHits.length > 40) return
        wsHits.push({ url: ws.url().slice(0, 200), payload: typeof f.payload === 'string' ? f.payload.slice(0, 600) : '[binary]' })
      })
    })
  }
  const pages = await session.browser.pages()
  pages.forEach(attach)
  const onTarget = async (target) => {
    try { const page = await target.page(); if (page) attach(page) } catch (e) { }
  }
  session.browser.on('targetcreated', onTarget)
  const seconds = Math.min(900, Number(req.body?.seconds || 120))
  try {
    await sleep(seconds * 1000)
  } finally {
    session.browser.off('targetcreated', onTarget)
  }
  return { http: hits.filter((h) => /send|create/i.test(h.url)).slice(0, 5), allHttp: hits.slice(0, 12), ws: wsHits.slice(0, 20), waited: seconds }
}))

// 用页面签名能力给 imapi 请求签名（a_bogus / msToken），再由 Node 发出
app.post('/signed-send', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  let bodyBuf
  if (req.body?.bodyB64) {
    bodyBuf = Buffer.from(req.body.bodyB64, 'base64')
  } else throw new Error('需要 bodyB64')

  const query = req.body?.query || ''
  const baseURL = 'https://imapi.douyin.com' + (req.body?.path || '/v1/message/send') + (query ? '?' + query : '')
  const bin = bodyBuf.toString('latin1')

  // 确保页面在 douyin.com 主域，securitySDK 才会加载
  try {
    const hasSDK = await session.page.evaluate(() => typeof window.securitySDK !== 'undefined')
    if (!hasSDK || !session.page.url().includes('douyin.com')) {
      await session.page.goto('https://www.douyin.com/', { waitUntil: 'domcontentloaded', timeout: 45000 })
      await sleep(4000)
    }
  } catch (e) { /* ignore */ }

  // 页面里同时生成 a_bogus 与 bd-ticket-guard 头
  const pageOut = await session.page.evaluate(async ({ url, data }) => {
    const out = { params: {}, guard: {}, msToken: '' }
    try {
      const ac = window.byted_acrawler
      if (ac && typeof ac.frontierSign === 'function') out.params = ac.frontierSign({ url, data, method: 'POST' }) || {}
    } catch (e) { out.sigError = String(e && e.message || e) }
    try {
      const sdk = window.securitySDK
      if (sdk && typeof sdk.getBdTicketGuardHeader === 'function') {
        const res = await sdk.getBdTicketGuardHeader(url, data)
        out.guard = (res && res.bdTicketGuardHeaders) || {}
      }
    } catch (e) { out.guardError = String(e && e.message || e) }
    try {
      const m = document.cookie.match(/(?:^|;\s*)msToken=([^;]+)/)
      if (m) out.msToken = m[1]
    } catch (e) { }
    return out
  }, { url: baseURL, data: bin })

  const u = new URL(baseURL)
  for (const [k, v] of Object.entries(pageOut.params || {})) u.searchParams.set(k, String(v))
  if (pageOut.msToken && !u.searchParams.get('msToken')) u.searchParams.set('msToken', pageOut.msToken)
  if (req.body?.query) for (const [k, v] of new URLSearchParams(req.body.query)) if (!u.searchParams.get(k)) u.searchParams.set(k, v)

  const client = await session.page.createCDPSession()
  let cookieHeader = ''
  try {
    const { cookies } = await client.send('Network.getAllCookies')
    cookieHeader = cookies.filter((c) => c.domain.endsWith('douyin.com')).map((c) => `${c.name}=${c.value}`).join('; ')
  } finally { await client.detach().catch(() => {}) }

  const headers = {
    'content-type': 'application/x-protobuf',
    accept: 'application/x-protobuf',
    origin: 'https://www.douyin.com',
    referer: 'https://www.douyin.com/',
    'user-agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36 Edg/156.0.0.0',
    'accept-language': 'zh-Hans-CN,zh-CN;q=0.9,zh;q=0.8,en;q=0.7',
    cookie: cookieHeader,
    ...(pageOut.guard || {}),
  }
  const res = await fetch(u.toString(), { method: 'POST', headers, body: bodyBuf })
  const buf = Buffer.from(await res.arrayBuffer())
  return {
    status: res.status, size: buf.length, hex: buf.slice(0, 40).toString('hex'),
    printable: buf.toString('latin1').replace(/[^\x20-\x7e]/g, ' ').slice(0, 300),
    signKeys: Object.keys(pageOut.params || {}), guardKeys: Object.keys(pageOut.guard || {}),
  }
}))

// 用真实信封拼装请求：保留字段 1-7 与元数据，只替换字段 8 的 payload
function varint(n) {
  const out = []
  let v = n
  while (v > 127) { out.push((v & 0x7f) | 0x80); v = Math.floor(v / 128) }
  out.push(v)
  return Buffer.from(out)
}

function spliceField8(template, payload) {
  // 定位 tag 0x42（field 8, wire type 2）
  let i = 0
  while (i < template.length) {
    const tagStart = i
    let tag = 0, shift = 0, b
    do { b = template[i++]; tag |= (b & 0x7f) << shift; shift += 7 } while (b & 0x80)
    const field = tag >> 3, wire = tag & 7
    if (wire === 2) {
      let len = 0; shift = 0
      do { b = template[i++]; len |= (b & 0x7f) << shift; shift += 7 } while (b & 0x80)
      if (field === 8) {
        return Buffer.concat([template.slice(0, tagStart), Buffer.from([0x42]), varint(payload.length), payload, template.slice(i + len)])
      }
      i += len
    } else if (wire === 0) {
      do { b = template[i++] } while (b & 0x80)
    } else if (wire === 5) { i += 4 } else if (wire === 1) { i += 8 } else { break }
  }
  return null
}

app.post('/splice-send', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const captured = (session.capture || []).filter((c) => /get_user_message|get_conversation_list|get_message_by_init/.test(c.url))
  if (!captured.length) throw new Error('还没有可用的真实信封，请先在聊天页加载一次')
  const template = Buffer.from(captured[captured.length - 1].bodyB64, 'base64')
  const payload = Buffer.from(req.body?.payloadHex || '', 'hex')
  const body = spliceField8(template, payload)
  if (!body) throw new Error('信封里没找到字段 8')
  const client = await session.page.createCDPSession()
  let cookieHeader = ''
  try {
    const { cookies } = await client.send('Network.getAllCookies')
    cookieHeader = cookies.filter((c) => c.domain.endsWith('douyin.com')).map((c) => `${c.name}=${c.value}`).join('; ')
  } finally {
    await client.detach().catch(() => {})
  }
  const res = await fetch('https://imapi.douyin.com' + (req.body?.path || '/v1/message/send_message'), {
    method: 'POST',
    headers: {
      'content-type': 'application/x-protobuf',
      accept: 'application/x-protobuf',
      origin: 'https://www.douyin.com',
      referer: 'https://www.douyin.com/chat',
      'user-agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36',
      cookie: cookieHeader,
    },
    body,
  })
  const buf = Buffer.from(await res.arrayBuffer())
  const printable = buf.toString('latin1').replace(/[^\x20-\x7e]/g, ' ')
  return { status: res.status, size: buf.length, hex: buf.slice(0, 40).toString('hex'), printable: printable.slice(0, 300) }
}))

// 直接探测 imapi（Node 侧带 Cookie，不受 CORS 限制）
app.post('/imapi-probe', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const client = await session.page.createCDPSession()
  let cookieHeader = ''
  try {
    const { cookies } = await client.send('Network.getAllCookies')
    cookieHeader = cookies
      .filter((c) => /douyin\.com$/.test(c.domain) || c.domain.endsWith('.douyin.com'))
      .map((c) => `${c.name}=${c.value}`)
      .join('; ')
  } finally {
    await client.detach().catch(() => {})
  }
  const path = req.body?.path || '/v1/message/send'
  const headers = {
    'content-type': req.body?.contentType || 'application/x-protobuf',
    cookie: cookieHeader,
    'user-agent': req.body?.ua || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36',
    ...(req.body?.headers || {}),
  }
  const payload = req.body?.bodyB64 ? Buffer.from(req.body.bodyB64, 'base64') : (req.body?.body || '')
  const res = await fetch('https://imapi.douyin.com' + path, { method: req.body?.method || 'POST', headers, body: payload })
  const text = await res.text()
  return { status: res.status, ct: res.headers.get('content-type') || '', body: text.slice(0, 400) }
}))

// 安全抓包：劫持 fetch / WebSocket.send，只记录不发送
app.post('/capture-send-safe', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  await page.evaluateOnNewDocument(() => {
    window.__sent = []
    const origFetch = window.fetch
    window.fetch = function (input, init) {
      const url = typeof input === 'string' ? input : (input && input.url) || ''
      if (url.includes('imapi.douyin.com') && init && String(init.method || '').toUpperCase() === 'POST' && /send|create/i.test(url)) {
        try {
          const raw = init.body
          let b64 = ''
          if (raw instanceof Uint8Array) b64 = btoa(String.fromCharCode.apply(null, raw))
          else if (raw instanceof ArrayBuffer) b64 = btoa(String.fromCharCode.apply(null, new Uint8Array(raw)))
          else if (typeof raw === 'string') b64 = btoa(unescape(encodeURIComponent(raw)))
          window.__sent.push({ kind: 'fetch', url, ct: (init.headers && (init.headers['content-type'] || init.headers['Content-Type'])) || '', bodyB64: b64.slice(0, 6000) })
        } catch (e) { }
        return Promise.reject(new Error('blocked by probe'))
      }
      return origFetch.apply(this, arguments)
    }
    const origSend = WebSocket.prototype.send
    WebSocket.prototype.send = function (data) {
      try { window.__sent.push({ kind: 'ws', url: this.url, data: typeof data === 'string' ? data.slice(0, 2000) : '[binary]' }) } catch (e) { }
    }
  })
  let typed = false
  let buttons = []
  try {
    const target = req.body?.secUid ? `https://www.douyin.com/chat/${req.body.secUid}` : 'https://www.douyin.com/chat'
    await page.goto(target, { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(8000)
    const pos = await page.evaluate(() => {
      const el = document.querySelector('[data-e2e="conversation-item"]')
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
    })
    if (pos) { await page.mouse.click(pos.x, pos.y); await sleep(4000) }
    const inputPos = await page.evaluate(() => {
      const el = document.querySelector('div[contenteditable="true"], textarea')
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
    })
    if (inputPos) {
      await page.mouse.click(inputPos.x, inputPos.y)
      await page.keyboard.type(req.body?.text || 'ping', { delay: 60 })
      typed = true
      await sleep(600)
    }
    buttons = await page.evaluate(() => [...document.querySelectorAll('button, [role="button"]')]
      .map((el) => ({ text: (el.textContent || '').trim().slice(0, 8), cls: String(el.className || '').slice(0, 40) }))
      .filter((b) => /发送|send/i.test(b.text + b.cls)).slice(0, 5))
    const sendPos = await page.evaluate(() => {
      const el = [...document.querySelectorAll('button, [role="button"]')]
        .find((e) => /发送/.test((e.textContent || '').trim()) || /send/i.test(String(e.className || '')))
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
    })
    if (sendPos) { await page.mouse.click(sendPos.x, sendPos.y); await sleep(3500) }
    const captured = await page.evaluate(() => window.__sent || [])
    return { typed, hasSendButton: Boolean(sendPos), buttons, sent: captured.slice(0, 6) }
  } finally {
    await page.close().catch(() => {})
  }
}))

// 记录聊天页建立的 WebSocket（地址 + 前几帧）
app.post('/ws-record', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  const sockets = []
  page.on('websocket', (ws) => {
    const rec = { url: ws.url().slice(0, 300), sent: [], received: 0 }
    sockets.push(rec)
    ws.on('framesent', (f) => { if (rec.sent.length < 8) rec.sent.push(typeof f.payload === 'string' ? f.payload.slice(0, 300) : '[bin]') })
    ws.on('framereceived', () => { rec.received += 1 })
  })
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(Number(req.body?.seconds || 12) * 1000)
  } finally {
    await page.close().catch(() => {})
  }
  return { sockets: sockets.slice(0, 6) }
}))

// 录制一次真实发送请求（人工在 noVNC 里点发送时触发）
app.get('/captured', handle(async (req) => {
  const session = await ensureSession(idOf(req))
  return { http: (session.capture || []).slice(-20), ws: (session.wsFrames || []).slice(-20) }
}))

app.post('/record-send', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const hits = []
  const handler = (request) => {
    const url = request.url()
    if (!url.includes('imapi.douyin.com') || !/send|create/i.test(url)) return
    const post = request.postData() || ''
    hits.push({
      url: url.slice(0, 300),
      ct: request.headers()['content-type'] || '',
      method: request.method(),
      bodyB64: post ? Buffer.from(post, 'latin1').toString('base64') : '',
    })
  }
  session.page.on('request', handler)
  const seconds = Math.min(300, Number(req.body?.seconds || 120))
  try {
    await sleep(seconds * 1000)
  } finally {
    session.page.off('request', handler)
  }
  return { captured: hits.slice(0, 5), waited: seconds }
}))

// 直接探测 imapi（Node 侧带 Cookie，不受 CORS 限制）
app.post('/imapi-probe', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const client = await session.page.createCDPSession()
  let cookieHeader = ''
  try {
    const { cookies } = await client.send('Network.getAllCookies')
    cookieHeader = cookies
      .filter((c) => /douyin\.com$/.test(c.domain) || c.domain.endsWith('.douyin.com'))
      .map((c) => `${c.name}=${c.value}`)
      .join('; ')
  } finally {
    await client.detach().catch(() => {})
  }
  const path = req.body?.path || '/v1/message/send'
  const headers = {
    'content-type': req.body?.contentType || 'application/x-protobuf',
    cookie: cookieHeader,
    'user-agent': req.body?.ua || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36',
    ...(req.body?.headers || {}),
  }
  const payload = req.body?.bodyB64 ? Buffer.from(req.body.bodyB64, 'base64') : (req.body?.body || '')
  const res = await fetch('https://imapi.douyin.com' + path, { method: req.body?.method || 'POST', headers, body: payload })
  const text = await res.text()
  return { status: res.status, ct: res.headers.get('content-type') || '', body: text.slice(0, 400) }
}))

// 安全抓包：劫持 fetch / WebSocket.send，只记录不发送
app.post('/capture-send-safe', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  await page.evaluateOnNewDocument(() => {
    window.__sent = []
    const origFetch = window.fetch
    window.fetch = function (input, init) {
      const url = typeof input === 'string' ? input : (input && input.url) || ''
      if (/imapi|send|message/i.test(url)) {
        try { window.__sent.push({ kind: 'fetch', url, method: (init && init.method) || 'GET', body: init && init.body ? String(init.body).slice(0, 3000) : '', ct: (init && init.headers && (init.headers['content-type'] || init.headers['Content-Type'])) || '' }) } catch (e) { }
        return Promise.reject(new Error('blocked by probe'))
      }
      return origFetch.apply(this, arguments)
    }
    const origSend = WebSocket.prototype.send
    WebSocket.prototype.send = function (data) {
      try {
        window.__sent.push({ kind: 'ws', url: this.url, data: typeof data === 'string' ? data.slice(0, 3000) : '[binary ' + (data && data.byteLength ? data.byteLength : '?') + ' bytes]' })
      } catch (e) { }
      return undefined // 丢弃，不真正发送
    }
  })
  let typed = false
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(8000)
    const box = await page.evaluate(() => {
      const el = document.querySelector('[data-e2e="conversation-item"]')
      if (!el) return null
      const r = el.getBoundingClientRect()
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
    })
    if (box) await page.mouse.click(box.x, box.y)
    await sleep(4000)
    typed = await page.evaluate((text) => {
      const input = document.querySelector('div[contenteditable="true"], textarea')
      if (!input) return false
      input.focus()
      if (input.tagName === 'TEXTAREA') {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set
        setter.call(input, text)
        input.dispatchEvent(new Event('input', { bubbles: true }))
      } else {
        input.textContent = text
        input.dispatchEvent(new InputEvent('input', { bubbles: true, data: text }))
      }
      return true
    }, req.body?.text || 'ping')
    await sleep(800)
    const buttons = await page.evaluate(() => [...document.querySelectorAll('button, [role="button"], [data-e2e]')]
      .map((el) => ({ text: (el.textContent || '').trim().slice(0, 10), e2e: el.getAttribute('data-e2e') || '', cls: String(el.className || '').slice(0, 40) }))
      .filter((b) => /发送|send/i.test(b.text + b.e2e + b.cls))
      .slice(0, 6))
    let clicked = false
    if (buttons.length) {
      const hit = await page.evaluate(() => {
        const el = [...document.querySelectorAll('button, [role="button"], [data-e2e]')]
          .find((e) => /发送/.test((e.textContent || '').trim()) || /send/i.test(e.getAttribute('data-e2e') || ''))
        if (!el) return null
        const r = el.getBoundingClientRect()
        return { x: r.x + r.width / 2, y: r.y + r.height / 2 }
      })
      if (hit) { await page.mouse.click(hit.x, hit.y); clicked = true }
    }
    if (!clicked) await page.keyboard.press('Enter').catch(() => {})
    await sleep(4000)
    const state = await page.evaluate(() => {
      const input = document.querySelector('div[contenteditable="true"], textarea')
      return { text: input ? (input.textContent || input.value || '').slice(0, 50) : '', captured: window.__sent || [] }
    })
    return { typed, clicked, buttons, inputText: state.text, sent: state.captured.slice(0, 10) }
  } finally {
    await page.close().catch(() => {})
  }
}))

// 抓发送协议：只截获 imapi 的 send 请求并 abort，不实际发出消息
app.post('/capture-send', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  const seen = []
  const sends = []
  await page.setRequestInterception(true)
  page.on('request', (r) => {
    const u = r.url()
    if (!u.includes('imapi.douyin.com')) { r.continue(); return }
    const isSend = /send/i.test(u)
    const rec = {
      url: u.slice(0, 300), method: r.method(), isSend,
      headers: Object.fromEntries(Object.entries(r.headers()).filter(([k]) => /content-type|session|app|priority|user-agent|x-|cookie/i.test(k))),
      bodyB64: r.postData() ? Buffer.from(r.postData(), 'binary').toString('base64').slice(0, 4000) : '',
    }
    if (isSend) { sends.push(rec); r.abort(); return }
    if (seen.length < 12) seen.push(rec)
    r.continue()
  })
  let typed = false
  try {
    const target = req.body?.secUid ? `https://www.douyin.com/chat/${req.body.secUid}` : 'https://www.douyin.com/chat'
    await page.goto(target, { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(7000)
    if (!req.body?.secUid) {
      const item = await page.$('[data-e2e="conversation-item"]')
      if (item) await item.click().catch(() => {})
      await sleep(3500)
    }
    var inputsDump = await page.evaluate(() => {
      const sel = 'textarea, [contenteditable], [role="textbox"], [data-e2e*="input"]'
      return [...document.querySelectorAll(sel)].slice(0, 20).map((el) => ({
        tag: el.tagName, ce: el.getAttribute('contenteditable'), e2e: el.getAttribute('data-e2e'),
        cls: String(el.className || '').slice(0, 60), vis: el.getBoundingClientRect().width > 0,
      }))
    })
    typed = await page.evaluate((text) => {
      const input = document.querySelector('div[contenteditable="true"], textarea, [data-e2e="chat-input"]')
      if (!input) return false
      input.focus()
      if (input.tagName === 'TEXTAREA') {
        const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set
        setter.call(input, text)
        input.dispatchEvent(new Event('input', { bubbles: true }))
      } else {
        input.textContent = text
        input.dispatchEvent(new InputEvent('input', { bubbles: true, data: text }))
      }
      return true
    }, req.body?.text || 'ping')
    await sleep(600)
    if (typed) {
      await page.keyboard.press('Enter').catch(() => {})
      await sleep(4000)
    }
  } finally {
    await page.close().catch(() => {})
  }
  return { typed, inputs: inputsDump, sends: sends.slice(0, 3), others: seen.slice(0, 8) }
}))

// 好友（会话）列表：聊天页的会话数据走 WebSocket，
// 但页面会用 im/user/active/status 把全部会话的 sec_user_ids 一次性提交上来，
// 这里截获这个请求体拿到会话集合，再交给 Go 侧批量换资料。
app.post('/friends', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  let secUserIds = []
  page.on('request', (request) => {
    if (request.method() !== 'POST' || !request.url().includes('im/user/active/status')) return
    try {
      const params = new URLSearchParams(request.postData() || '')
      const list = JSON.parse(params.get('sec_user_ids') || '[]')
      if (Array.isArray(list) && list.length) secUserIds = list
    } catch { /* 忽略解析失败 */ }
  })
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(Number(req.body?.waitMs || 9000))
  } finally {
    await page.close().catch(() => {})
  }
  return { secUserIds: [...new Set(secUserIds)] }
}))

// 调试用：打开聊天页，抓取页面自己发出的 IM 请求，用于对齐接口路径
app.post('/probe', handle(async (req) => {
  const id = idOf(req)
  const session = await ensureSession(id)
  const page = await session.browser.newPage()
  const captured = []
  const calls = []
  page.on('request', (req) => {
    const url = req.url()
    if (!/\/im\/|emoji|conversation|friend|stranger/.test(url)) return
    calls.push({ url: url.slice(0, 900), method: req.method(), post: (req.postData() || '').slice(0, 400) })
  })
  page.on('response', async (res) => {
    const url = res.url()
    if (!/\/im\/|emoji|conversation|friend/.test(url)) return
    let body = ''
    try {
      const type = res.headers()['content-type'] || ''
      if (type.includes('json')) body = (await res.text()).slice(0, 1500)
    } catch { /* ignore */ }
    captured.push({ url: url.slice(0, 500), status: res.status(), body })
  })
  try {
    await page.goto('https://www.douyin.com/chat', { waitUntil: 'domcontentloaded', timeout: 45000 })
    await sleep(Number(req.body?.waitMs || 9000))
    var bundleInfo
    if (req.body?.bundles) {
      bundleInfo = await page.evaluate(async () => {
        const urls = [...new Set(performance.getEntriesByType('resource').map((e) => e.name))]
          .filter((u) => /\.(js|mjs)(\?|$)/.test(u))
        const paths = new Set()
        const ctx = []
        for (const u of urls.slice(0, 40)) {
          try {
            const txt = await (await fetch(u)).text()
            const hit = txt.match(/\/v1\/[a-z0-9_\-]+\/[a-z0-9_\-]+/gi) || []
            hit.forEach((x) => paths.add(x))
            const idx = txt.search(/send_message|sendMessage|\/message\/send/i)
            if (idx > 0) ctx.push(txt.slice(Math.max(0, idx - 120), idx + 200))
          } catch { /* ignore */ }
        }
        return { count: urls.length, paths: [...paths].slice(0, 60), ctx: ctx.slice(0, 4) }
      })
    }
    var findKeyword
    if (req.body?.findKeyword) {
      findKeyword = await page.evaluate(async (kw) => {
        const urls = [...new Set(performance.getEntriesByType('resource').map((e) => e.name))].filter((u) => /\.js(\?|$)|es5|bundle/.test(u))
        const hits = []
        for (const u of urls) {
          try {
            const t = await (await fetch(u)).text()
            if (t.includes(kw)) hits.push({ url: u, size: t.length, at: t.indexOf(kw) })
          } catch (e) { }
        }
        return hits
      }, req.body.findKeyword)
    }
    var resInfo
    if (req.body?.resources) {
      resInfo = await page.evaluate(() => [...new Set(performance.getEntriesByType('resource').map((e) => e.name))]
        .filter((u) => /sec|sdk|risk|verify|guard/i.test(u)).slice(0, 60))
    }
    var pageXhr
    if (req.body?.pageXhr) {
      pageXhr = await page.evaluate(({ url, bodyB64 }) => new Promise((resolve) => {
        try {
          const bin = atob(bodyB64 || '')
          const bytes = new Uint8Array(bin.length)
          for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
          const xhr = new XMLHttpRequest()
          xhr.open('POST', url, true)
          xhr.withCredentials = true
          xhr.responseType = 'arraybuffer'
          xhr.setRequestHeader('content-type', 'application/x-protobuf')
          xhr.setRequestHeader('accept', 'application/x-protobuf')
          xhr.timeout = 15000
          xhr.onload = () => {
            const buf = new Uint8Array(xhr.response || new ArrayBuffer(0))
            let s = ''
            for (const b of buf.slice(0, 300)) s += (b >= 32 && b < 127) ? String.fromCharCode(b) : ' '
            resolve({ ok: true, status: xhr.status, size: buf.length, printable: s.slice(0, 200) })
          }
          xhr.onerror = () => resolve({ ok: false, error: 'xhr error status=' + xhr.status })
          xhr.ontimeout = () => resolve({ ok: false, error: 'timeout' })
          xhr.send(bytes)
        } catch (e) { resolve({ ok: false, error: String(e && e.message || e) }) }
      }), { url: req.body.url, bodyB64: req.body.bodyB64 })
    }
    var pageSend
    if (req.body?.pageSend) {
      pageSend = await page.evaluate(async ({ url, bodyB64 }) => {
        const bin = atob(bodyB64 || '')
        const bytes = new Uint8Array(bin.length)
        for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
        try {
          const res = await fetch(url, {
            method: 'POST',
            credentials: 'include',
            mode: 'cors',
            headers: { 'content-type': 'application/x-protobuf', accept: 'application/x-protobuf' },
            body: bytes,
          })
          const buf = new Uint8Array(await res.arrayBuffer())
          let s = ''
          for (const b of buf) s += (b >= 32 && b < 127) ? String.fromCharCode(b) : ' '
          return { status: res.status, size: buf.length, printable: s.slice(0, 200) }
        } catch (e) {
          return { error: String(e && e.message || e) }
        }
      }, { url: req.body.url, bodyB64: req.body.bodyB64 })
    }
    var certTry
    if (req.body?.certTry) {
      certTry = await page.evaluate(async ({ url, bodyB64 }) => {
        const bin = atob(bodyB64 || '')
        const sdk = window.securitySDK
        const out = {}
        for (const name of ['generateCertHeader', 'generateCertHeaderWithAutoKey']) {
          try {
            const r = await sdk[name](url, bin)
            out[name] = { type: typeof r, keys: r && typeof r === 'object' ? Object.keys(r) : null, sample: JSON.stringify(r).slice(0, 120) }
            if (name === 'generateCertHeaderWithAutoKey' && r && r.certHeader) out.certHeaderKeys = Object.keys(r.certHeader)
          } catch (e) { out[name] = { error: String(e && e.message || e) } }
        }
        return out
      }, { url: req.body.url, bodyB64: req.body.bodyB64 })
    }
    var secureTry
    if (req.body?.secureTry) {
      secureTry = await page.evaluate(async ({ url, bodyB64 }) => {
        const bin = atob(bodyB64 || '')
        const out = {}
        try {
          const sp = window.securitySDK && window.securitySDK.secureProxy
          if (sp && typeof sp.getBdTicketGuardHeader === 'function') {
            const r = await sp.getBdTicketGuardHeader(url, bin)
            out.secure = r && r.bdTicketGuardHeaders ? r.bdTicketGuardHeaders : r
          }
          if (sp && typeof sp.ensureTrustedTimestamp === 'function') {
            await sp.ensureTrustedTimestamp().catch(() => {})
          }
          if (sp && typeof sp.getBdTicketGuardHeader === 'function') {
            const r2 = await sp.getBdTicketGuardHeader(url, bin)
            out.secureAfter = r2 && r2.bdTicketGuardHeaders ? r2.bdTicketGuardHeaders : r2
          }
        } catch (e) { out.error = String(e && e.message || e) }
        return out
      }, { url: req.body.url, bodyB64: req.body.bodyB64 })
    }
    var guardTry
    if (req.body?.guardTry) {
      guardTry = await page.evaluate(async ({ url, bodyB64 }) => {
        const sdk = window.securitySDK
        if (!sdk || typeof sdk.getBdTicketGuardHeader !== 'function') return { error: 'no api' }
        const bin = atob(bodyB64 || '')
        const out = []
        const attempt = async (label, fn) => {
          try { out.push({ label, ok: true, value: await fn() }) }
          catch (e) { out.push({ label, ok: false, error: String(e && e.message || e) }) }
        }
        await attempt('url,bin', () => sdk.getBdTicketGuardHeader(url, bin))
        await attempt('url,obj', () => sdk.getBdTicketGuardHeader(url, { data: bin }))
        await attempt('obj', () => sdk.getBdTicketGuardHeader({ url, data: bin, body: bin }))
        await attempt('urlonly', () => sdk.getBdTicketGuardHeader(url))
        return { out: out.map((o) => ({ label: o.label, ok: o.ok, keys: o.value && typeof o.value === 'object' ? Object.keys(o.value) : typeof o.value, sample: JSON.stringify(o.value).slice(0, 160), error: o.error })) }
      }, { url: req.body.url, bodyB64: req.body.bodyB64 })
    }
    var guardApi
    if (req.body?.guardApi) {
      guardApi = await page.evaluate(() => {
        const names = (obj) => {
          const out = new Set()
          let o = obj
          let depth = 0
          while (o && depth < 3) {
            for (const k of Object.getOwnPropertyNames(o)) out.add(k)
            o = Object.getPrototypeOf(o); depth++
          }
          return [...out].slice(0, 60)
        }
        const sdk = window.securitySDK
        if (!sdk) return { error: 'no securitySDK' }
        return {
          sdk: names(sdk),
          crypto: sdk.cryptoSDK ? names(sdk.cryptoSDK) : null,
          secure: sdk.secureProxy ? names(sdk.secureProxy) : null,
          config: (() => { try { return Object.keys(sdk.config || {}).slice(0, 20) } catch (e) { return null } })(),
        }
      })
    }
    var guardInfo
    if (req.body?.guard) {
      guardInfo = await page.evaluate(() => {
        const hits = []
        for (const k of Object.keys(window)) {
          if (!/ticket|guard|security|bd_/i.test(k)) continue
          let t = typeof window[k]
          let keys = []
          try { if (t === 'object' && window[k]) keys = Object.keys(window[k]).slice(0, 12) } catch (e) { }
          hits.push({ name: k, type: t, keys })
        }
        // 常见 SDK 位置
        const cands = ['bdTicketGuard', '_bd_ticket_guard', 'BDTicketGuard', '__ticketGuard', 'webSecSDK', 'bdSecurity']
        const found = {}
        for (const c of cands) { try { if (window[c]) found[c] = Object.keys(window[c]).slice(0, 12) } catch (e) { } }
        return { hits: hits.slice(0, 30), found, cookie: (document.cookie.match(/bd_ticket_guard_client_data=([^;]+)/) || [])[1] ? 'present' : 'missing' }
      })
    }
    var wsInfo
    if (req.body?.ws) {
      wsInfo = await page.evaluate(async () => {
        const urls = [...new Set(performance.getEntriesByType('resource').map((e) => e.name))]
        const cand = urls.filter((u) => /573\.|im|chat/i.test(u) && /\.js(\?|$)/.test(u)).slice(0, 6)
        const wss = new Set()
        const cmds = []
        for (const u of cand) {
          let t = ''
          try { t = await (await fetch(u)).text() } catch (e) { continue }
          for (const m of t.matchAll(/wss?:\/\/[^"'`\s)]{6,90}/g)) wss.add(m[0])
          const i = t.indexOf('SEND_MESSAGE')
          if (i > 0) cmds.push(t.slice(Math.max(0, i - 200), i + 400))
        }
        return { files: cand, wss: [...wss].slice(0, 8), cmds: cmds.slice(0, 2) }
      })
    }
    var scanInfo
    if (req.body?.scan) {
      scanInfo = await page.evaluate(async () => {
        const urls = [...new Set(performance.getEntriesByType('resource').map((e) => e.name))].filter((u) => /\.(js|mjs)(\?|$)/.test(u))
        const out = []
        const paths = new Set()
        for (const u of urls) {
          let txt = ''
          try {
            const res = await fetch(u)
            txt = await res.text()
          } catch (e) { continue }
          if (txt.length > 6e6) txt = txt.slice(0, 6e6)
          for (const m of txt.matchAll(/\/v1\/message\/[a-z0-9_]{2,40}/gi)) paths.add(m[0])
          const idx = txt.search(/send_message|SendMessage|client_msg_id|conversation_id/i)
          if (idx > 0) out.push({ url: u.slice(-60), snippet: txt.slice(Math.max(0, idx - 140), idx + 220) })
          if (out.length >= 6) break
        }
        return { scanned: urls.length, paths: [...paths], snippets: out }
      })
    }
    var openInfo
    if (req.body?.open) {
      const before = await page.evaluate(() => document.body.innerText.length)
      const box = await page.evaluate(() => {
        const el = document.querySelector('.conversationConversationItemrowArea1') || document.querySelector('[data-e2e="conversation-item"]')
        if (!el) return null
        const r = el.getBoundingClientRect()
        return { x: r.x + r.width * 0.4, y: r.y + 18 }
      })
      if (box) { await page.mouse.click(box.x, box.y); await sleep(1200); await page.mouse.click(box.x, box.y) }
      await sleep(3500)
      openInfo = await page.evaluate(() => {
        const right = document.querySelector('[class*="chat"], [class*="Chat"], [class*="conversation"]')
        const inputs = [...document.querySelectorAll('div[contenteditable="true"], textarea, [role="textbox"]')]
          .map((el) => ({ tag: el.tagName, ce: el.getAttribute('contenteditable'), cls: String(el.className || '').slice(0, 50), vis: el.getBoundingClientRect().width > 0 }))
        return { textLen: document.body.innerText.length, inputs, rightText: right ? right.textContent.slice(0, 120) : '' }
      })
      openInfo.image = (await page.screenshot({ type: 'jpeg', quality: 45, encoding: 'base64' })).slice(0, 200000)
    }
    var snapInfo
    if (req.body?.snap) {
      snapInfo = await page.evaluate(() => ({ url: location.href, title: document.title, text: (document.body.innerText || '').slice(0, 700) }))
      snapInfo.image = (await page.screenshot({ type: 'jpeg', quality: 45, encoding: 'base64' })).slice(0, 200000)
    }
    var inputDump, imapiPaths
    if (req.body?.inputs) {
      const item = await page.$('[data-e2e="conversation-item"]')
      if (item) await item.click().catch(() => {})
      await sleep(3000)
      inputDump = await page.evaluate(() => {
        const sel = 'textarea, [contenteditable], input, [role="textbox"], [data-e2e*="input"], [class*="input"]'
        return [...document.querySelectorAll(sel)].slice(0, 40).map((el) => ({
          tag: el.tagName,
          ce: el.getAttribute('contenteditable'),
          role: el.getAttribute('role'),
          e2e: el.getAttribute('data-e2e'),
          cls: String(el.className || '').slice(0, 80),
          ph: el.getAttribute('placeholder') || '',
          visible: el.getBoundingClientRect().width > 0,
        }))
      })
      imapiPaths = await page.evaluate(() => {
        const set = new Set()
        for (const e of performance.getEntriesByType('resource')) {
          if (e.name.includes('imapi.douyin.com')) set.add(new URL(e.name).pathname)
        }
        return [...set]
      })
    }
    var jsPaths
    if (req.body?.js) {
      jsPaths = await page.evaluate(async () => {
        const urls = [...document.querySelectorAll('script[src]')].map((el) => el.src).filter((u) => u.includes('douyin'))
        const found = new Set()
        for (const u of urls.slice(0, 20)) {
          try {
            const txt = await (await fetch(u)).text()
            const hit = txt.match(/\/aweme\/v1\/web\/im\/[a-zA-Z0-9_\-\/]+/g) || []
            hit.forEach((x) => found.add(x))
          } catch { /* ignore */ }
        }
        return [...found].slice(0, 200)
      })
    }
    var attrsDump
    if (req.body?.attrs) {
      attrsDump = await page.evaluate(() => {
        const out = []
        for (const el of [...document.querySelectorAll('[data-e2e="conversation-item"]')].slice(0, 4)) {
          const attrs = {}
          for (const a of el.attributes) attrs[a.name] = String(a.value).slice(0, 120)
          const imgs = [...el.querySelectorAll('img')].map((i) => (i.src || '').slice(0, 140))
          out.push({ attrs, imgs, html: el.innerHTML.slice(0, 600) })
        }
        return out
      })
    }
    if (req.body?.dom) {
      var domItems = await page.evaluate(() => {
        const rows = []
        for (const el of document.querySelectorAll('[data-e2e], a[href*="/user/"], [class*="conversation"], [class*="chat-item"]')) {
          const text = (el.textContent || '').trim().replace(/\s+/g, ' ')
          if (!text || text.length > 80) continue
          rows.push({
            tag: el.tagName,
            e2e: el.getAttribute('data-e2e') || '',
            cls: String(el.className || '').slice(0, 70),
            href: el.getAttribute('href') || '',
            text,
          })
          if (rows.length > 120) break
        }
        return rows
      })
    }
  } finally {
    await page.close().catch(() => {})
  }
  return { captured: captured.slice(0, 40), calls: calls.slice(0, 60), dom: req.body?.dom ? domItems : undefined, attrs: attrsDump, js: jsPaths, inputs: inputDump, imapi: imapiPaths, snap: snapInfo, bundles: bundleInfo, open: openInfo, scan: scanInfo, ws: wsInfo, guard: guardInfo, guardApi, guardTry, secureTry, certTry, pageSend, pageXhr, resources: resInfo, findKeyword }
}))
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
