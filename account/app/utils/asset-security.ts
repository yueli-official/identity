import type { AssetItem } from '~/types/asset-admin'

export interface AssetSecurityPresentation {
  label: string
  description: string
  color: 'success' | 'info' | 'warning' | 'error' | 'neutral'
  icon: string
}

export function assetSecurityPresentation(asset: Pick<AssetItem, 'securityState' | 'scanStatus'>): AssetSecurityPresentation {
  if (asset.securityState === 'ready' && asset.scanStatus === 'clean') {
    return { label: '安全通过', description: '已完成恶意内容扫描和类型检查，可以交付。', color: 'success', icon: 'i-tabler-shield-check' }
  }
  if (asset.scanStatus === 'legacy_unverified' || asset.securityState === 'legacy_unverified') {
    return { label: '等待补扫', description: '这是迁移前对象，尚未生成完整安全证据。', color: 'warning', icon: 'i-tabler-history' }
  }
  if (asset.scanStatus === 'running') {
    return { label: '扫描中', description: '安全处理任务正在运行，完成前不会交付。', color: 'info', icon: 'i-tabler-loader-2' }
  }
  if (asset.scanStatus === 'pending' || asset.securityState === 'quarantined') {
    if (asset.scanStatus === 'failed') {
      return { label: '扫描失败', description: '处理发生故障，对象仍在隔离区，可人工重试。', color: 'error', icon: 'i-tabler-alert-triangle' }
    }
    return { label: '等待扫描', description: '对象位于隔离区，等待安全处理。', color: 'info', icon: 'i-tabler-hourglass' }
  }
  if (asset.scanStatus === 'malicious') {
    return { label: '发现恶意内容', description: '扫描器检出威胁，对象已拒绝且不可交付。', color: 'error', icon: 'i-tabler-shield-x' }
  }
  if (asset.scanStatus === 'admin_rejected') {
    return { label: '管理员拒绝', description: '管理员已拒绝交付，对象保留在隔离区等待后续处置。', color: 'warning', icon: 'i-tabler-user-shield' }
  }
  if (asset.scanStatus === 'policy_rejected' || asset.securityState === 'rejected') {
    return { label: '策略拒绝', description: '类型、结构或处理结果不符合安全策略。', color: 'warning', icon: 'i-tabler-file-alert' }
  }
  if (asset.scanStatus === 'failed') {
    return { label: '扫描失败', description: '处理发生故障，对象保持不可交付。', color: 'error', icon: 'i-tabler-alert-triangle' }
  }
  return { label: '状态未知', description: '没有可识别的安全状态，请检查处理记录。', color: 'neutral', icon: 'i-tabler-shield-question' }
}

export function assetScanAttemptPresentation(status: string): Pick<AssetSecurityPresentation, 'label' | 'color' | 'icon'> {
  const states: Record<string, Pick<AssetSecurityPresentation, 'label' | 'color' | 'icon'>> = {
    running: { label: '运行中', color: 'info', icon: 'i-tabler-loader-2' },
    clean: { label: '安全通过', color: 'success', icon: 'i-tabler-shield-check' },
    malicious: { label: '发现恶意内容', color: 'error', icon: 'i-tabler-shield-x' },
    policy_rejected: { label: '策略拒绝', color: 'warning', icon: 'i-tabler-file-alert' },
    admin_rejected: { label: '管理员拒绝', color: 'warning', icon: 'i-tabler-user-shield' },
    failed: { label: '处理失败', color: 'error', icon: 'i-tabler-alert-triangle' }
  }
  return states[status] ?? { label: status || '未知', color: 'neutral', icon: 'i-tabler-help' }
}
