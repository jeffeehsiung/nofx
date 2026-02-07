import { useState, useEffect } from 'react'
import { useLanguage } from '../contexts/LanguageContext'
import { X, Settings } from 'lucide-react'

interface TraderSettings {
  enableFeedback: boolean
  enableLLMFeedback: boolean
  enablePromptEvolution: boolean
}

interface TraderSettingsModalProps {
  isOpen: boolean
  onClose: () => void
  traderId: string
  traderName: string
  onSave?: (settings: TraderSettings) => Promise<void>
  initialSettings?: TraderSettings
}

export function TraderSettingsModal({
  isOpen,
  onClose,
  traderId,
  traderName,
  onSave,
  initialSettings = {
    enableFeedback: true,
    enableLLMFeedback: true,
    enablePromptEvolution: true,
  },
}: TraderSettingsModalProps) {
  const { language } = useLanguage()
  const [settings, setSettings] = useState<TraderSettings>(initialSettings)
  const [isSaving, setIsSaving] = useState(false)

  useEffect(() => {
    if (isOpen) {
      setSettings(initialSettings)
    }
  }, [isOpen, initialSettings])

  if (!isOpen) return null

  const handleSave = async () => {
    if (onSave) {
      setIsSaving(true)
      try {
        await onSave(settings)
      } finally {
        setIsSaving(false)
      }
    }
    onClose()
  }

  const handleToggle = (key: keyof TraderSettings) => {
    setSettings((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className="rounded-lg p-6 max-w-md w-full mx-4"
        style={{ background: '#0B0E11', border: '1px solid #2B3139' }}
      >
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Settings className="w-5 h-5" style={{ color: '#F0B90B' }} />
            <h2 className="text-lg font-bold" style={{ color: '#EAECEF' }}>
              {language === 'zh' ? '交易员设置' : 'Trader Settings'}
            </h2>
          </div>
          <button onClick={onClose} className="p-1 hover:bg-gray-800 rounded">
            <X className="w-5 h-5" style={{ color: '#848E9C' }} />
          </button>
        </div>

        <div className="mb-4">
          <p className="text-sm" style={{ color: '#848E9C' }}>
            {traderName}
          </p>
          <p className="text-xs font-mono mt-1" style={{ color: '#5E6673' }}>
            {traderId.slice(0, 16)}...
          </p>
        </div>

        <div className="space-y-4 mb-6">
          <label className="flex items-center gap-3 cursor-pointer p-3 rounded" style={{ background: '#1E2329' }}>
            <input
              type="checkbox"
              checked={settings.enableFeedback}
              onChange={() => handleToggle('enableFeedback')}
              className="accent-[#F0B90B]"
            />
            <div>
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? '启用交易失败分析' : 'Enable Trade Analysis'}
              </div>
              <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '实时分析失败交易原因'
                  : 'Analyze failed trades in real-time'}
              </div>
            </div>
          </label>

          <label className="flex items-center gap-3 cursor-pointer p-3 rounded" style={{ background: '#1E2329' }}>
            <input
              type="checkbox"
              checked={settings.enableLLMFeedback}
              onChange={() => handleToggle('enableLLMFeedback')}
              disabled={!settings.enableFeedback}
              className="accent-[#F0B90B] disabled:opacity-50"
            />
            <div>
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? '启用LLM反馈分析' : 'Enable LLM Feedback'}
              </div>
              <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '使用LLM生成更深入的交易反馈'
                  : 'Use LLMs to generate deeper feedback insights'}
              </div>
            </div>
          </label>

          <label className="flex items-center gap-3 cursor-pointer p-3 rounded" style={{ background: '#1E2329' }}>
            <input
              type="checkbox"
              checked={settings.enablePromptEvolution}
              onChange={() => handleToggle('enablePromptEvolution')}
              className="accent-[#F0B90B]"
            />
            <div>
              <div className="text-sm font-medium" style={{ color: '#EAECEF' }}>
                {language === 'zh' ? '启用提示词优化' : 'Enable Prompt Optimization'}
              </div>
              <div className="text-xs mt-0.5" style={{ color: '#848E9C' }}>
                {language === 'zh'
                  ? '实时优化AI提示词'
                  : 'Optimize AI prompts in real-time'}
              </div>
            </div>
          </label>
        </div>

        <div className="flex gap-2">
          <button
            onClick={onClose}
            className="flex-1 py-2 rounded-lg font-medium"
            style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
          >
            {language === 'zh' ? '取消' : 'Cancel'}
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="flex-1 py-2 rounded-lg font-bold disabled:opacity-50"
            style={{ background: '#F0B90B', color: '#0B0E11' }}
          >
            {isSaving ? (language === 'zh' ? '保存中...' : 'Saving...') : language === 'zh' ? '保存' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  )
}
