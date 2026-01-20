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
  BarChart3,
  Info,
  AlertTriangle,
  Loader2,
} from 'lucide-react'
import { api } from '../lib/api'
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

interface TraderPromptVariantsResponse {
  trader_id: string
  variants: PromptVariant[]
  total: number
  generation: number
  active: PromptVariant
  timestamp: string
  message?: string
}

interface LiveTraderPromptLabProps {
  traderId?: string
}

export function LiveTraderPromptLab({ traderId }: LiveTraderPromptLabProps) {
  const { language } = useLanguage()
  const [selectedVariant, setSelectedVariant] = useState<PromptVariant | null>(null)
  const [activating, setActivating] = useState<string | null>(null)

  const { data, error, mutate } = useSWR<TraderPromptVariantsResponse>(
    traderId ? `trader-prompt-variants-${traderId}` : null,
    async () => {
      if (!traderId) return undefined
      console.log('[LiveTraderPromptLab] Fetching variants for trader:', traderId)
      const result = await api.getTraderPromptVariants(traderId)
      console.log('[LiveTraderPromptLab] Response:', result)
      return result
    },
    {
      refreshInterval: 5000, // Refresh every 5 seconds during live trading
      onError: (err) => {
        console.error('[LiveTraderPromptLab] SWR Error:', err)
      }
    }
  )

  // Add this SWR for performance data
  const { data: performance, error: perfError } = useSWR(
    traderId ? `trader-prompt-performance-${traderId}` : null,
    () => traderId ? api.getTraderPromptPerformance(traderId) : undefined,
    { refreshInterval: 10000 }
  )

  const variants = Array.isArray(data?.variants)
    ? data.variants.filter((v) => v && typeof v.VariantID === 'string')
    : []
  const activeVariant = variants.length > 0 ? variants.find((v) => v.IsActive) : undefined

  useEffect(() => {
    if (activeVariant && !selectedVariant) {
      setSelectedVariant(activeVariant)
    }
  }, [activeVariant, selectedVariant])

  const handleActivate = async (variantID: string) => {
    if (!traderId || activating) return

    setActivating(variantID)
    try {
      await api.activateTraderPromptVariant(traderId, variantID)
      mutate()
    } catch (err: any) {
      console.error('Failed to activate variant:', err)
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

  if (!traderId) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Info className="h-12 w-12 mx-auto mb-4 text-slate-500" />
          <p className="text-slate-400">
            {language === 'zh'
              ? '请选择一个交易员查看提示词优化'
              : 'Select a trader to view prompt optimization'}
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    console.error('[LiveTraderPromptLab] Error loading:', error)
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <AlertTriangle className="h-12 w-12 mx-auto mb-4 text-red-500" />
          <p className="text-red-400">
            {language === 'zh'
              ? '加载提示词优化数据失败'
              : 'Failed to load prompt optimization data'}
          </p>
          <p className="text-slate-500 text-sm mt-2">{String(error)}</p>
        </div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="flex items-center justify-center h-96">
        <Loader2 className="h-8 w-8 animate-spin text-blue-400" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Sparkles className="h-6 w-6 text-yellow-400" />
          <h2 className="text-2xl font-bold text-white">
            {language === 'zh' ? '交易员提示词实验室' : 'Trader Prompt Lab'}
          </h2>
        </div>
        <div className="text-sm text-slate-400">
          {language === 'zh' ? '代 #' : 'Gen #'}
          <span className={`ml-2 font-bold ${getGenerationColor(data.generation)}`}>
            {data.generation}
          </span>
        </div>
      </div>

      {/* Stats Summary */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-slate-800/50 rounded-lg p-4 border border-slate-700">
          <div className="text-slate-400 text-xs mb-1">
            {language === 'zh' ? '总变体数' : 'Total Variants'}
          </div>
          <div className="text-2xl font-bold text-white">{variants.length}</div>
        </div>
        <div className="bg-slate-800/50 rounded-lg p-4 border border-slate-700">
          <div className="text-slate-400 text-xs mb-1">
            {language === 'zh' ? '总代数' : 'Generations'}
          </div>
          <div className="text-2xl font-bold text-blue-400">{data.generation}</div>
        </div>
        <div className="bg-slate-800/50 rounded-lg p-4 border border-slate-700">
          <div className="text-slate-400 text-xs mb-1">
            {language === 'zh' ? '活跃变体' : 'Active Variant'}
          </div>
          <div className="text-2xl font-bold text-green-400">
            {activeVariant ? `Gen ${activeVariant.Generation}` : '-'}
          </div>
        </div>
        <div className="bg-slate-800/50 rounded-lg p-4 border border-slate-700">
          <div className="text-slate-400 text-xs mb-1">
            {language === 'zh' ? '活跃适应度' : 'Active Fitness'}
          </div>
          <div className={`text-2xl font-bold ${getFitnessColor(activeVariant?.FitnessScore ?? 0)}`}>
            {activeVariant ? (activeVariant.FitnessScore * 100).toFixed(0) : '-'}%
          </div>
        </div>
      </div>

      {/* Variants Grid */}
      <div>
        <h3 className="text-lg font-semibold text-white mb-4">
          {language === 'zh' ? '提示词变体' : 'Prompt Variants'}
        </h3>

        {variants.length === 0 ? (
          <div className="bg-slate-800/30 rounded-lg border border-dashed border-slate-700 p-8 text-center">
            <Activity className="h-12 w-12 mx-auto mb-3 text-slate-600" />
            <p className="text-slate-400">
              {language === 'zh'
                ? '等待提示词优化生成第一个变体...'
                : 'Waiting for prompt optimization to generate the first variant...'}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {variants.map((variant, idx) => (
              <motion.div
                key={variant.VariantID}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: idx * 0.05 }}
                onClick={() => setSelectedVariant(variant)}
                className={`cursor-pointer rounded-lg border transition-all ${
                  variant.IsActive
                    ? 'border-green-500/50 bg-green-950/20 ring-2 ring-green-500/30'
                    : selectedVariant?.VariantID === variant.VariantID
                    ? 'border-blue-500/50 bg-blue-950/20'
                    : 'border-slate-700 bg-slate-800/30 hover:border-slate-600'
                } p-4`}
              >
                {/* Header */}
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-2">
                    {variant.IsActive ? (
                      <CheckCircle2 className="h-5 w-5 text-green-400" />
                    ) : (
                      <Circle className="h-5 w-5 text-slate-500" />
                    )}
                    <div>
                      <div className={`font-semibold text-sm ${getGenerationColor(variant.Generation)}`}>
                        Gen {variant.Generation}
                      </div>
                      <div className="text-xs text-slate-400">
                        {new Date(variant.CreatedAt).toLocaleTimeString()}
                      </div>
                    </div>
                  </div>

                  {variant.IsActive && (
                    <div className="flex items-center gap-1 px-2 py-1 bg-green-500/20 rounded text-xs text-green-400">
                      <Activity className="h-3 w-3" />
                      {language === 'zh' ? '活跃' : 'Active'}
                    </div>
                  )}
                </div>

                {/* Performance Metrics */}
                <div className="space-y-2 mb-4 pb-4 border-b border-slate-700">
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-400">
                      {language === 'zh' ? '总决策' : 'Decisions'}
                    </span>
                    <span className="text-white font-medium">{variant.TotalDecisions}</span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-400">
                      {language === 'zh' ? '总收益率' : 'Return'}
                    </span>
                    <span className={variant.TotalReturn >= 0 ? 'text-green-400' : 'text-red-400'}>
                      {(variant.TotalReturn * 100).toFixed(2)}%
                    </span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-400">
                      {language === 'zh' ? '胜率' : 'Win Rate'}
                    </span>
                    <span className="text-white">{(variant.WinRate * 100).toFixed(1)}%</span>
                  </div>
                  <div className="flex justify-between items-center text-sm">
                    <span className="text-slate-400">
                      {language === 'zh' ? '利润因子' : 'Profit Factor'}
                    </span>
                    <span className="text-white">{variant.ProfitFactor.toFixed(2)}</span>
                  </div>
                </div>

                {/* Fitness Score */}
                <div className="flex items-center justify-between mb-4">
                  <span className="text-sm text-slate-400">
                    {language === 'zh' ? '适应度分数' : 'Fitness Score'}
                  </span>
                  <div className={`text-lg font-bold ${getFitnessColor(variant.FitnessScore)}`}>
                    {(variant.FitnessScore * 100).toFixed(1)}%
                  </div>
                </div>

                {/* Activate Button */}
                {!variant.IsActive && (
                  <button
                    onClick={() => handleActivate(variant.VariantID)}
                    disabled={activating !== null}
                    className="w-full py-2 px-3 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-700 text-white text-sm font-medium rounded transition-colors flex items-center justify-center gap-2"
                  >
                    {activating === variant.VariantID ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        {language === 'zh' ? '激活中...' : 'Activating...'}
                      </>
                    ) : (
                      <>
                        <PlayCircle className="h-4 w-4" />
                        {language === 'zh' ? '激活此变体' : 'Activate'}
                      </>
                    )}
                  </button>
                )}
              </motion.div>
            ))}
          </div>
        )}
      </div>

      {/* Selected Variant Details */}
      {selectedVariant && (
        <div className="bg-slate-800/50 rounded-lg border border-slate-700 p-6">
          <div className="flex items-center gap-2 mb-4">
            <GitBranch className="h-5 w-5 text-blue-400" />
            <h3 className="text-lg font-semibold text-white">
              {language === 'zh' ? '选中变体详情' : 'Selected Variant Details'}
            </h3>
            {selectedVariant.IsActive && (
              <span className="ml-auto text-xs px-2 py-1 bg-green-500/20 text-green-400 rounded">
                {language === 'zh' ? '活跃' : 'Active'}
              </span>
            )}
          </div>

          <div className="grid grid-cols-2 md:grid-cols-3 gap-4 mb-4">
            <div>
              <div className="text-xs text-slate-400 mb-1">
                {language === 'zh' ? '代数' : 'Generation'}
              </div>
              <div className={`text-xl font-bold ${getGenerationColor(selectedVariant.Generation)}`}>
                {selectedVariant.Generation}
              </div>
            </div>
            <div>
              <div className="text-xs text-slate-400 mb-1">
                {language === 'zh' ? '创建时间' : 'Created'}
              </div>
              <div className="text-sm text-white">
                {new Date(selectedVariant.CreatedAt).toLocaleString()}
              </div>
            </div>
            <div>
              <div className="text-xs text-slate-400 mb-1">
                {language === 'zh' ? '适应度分数' : 'Fitness'}
              </div>
              <div className={`text-xl font-bold ${getFitnessColor(selectedVariant.FitnessScore)}`}>
                {(selectedVariant.FitnessScore * 100).toFixed(1)}%
              </div>
            </div>
          </div>

          {/* Prompt Text */}
          <div>
            <div className="text-sm text-slate-400 mb-2">
              {language === 'zh' ? '系统提示词' : 'System Prompt'}
            </div>
            <div className="bg-slate-900/50 rounded p-3 border border-slate-700 max-h-48 overflow-y-auto">
              <p className="text-sm text-slate-300 whitespace-pre-wrap font-mono">
                {selectedVariant.Prompt || (language === 'zh' ? '无' : 'N/A')}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Prompt Variant Performance */}
      <div>
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <BarChart3 className="h-5 w-5 text-orange-400" />
          {language === 'zh' ? '提示词变体表现' : 'Prompt Variant Performance'}
        </h3>
        {perfError && (
          <div className="text-red-400 text-sm mb-2">
            {language === 'zh' ? '加载表现数据失败' : 'Failed to load performance data'}
          </div>
        )}
        {!performance ? (
          <div className="flex items-center gap-2 text-slate-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            {language === 'zh' ? '加载中...' : 'Loading...'}
          </div>
        ) : (
          <div className="bg-slate-800/50 rounded-lg border border-slate-700 p-4 mb-6">
            {/* Render your performance data here. Adjust fields as needed */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <div className="text-xs text-slate-400 mb-1">
                  {language === 'zh' ? '总收益率' : 'Total Return'}
                </div>
                <div className="text-xl font-bold text-green-400">
                  {(performance.total_return * 100).toFixed(2)}%
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-400 mb-1">
                  {language === 'zh' ? '胜率' : 'Win Rate'}
                </div>
                <div className="text-xl font-bold text-blue-400">
                  {(performance.win_rate * 100).toFixed(1)}%
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-400 mb-1">
                  {language === 'zh' ? '最大回撤' : 'Max Drawdown'}
                </div>
                <div className="text-xl font-bold text-red-400">
                  {(performance.max_drawdown * 100).toFixed(1)}%
                </div>
              </div>
              <div>
                <div className="text-xs text-slate-400 mb-1">
                  {language === 'zh' ? '夏普比率' : 'Sharpe Ratio'}
                </div>
                <div className="text-xl font-bold text-yellow-400">
                  {performance.sharpe_ratio?.toFixed(2)}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Last Updated */}
      <div className="text-xs text-slate-500 text-center">
        {language === 'zh' ? '最后更新' : 'Last updated'}:{' '}
        {data?.timestamp ? new Date(data.timestamp).toLocaleTimeString() : '-'}
      </div>
    </div>
  )
}