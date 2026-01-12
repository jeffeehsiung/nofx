import { useEffect, useState } from 'react'
import useSWR from 'swr'
import { motion } from 'framer-motion'
import {
  Sparkles,
  Activity,
  CheckCircle2,
  Circle,
  PlayCircle,
  GitBranch,
  Award,
  Info,
  AlertTriangle,
  Loader2,
  ChevronLeft,
} from 'lucide-react'
import { httpClient } from '../lib/httpClient'
import { useLanguage } from '../contexts/LanguageContext'

interface PromptVariant {
  ID: string
  VariantID: string
  Generation: number
  IsActive: boolean
  Prompt: string
  CreatedAt: string
  TotalDecisions: number
  TotalReturn: number
  WinRate: number
  ProfitFactor: number
  SharpeRatio: number
  MaxDrawdown: number
  FitnessScore: number
}

interface PromptVariantsResponse {
  run_id: string
  variants: PromptVariant[]
  total: number
  generation: number
  active: PromptVariant
  timestamp: string
  message?: string
}

interface PromptLabPageProps {
  runID?: string
  onBack?: () => void
}

export function PromptLabPage({ runID, onBack }: PromptLabPageProps) {
  const { language } = useLanguage()
  const [selectedVariant, setSelectedVariant] = useState<PromptVariant | null>(null)
  const [activating, setActivating] = useState<string | null>(null)

  const { data, error, mutate } = useSWR<PromptVariantsResponse>(
    runID ? `/api/backtest/prompt-variants?run_id=${runID}` : null,
    async (url) => {
      const result = await httpClient.get<PromptVariantsResponse>(url)
      if (!result.success) throw new Error('Failed to load variants')
      return result.data!
    },
    {
      refreshInterval: 5000, // Refresh every 5 seconds during backtest
    }
  )

  const variants = data?.variants || []
  const activeVariant = variants.find((v) => v.IsActive)

  // Auto-select active variant on load
  useEffect(() => {
    if (activeVariant && !selectedVariant) {
      setSelectedVariant(activeVariant)
    }
  }, [activeVariant, selectedVariant])

  const handleActivate = async (variantID: string) => {
    if (!runID || activating) return

    setActivating(variantID)
    try {
      const result = await httpClient.post('/api/backtest/prompt-activate', {
        run_id: runID,
        variant_id: variantID,
      })
      if (!result.success) throw new Error(result.message || 'Failed to activate variant')
      mutate() // Refresh data
    } catch (err: any) {
    } finally {
      setActivating(null)
    }
  }

  const getGenerationColor = (generation: number) => {
    const colors = [
      'text-blue-400',
      'text-purple-400',
      'text-pink-400',
      'text-orange-400',
      'text-green-400',
    ]
    return colors[generation % colors.length]
  }

  const getFitnessColor = (score: number) => {
    if (score >= 0.8) return 'text-green-400'
    if (score >= 0.5) return 'text-yellow-400'
    return 'text-red-400'
  }

  if (!runID) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Info className="h-12 w-12 mx-auto mb-4 text-slate-500" />
          <p className="text-slate-400">
            {language === 'zh'
              ? '请选择一个回测运行查看提示词优化'
              : 'Select a backtest run to view prompt optimization'}
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <AlertTriangle className="h-12 w-12 mx-auto mb-4 text-red-400" />
          <p className="text-red-400">
            {language === 'zh' ? '加载失败' : 'Failed to load'}
          </p>
          <p className="text-slate-500 text-sm mt-2">{error.message}</p>
        </div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Loader2 className="h-12 w-12 mx-auto mb-4 text-blue-400 animate-spin" />
          <p className="text-slate-400">
            {language === 'zh' ? '加载中...' : 'Loading...'}
          </p>
        </div>
      </div>
    )
  }

  if (variants.length === 0) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Sparkles className="h-12 w-12 mx-auto mb-4 text-slate-500" />
          <p className="text-slate-400">
            {data.message ||
              (language === 'zh'
                ? '此回测未启用提示词优化'
                : 'Prompt optimization not enabled for this run')}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
      {/* Header */}
      <div className="border-b border-slate-700/50 bg-slate-900/50 backdrop-blur-sm sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {onBack && (
                <button
                  onClick={onBack}
                  className="p-2 hover:bg-slate-700/50 rounded-lg transition-colors"
                >
                  <ChevronLeft className="h-5 w-5" />
                </button>
              )}
              <Sparkles className="h-6 w-6 text-purple-400" />
              <h1 className="text-xl font-bold">
                {language === 'zh' ? '提示词实验室' : 'Prompt Lab'}
              </h1>
            </div>
            <div className="flex items-center gap-4 text-sm">
              <div className="flex items-center gap-2">
                <GitBranch className="h-4 w-4 text-slate-400" />
                <span className="text-slate-400">
                  {language === 'zh' ? '代数' : 'Generation'}:
                </span>
                <span className="font-mono font-bold text-purple-400">
                  {data.generation}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <Activity className="h-4 w-4 text-slate-400" />
                <span className="text-slate-400">
                  {language === 'zh' ? '变体' : 'Variants'}:
                </span>
                <span className="font-mono font-bold text-blue-400">
                  {variants.length}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="max-w-7xl mx-auto px-4 py-6">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Variants List */}
          <div className="lg:col-span-2 space-y-3">
            <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
              <Award className="h-5 w-5 text-yellow-400" />
              {language === 'zh' ? '提示词变体' : 'Prompt Variants'}
            </h2>

            {variants.map((variant) => (
              <motion.div
                key={variant.VariantID}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                className={`
                  relative overflow-hidden rounded-lg border transition-all cursor-pointer
                  ${
                    variant.IsActive
                      ? 'border-green-500/50 bg-green-500/5'
                      : selectedVariant?.VariantID === variant.VariantID
                        ? 'border-blue-500/50 bg-blue-500/5'
                        : 'border-slate-700/50 bg-slate-800/50 hover:border-slate-600/50'
                  }
                `}
                onClick={() => setSelectedVariant(variant)}
              >
                {variant.IsActive && (
                  <div className="absolute top-0 right-0 px-3 py-1 bg-green-500 text-white text-xs font-bold">
                    {language === 'zh' ? '激活中' : 'ACTIVE'}
                  </div>
                )}

                <div className="p-4">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-3">
                      {variant.IsActive ? (
                        <CheckCircle2 className="h-5 w-5 text-green-400 flex-shrink-0" />
                      ) : (
                        <Circle className="h-5 w-5 text-slate-500 flex-shrink-0" />
                      )}
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-sm text-slate-400">
                            {variant.VariantID.substring(0, 8)}
                          </span>
                          <span
                            className={`text-xs font-bold ${getGenerationColor(variant.Generation)}`}
                          >
                            Gen {variant.Generation}
                          </span>
                        </div>
                        <div className="text-xs text-slate-500 mt-1">
                          {new Date(variant.CreatedAt).toLocaleString()}
                        </div>
                      </div>
                    </div>

                    {!variant.IsActive && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          handleActivate(variant.VariantID)
                        }}
                        disabled={!!activating}
                        className={`
                          flex items-center gap-1 px-3 py-1 rounded-lg text-xs font-medium
                          transition-colors
                          ${
                            activating === variant.VariantID
                              ? 'bg-blue-500/20 text-blue-400 cursor-not-allowed'
                              : 'bg-blue-500/10 text-blue-400 hover:bg-blue-500/20'
                          }
                        `}
                      >
                        <PlayCircle className="h-3 w-3" />
                        {activating === variant.VariantID
                          ? language === 'zh'
                            ? '激活中...'
                            : 'Activating...'
                          : language === 'zh'
                            ? '激活'
                            : 'Activate'}
                      </button>
                    )}
                  </div>

                  {/* Performance Metrics */}
                  <div className="grid grid-cols-4 gap-3">
                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '决策' : 'Decisions'}
                      </div>
                      <div className="font-mono text-sm font-bold">
                        {variant.TotalDecisions}
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '总收益' : 'Return'}
                      </div>
                      <div
                        className={`font-mono text-sm font-bold ${
                          variant.TotalReturn >= 0
                            ? 'text-green-400'
                            : 'text-red-400'
                        }`}
                      >
                        {variant.TotalReturn >= 0 ? '+' : ''}
                        {variant.TotalReturn.toFixed(2)}%
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '胜率' : 'Win Rate'}
                      </div>
                      <div className="font-mono text-sm font-bold text-blue-400">
                        {(variant.WinRate * 100).toFixed(1)}%
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '适应度' : 'Fitness'}
                      </div>
                      <div
                        className={`font-mono text-sm font-bold ${getFitnessColor(variant.FitnessScore)}`}
                      >
                        {variant.FitnessScore.toFixed(3)}
                      </div>
                    </div>
                  </div>
                </div>
              </motion.div>
            ))}
          </div>

          {/* Details Panel */}
          <div className="lg:col-span-1">
            <div className="sticky top-24">
              <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
                <Info className="h-5 w-5 text-blue-400" />
                {language === 'zh' ? '详细信息' : 'Details'}
              </h2>

              {selectedVariant ? (
                <div className="rounded-lg border border-slate-700/50 bg-slate-800/50 p-4 space-y-4">
                  <div>
                    <div className="text-xs text-slate-500 mb-1">
                      {language === 'zh' ? '变体 ID' : 'Variant ID'}
                    </div>
                    <div className="font-mono text-sm break-all">
                      {selectedVariant.VariantID}
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '夏普比率' : 'Sharpe Ratio'}
                      </div>
                      <div className="font-mono text-sm font-bold text-purple-400">
                        {selectedVariant.SharpeRatio.toFixed(3)}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '最大回撤' : 'Max DD'}
                      </div>
                      <div className="font-mono text-sm font-bold text-red-400">
                        {(selectedVariant.MaxDrawdown * 100).toFixed(2)}%
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '盈亏比' : 'Profit Factor'}
                      </div>
                      <div className="font-mono text-sm font-bold text-green-400">
                        {selectedVariant.ProfitFactor.toFixed(2)}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '胜率' : 'Win Rate'}
                      </div>
                      <div className="font-mono text-sm font-bold text-blue-400">
                        {(selectedVariant.WinRate * 100).toFixed(1)}%
                      </div>
                    </div>
                  </div>

                  <div>
                    <div className="text-xs text-slate-500 mb-2">
                      {language === 'zh' ? '提示词内容' : 'Prompt Content'}
                    </div>
                    <div className="max-h-96 overflow-y-auto rounded bg-slate-900/50 p-3 text-xs font-mono whitespace-pre-wrap border border-slate-700/30">
                      {selectedVariant.Prompt || (
                        <span className="text-slate-500">
                          {language === 'zh' ? '默认提示词' : 'Default prompt'}
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="pt-3 border-t border-slate-700/50">
                    <div className="text-xs text-slate-500">
                      {language === 'zh' ? '创建时间' : 'Created At'}
                    </div>
                    <div className="text-sm mt-1">
                      {new Date(selectedVariant.CreatedAt).toLocaleString()}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="rounded-lg border border-slate-700/50 bg-slate-800/50 p-8 text-center">
                  <Sparkles className="h-12 w-12 mx-auto mb-3 text-slate-600" />
                  <p className="text-slate-500 text-sm">
                    {language === 'zh'
                      ? '选择一个变体查看详情'
                      : 'Select a variant to view details'}
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
