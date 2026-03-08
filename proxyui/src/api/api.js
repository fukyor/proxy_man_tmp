export async function fetchConfig(apiUrl) {
  const resp = await fetch(`${apiUrl}/api/config`)
  if (!resp.ok) throw new Error('获取配置失败')
  return resp.json()
}

export async function updateConfig(apiUrl, config) {
  const resp = await fetch(`${apiUrl}/api/config`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config)
  })
  if (!resp.ok) throw new Error('更新配置失败')
  return resp.json()
}
