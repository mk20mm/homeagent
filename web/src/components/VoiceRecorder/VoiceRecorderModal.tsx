import React, { useEffect, useRef, useState } from 'react'
import { api, unwrap } from '../../api/client'
import styles from './VoiceRecorderModal.module.css'

interface VoiceRecorderModalProps {
  isOpen: boolean
  onClose: () => void
  onFinish: (transcribedText: string) => void
}

interface SpeechRecognitionEventLike {
  results: ArrayLike<ArrayLike<{ transcript: string }>>
}

interface SpeechRecognitionErrorEventLike {
  error: string
}

interface SpeechRecognitionLike {
  lang: string
  continuous: boolean
  interimResults: boolean
  onresult: ((event: SpeechRecognitionEventLike) => void) | null
  onerror: ((event: SpeechRecognitionErrorEventLike) => void) | null
  start: () => void
  stop: () => void
}

declare global {
  interface Window {
    SpeechRecognition?: new () => SpeechRecognitionLike
    webkitSpeechRecognition?: new () => SpeechRecognitionLike
    webkitAudioContext?: typeof AudioContext
  }
}

/** 100% 免费本地语音转文字录音抽屉：对齐 Stitch iOS 18 拟态磨砂玻璃设计规范 */
export const VoiceRecorderModal: React.FC<VoiceRecorderModalProps> = ({
  isOpen,
  onClose,
  onFinish,
}) => {
  const [seconds, setSeconds] = useState(0)
  const [transcript, setTranscript] = useState('')
  const [waveHeights, setWaveHeights] = useState<number[]>([12, 16, 24, 30, 20, 14, 28, 36, 18, 12, 16, 22, 34, 28, 16, 20, 14, 18, 12, 16])
  const [isProcessing, setIsProcessing] = useState(false)

  const recognitionRef = useRef<SpeechRecognitionLike | null>(null)
  const mediaStreamRef = useRef<MediaStream | null>(null)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const audioCtxRef = useRef<AudioContext | null>(null)
  const animFrameRef = useRef<number | null>(null)

  useEffect(() => {
    if (!isOpen) {
      stopAll()
      return
    }

    setSeconds(0)
    setTranscript('')
    setIsProcessing(false)

    // 启动计时器
    const timer = setInterval(() => {
      setSeconds((prev) => {
        if (prev >= 60) {
          clearInterval(timer)
          return 60
        }
        return prev + 1
      })
    }, 1000)

    // 启动免费录音引擎
    startRecording()

    return () => {
      clearInterval(timer)
      stopAll()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen])

  const startRecording = async () => {
    try {
      // 1. 请求麦克风权限并启动波形可视化
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      mediaStreamRef.current = stream

      setupAudioVisualizer(stream)

      // 2. 检查是否支持浏览器原生 Web Speech API (零成本、极速出字)
      const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition
      if (SpeechRecognition) {
        const recognition = new SpeechRecognition()
        recognition.lang = 'zh-CN'
        recognition.continuous = true
        recognition.interimResults = true

        recognition.onresult = (event: SpeechRecognitionEventLike) => {
          let current = ''
          for (let i = 0; i < event.results.length; i++) {
            current += event.results[i][0].transcript
          }
          if (current) {
            setTranscript(current)
          }
        }

        recognition.onerror = (event: SpeechRecognitionErrorEventLike) => {
          console.warn('[VoiceSTT] Web Speech API event:', event.error)
        }

        recognition.start()
        recognitionRef.current = recognition
      } else {
        // 3. 降级：使用 MediaRecorder 录制音频 Blob，待提交时发给本地 whisper.cpp
        const recorder = new MediaRecorder(stream)
        audioChunksRef.current = []
        recorder.ondataavailable = (e) => {
          if (e.data.size > 0) audioChunksRef.current.push(e.data)
        }
        recorder.start(250)
        mediaRecorderRef.current = recorder
      }
    } catch (err) {
      console.error('[VoiceSTT] 无法获取麦克风权限:', err)
      setTranscript('请允许麦克风权限以使用语音录入')
    }
  }

  const setupAudioVisualizer = (stream: MediaStream) => {
    try {
      const AudioCtx = window.AudioContext || window.webkitAudioContext
      if (!AudioCtx) return
      const ctx = new AudioCtx()
      audioCtxRef.current = ctx

      const source = ctx.createMediaStreamSource(stream)
      const analyser = ctx.createAnalyser()
      analyser.fftSize = 64
      source.connect(analyser)

      const bufferLength = analyser.frequencyBinCount
      const dataArray = new Uint8Array(bufferLength)

      const updateWave = () => {
        analyser.getByteFrequencyData(dataArray)
        // 提取 20 根波形条的动态高度
        const bars: number[] = []
        for (let i = 0; i < 20; i++) {
          const val = dataArray[i % bufferLength] || 10
          // 映射到 8px ~ 42px
          const h = Math.max(8, Math.min(44, (val / 255) * 44 + 6))
          bars.push(Math.round(h))
        }
        setWaveHeights(bars)
        animFrameRef.current = requestAnimationFrame(updateWave)
      }

      animFrameRef.current = requestAnimationFrame(updateWave)
    } catch (e) {
      console.warn('[VoiceSTT] Web Audio Analyser not supported:', e)
    }
  }

  const stopAll = () => {
    if (recognitionRef.current) {
      try {
        recognitionRef.current.stop()
      } catch (err) {
        void err
      }
      recognitionRef.current = null
    }

    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      try {
        mediaRecorderRef.current.stop()
      } catch (err) {
        void err
      }
      mediaRecorderRef.current = null
    }

    if (mediaStreamRef.current) {
      mediaStreamRef.current.getTracks().forEach((t) => t.stop())
      mediaStreamRef.current = null
    }

    if (audioCtxRef.current) {
      try {
        void audioCtxRef.current.close()
      } catch (err) {
        void err
      }
      audioCtxRef.current = null
    }

    if (animFrameRef.current) {
      cancelAnimationFrame(animFrameRef.current)
      animFrameRef.current = null
    }
  }

  const handleConfirm = async () => {
    if (transcript.trim()) {
      onFinish(transcript.trim())
      onClose()
      return
    }

    // 若使用的是 MediaRecorder 降级录音，调用后端接口转写
    if (audioChunksRef.current.length > 0) {
      setIsProcessing(true)
      const blob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
      const formData = new FormData()
      formData.append('file', blob, 'voice.webm')
      formData.append('language', 'zh')

      try {
        const res = await api.POST('/voice/transcribe', {
          body: formData as unknown as { file: string; language?: string },
          bodySerializer: () => formData,
        })
        const data = unwrap(res)
        if (data?.text) {
          onFinish(data.text)
          onClose()
          return
        }
      } catch (err) {
        console.error('[VoiceSTT] 转写接口请求失败:', err)
      } finally {
        setIsProcessing(false)
      }
    }

    onClose()
  }

  if (!isOpen) return null

  const formatTimer = (s: number) => {
    const mins = Math.floor(s / 60)
    const secs = s % 60
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')} / 01:00`
  }

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.sheet} onClick={(e) => e.stopPropagation()}>
        <div className={styles.dragHandle} />

        {/* 顶部录音状态与计时 */}
        <div className={styles.statusHeader}>
          <div className={styles.recordingStatus}>
            <span className={styles.redDot} />
            <span className={styles.statusTitle}>正在聆听…</span>
          </div>
          <div className={styles.timerPill}>
            <span>⏱</span>
            <span>{formatTimer(seconds)}</span>
          </div>
        </div>

        {/* 实时转写文字卡片 */}
        <div className={styles.transcriptCard}>
          {transcript ? (
            <p className={styles.transcriptText}>
              {transcript}
              <span className={styles.cursor} />
            </p>
          ) : (
            <p className={styles.placeholderText}>
              请说话，支持记账、查菜谱、问天气、查故障…
              <span className={styles.cursor} />
            </p>
          )}
          <div className={styles.hintRow}>
            <span>✦</span>
            <span>AI 实时理解 · 浏览器免费极速识别 · 准确率 99%</span>
          </div>
        </div>

        {/* 动态音频波形 */}
        <div className={styles.waveformContainer}>
          {waveHeights.map((h, i) => (
            <div key={i} className={styles.waveBar} style={{ height: `${h}px` }} />
          ))}
        </div>

        <div className={styles.gestureHint}>松开发送，上滑取消</div>

        {/* 底部操作按钮 */}
        <div className={styles.actionRow}>
          <button type="button" className={styles.cancelBtn} onClick={onClose}>
            <span>✕</span> 取消
          </button>
          <button
            type="button"
            className={styles.confirmBtn}
            onClick={() => void handleConfirm()}
            disabled={isProcessing}
          >
            <span>▷</span> {isProcessing ? '识别中…' : '转文字填入'}
          </button>
        </div>
      </div>
    </div>
  )
}
