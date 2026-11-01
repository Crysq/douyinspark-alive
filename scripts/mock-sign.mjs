// 仅用于验证链路：模拟抖音会话列表与发送接口，不连接真实抖音。
import http from 'node:http'
const PORT = Number(process.env.PORT || 9100)
const conversations = [
  { user: { user_id: '880001', nickname: '橘子汽水', sec_uid: 'MS4wLjAB1', unique_id: 'juzi' } },
  { user: { user_id: '880002', nickname: '晚安电台', sec_uid: 'MS4wLjAB2', unique_id: 'wanan' } },
  { user: { user_id: '880003', nickname: '雾岛听风', sec_uid: 'MS4wLjAB3', unique_id: 'wudao' } },
]
http.createServer((req, res) => {
  let body = ''
  req.on('data', (c) => { body += c })
  req.on('end', () => {
    const json = (d) => { res.writeHead(200, { 'content-type': 'application/json' }); res.end(JSON.stringify(d)) }
    if (req.url === '/health') return json({ ok: true, ready: true })
    if (req.url === '/login/status') return json({ logged: true, cookieCount: 6 })
    if (req.url === '/cookie/import') return json({ logged: true, cookieCount: 6 })
    if (req.url === '/account/profile') return json({ uid: '900001', nickname: '同步验证号', uniqueId: 'sync_demo', secUid: 'MS4wLjAB', avatarUrl: '' })
    if (req.url === '/fetch') {
      const payload = JSON.parse(body || '{}')
      if ((payload.url || '').includes('im/conversation/list')) {
        return json({ status: 200, body: JSON.stringify({ status_code: 0, data: { conversation_list: conversations } }) })
      }
      return json({ status: 200, body: JSON.stringify({ status_code: 0, status_msg: '' }) })
    }
    json({ ok: true })
  })
}).listen(PORT, '0.0.0.0', () => console.log('mock sign on', PORT))
