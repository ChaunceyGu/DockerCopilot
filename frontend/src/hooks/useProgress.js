import { useState, useEffect, useRef } from 'react'
import { progressAPI } from '../api/client.js'

/**
 * 进度查询Hook
 * @param {string} taskId - 任务ID
 * @param {function} onComplete - 完成回调
 * @param {function} onError - 错误回调
 */
export function useProgress(taskId, onComplete, onError) {
  const [progress, setProgress] = useState(null)
  const [isPolling, setIsPolling] = useState(false)
  const intervalRef = useRef(null)

  useEffect(() => {
    if (!taskId || isPolling) return

    setIsPolling(true)

    const pollProgress = async () => {
      try {
        const response = await progressAPI.getProgress(taskId)
        const data = response.data

        setProgress(data)

        // 后端 data.data.isDone 标记任务结束，isError 区分失败/成功
        const isDone = data.data?.isDone === true
        const isError = data.data?.isError === true || data.code >= 400

        if (isDone && !isError) {
          stopPolling()
          onComplete?.(data)
        } else if (isDone && isError) {
          stopPolling()
          onError?.(data)
        }
      } catch (error) {
        console.error('查询进度失败:', error)
        stopPolling()
        onError?.(error)
      }
    }

    const stopPolling = () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      setIsPolling(false)
    }

    // 立即执行一次
    pollProgress()

    // 每2秒查询一次进度
    intervalRef.current = setInterval(pollProgress, 2000)

    // 清理函数
    return () => {
      stopPolling()
    }
  }, [taskId, isPolling, onComplete, onError])

  return { progress, isPolling }
}
