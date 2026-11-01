import puppeteer from 'puppeteer-core'

const BASE = process.env.BASE || 'http://127.0.0.1:8787'
const TOKEN = process.env.TOKEN
const OUT = process.env.OUT || '/shots'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const browser = await puppeteer.launch({
  executablePath: '/usr/bin/chromium',
  headless: 'new',
  args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--font-render-hinting=none'],
})

async function shot(route, name, viewport = { width: 1440, height: 980, deviceScaleFactor: 2 }, authed = true) {
  const page = await browser.newPage()
  await page.setViewport(viewport)
  await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' })
  if (authed) {
    await page.evaluate((t) => localStorage.setItem('doyinspark.token', t), TOKEN)
  }
  await page.goto(`${BASE}/${route}`, { waitUntil: 'domcontentloaded' })
  await sleep(1800)
  await page.screenshot({ path: `${OUT}/${name}.png` })
  await page.close()
}

await shot('login', '01-login', undefined, false)
await shot('register', '10-register', undefined, false)
await shot('dashboard', '02-dashboard')
await shot('friends', '03-friends')
await shot('accounts', '04-accounts')
await shot('tasks', '05-tasks')
await shot('templates', '06-templates')
await shot('records', '07-records')
await shot('settings', '08-settings')
await shot('membership', '11-membership')
await shot('admin/users', '12-admin-users')
await shot('admin/cards', '13-admin-cards')
await shot('dashboard', '09-mobile', { width: 390, height: 844, deviceScaleFactor: 2, isMobile: true })

await browser.close()
console.log('shots done')
