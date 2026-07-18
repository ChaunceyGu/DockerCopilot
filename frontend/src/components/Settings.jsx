import React, { useState, useEffect } from 'react'
import { Server, Save, Loader2, CheckCircle2, AlertCircle, Zap } from 'lucide-react'
import { settingsAPI } from '../api/client.js'

// 常用国内镜像加速器，方便用户一键填入
const PRESET_MIRRORS = [
  { label: '1Panel', url: 'https://docker.1panel.live' },
  { label: 'DaoCloud', url: 'https://docker.m.daocloud.io' },
  { label: '毫秒镜像', url: 'https://docker.1ms.run' },
  { label: 'AtomHub', url: 'https://hub.atomgit.com' },
]

export function Settings() {
  const [registryMirror, setRegistryMirror] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  // status: { type: 'success' | 'error', msg: string } | null
  const [status, setStatus] = useState(null)

  useEffect(() => {
    let mounted = true
    const load = async () => {
      try {
        const res = await settingsAPI.getSettings()
        if (mounted && (res.data.code === 200 || res.data.code === 0)) {
          setRegistryMirror(res.data.data?.registryMirror || '')
        }
      } catch (err) {
        if (mounted) {
          setStatus({ type: 'error', msg: '加载设置失败: ' + (err.response?.data?.msg || err.message) })
        }
      } finally {
        if (mounted) setIsLoading(false)
      }
    }
    load()
    return () => { mounted = false }
  }, [])

  const handleSave = async () => {
    setIsSaving(true)
    setStatus(null)
    try {
      const res = await settingsAPI.saveSettings({ registryMirror: registryMirror.trim() })
      if (res.data.code === 200 || res.data.code === 0) {
        setRegistryMirror(res.data.data?.registryMirror || '')
        setStatus({ type: 'success', msg: '设置已保存，下次拉取镜像时生效' })
      } else {
        setStatus({ type: 'error', msg: res.data.msg || '保存失败' })
      }
    } catch (err) {
      setStatus({ type: 'error', msg: '保存失败: ' + (err.response?.data?.msg || err.message) })
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="max-w-7xl mx-auto">
      <div className="px-2 sm:px-6 py-4 pt-4 sm:pt-4 space-y-6">
        {/* 标题 */}
        <div className="flex items-center gap-3">
          <div className="p-2 bg-primary-100 dark:bg-primary-900/30 rounded-lg">
            <Zap className="h-6 w-6 text-primary-600 dark:text-primary-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">设置</h1>
            <p className="text-sm text-gray-500 dark:text-gray-400">配置镜像加速器等运行时选项</p>
          </div>
        </div>

        {/* 镜像加速器卡片 */}
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-2">
            <Server className="h-5 w-5 text-primary-600 dark:text-primary-400" />
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white">镜像加速器 / 代理</h2>
          </div>
          <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            国内网络拉取 Docker Hub 镜像常常超时。填入镜像加速器地址后，更新和恢复容器时会自动改写
            <code className="mx-1 px-1.5 py-0.5 bg-gray-100 dark:bg-gray-800 rounded text-xs">docker.io</code>
            镜像走加速器拉取，拉取完成后自动还原为原始镜像名，不影响容器配置。留空则直连官方源。
          </p>

          {isLoading ? (
            <div className="flex items-center gap-2 text-gray-500 dark:text-gray-400 py-4">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span className="text-sm">加载中...</span>
            </div>
          ) : (
            <>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                加速器地址
              </label>
              <input
                type="text"
                value={registryMirror}
                onChange={(e) => setRegistryMirror(e.target.value)}
                placeholder="https://docker.1panel.live （留空则直连 Docker Hub）"
                className="input w-full"
                spellCheck={false}
                autoComplete="off"
              />

              {/* 预设快捷选择 */}
              <div className="flex flex-wrap items-center gap-2 mt-3">
                <span className="text-xs text-gray-400 dark:text-gray-500">常用：</span>
                {PRESET_MIRRORS.map((m) => (
                  <button
                    key={m.url}
                    type="button"
                    onClick={() => setRegistryMirror(m.url)}
                    className="px-2.5 py-1 text-xs rounded-full border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:border-primary-300 hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
                  >
                    {m.label}
                  </button>
                ))}
                <button
                  type="button"
                  onClick={() => setRegistryMirror('')}
                  className="px-2.5 py-1 text-xs rounded-full border border-gray-200 dark:border-gray-700 text-gray-500 hover:border-red-300 hover:text-red-500 transition-colors"
                >
                  清空
                </button>
              </div>

              {/* 状态提示 */}
              {status && (
                <div className={
                  'flex items-start gap-2 mt-4 p-3 rounded-lg text-sm ' +
                  (status.type === 'success'
                    ? 'bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-400'
                    : 'bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-400')
                }>
                  {status.type === 'success'
                    ? <CheckCircle2 className="h-4 w-4 mt-0.5 flex-shrink-0" />
                    : <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />}
                  <span className="whitespace-pre-wrap">{status.msg}</span>
                </div>
              )}

              <div className="flex justify-end mt-5">
                <button
                  onClick={handleSave}
                  disabled={isSaving}
                  className="flex items-center gap-2 px-4 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-60 disabled:cursor-not-allowed text-white rounded-lg transition-colors shadow-sm"
                >
                  {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                  <span>{isSaving ? '保存中...' : '保存设置'}</span>
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
