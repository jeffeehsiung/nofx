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
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'

export interface PromptVariant {
  // Support both camelCase (from API) and PascalCase (for compatibility)
  id?: string
  ID?: string
  promptRoleDefinition?: string
  PromptRoleDefinition?: string
  promptTradingFrequency?: string
  PromptTradingFrequency?: string
  promptEntryStandards?: string
  PromptEntryStandards?: string
  promptDecisionProcess?: string
  PromptDecisionProcess?: string
  createdAt?: string
  CreatedAt?: string
  totalDecisions?: number
  TotalDecisions?: number
  totalReturn?: number
  TotalReturn?: number
  winRate?: number
  WinRate?: number
  profitFactor?: number
  ProfitFactor?: number
  sharpeRatio?: number
  SharpeRatio?: number
  maxDrawdown?: number
  MaxDrawdown?: number
  fitnessScore?: number
  FitnessScore?: number
  generation?: number
  Generation?: number
  isActive?: boolean
  IsActive?: boolean
}

export interface PromptVariantsResponse {
  run_id: string
  variants: PromptVariant[]
  total: number
  generation: number
  active: PromptVariant
  timestamp: string
  message?: string
  error?: string
}

interface PromptLabPageProps {
  runID?: string
  onBack?: () => void
}

export function PromptLabPage({ runID, onBack }: PromptLabPageProps) {
  const { language } = useLanguage()
  const [selectedVariant, setSelectedVariant] = useState<PromptVariant | null>(null)
  const [activating, setActivating] = useState<string | null>(null)
  const [lastNonEmptyVariants, setLastNonEmptyVariants] = useState<PromptVariant[]>([])

  // Load cached variants from sessionStorage on mount or runID change
  useEffect(() => {
    if (runID) {
      const cached = sessionStorage.getItem(`promptlab-variants-${runID}`)
      if (cached) {
        try {
          const parsed = JSON.parse(cached)
          if (Array.isArray(parsed) && parsed.length > 0) {
            setLastNonEmptyVariants(parsed)
          }
        } catch {}
      }
    }
  }, [runID])

  // Debug: Log runID to console
  if (typeof window !== 'undefined' && runID) {
    console.log('[PromptLabPage] runID:', runID)
  }

  // Robust SWR usage: always fetch when runID is set, log all key steps, type-safe
  const shouldFetch = !!runID;
  const fetchPromptVariants = async (id: string) => {
    console.log('[PromptLabPage] Fetcher called with runID:', id);
    try {
      const result = await api.getPromptVariants(id);
      console.log('[PromptLabPage] Fetch result:', result);
      
      // Check for error response
      if (result?.error) {
        throw new Error(result.error);
      }
      
      // Validate variants array
      if (!Array.isArray(result?.variants)) {
        throw new Error('Invalid response: variants is not an array');
      }
      
      return result;
    } catch (err) {
      console.error('[PromptLabPage] Fetch error:', err);
      throw err;
    }
  };

  const { data, error, mutate } = useSWR<PromptVariantsResponse>(
    shouldFetch ? `prompt-variants-${runID}` : null,
    shouldFetch
      ? () => fetchPromptVariants(runID!)
      : null,
    {
      refreshInterval: 10000,
      onError: (err) => {
        console.error('[PromptLabPage] SWR Error:', err);
      },
      revalidateOnFocus: true,
      revalidateOnMount: true,
      revalidateIfStale: true,
      revalidateOnReconnect: true,
      dedupingInterval: 0,
    }
  );

  // Save variants to sessionStorage when fetched
  useEffect(() => {
    if (data?.variants && data.variants.length > 0 && runID) {
      setLastNonEmptyVariants(data.variants)
      sessionStorage.setItem(`promptlab-variants-${runID}`, JSON.stringify(data.variants))
    }
  }, [data?.variants, runID])

  // Debug: Log loading states
  if (runID && !data && !error) {
    console.log('[PromptLabPage] Loading...')
  }

  const variants = Array.isArray(data?.variants)
    ? data.variants.filter((v) => v && (typeof v.id === 'string' || typeof v.ID === 'string'))
    : [];
  const showVariants = variants.length > 0 ? variants : lastNonEmptyVariants;
  const everHadVariants = lastNonEmptyVariants.length > 0;
  const activeVariant = showVariants.length > 0 ? showVariants.find((v) => v.isActive || v.IsActive) : undefined

  // Auto-select active variant on load
  useEffect(() => {
    if (activeVariant && !selectedVariant) {
      setSelectedVariant(activeVariant)
    }
  }, [activeVariant, selectedVariant])

  const handleActivate = async (variantId: string) => {
    if (!runID || activating) return

    setActivating(variantId)
    try {
      await api.activatePromptVariant(runID, variantId)
      mutate() // Refresh data
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

  if (!runID) {
    console.warn('[PromptLabPage] No runID provided!')
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Info className="h-12 w-12 mx-auto mb-4 text-slate-500" />
          <p className="text-slate-400">
            {language === 'zh'
              ? '请选择一个回测运行查看提示词优化'
              : 'Select a backtest run to view prompt optimization'}
          </p>
          <p className="text-slate-500 text-xs mt-2">
            (runID missing or not passed)
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    console.error('[PromptLabPage] Error loading:', error)
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <AlertTriangle className="h-12 w-12 mx-auto mb-4 text-red-400" />
          <p className="text-red-400">
            {language === 'zh' ? '加载失败' : 'Failed to load'}
          </p>
          <p className="text-slate-500 text-sm mt-2">{error.message || String(error)}</p>
          {error.message?.toLowerCase().includes('api not found') && (
            <p className="text-slate-500 text-xs mt-2">
              {language === 'zh'
                ? '后端未运行或代理端口错误。请确认 http://localhost:8080/api/backtest/prompt-variants 可用且已登录。'
                : 'Backend may be offline or proxy target is wrong. Ensure http://localhost:8080/api/backtest/prompt-variants is reachable and you are logged in.'}
            </p>
          )}
        </div>
      </div>
    )
  }

  if (!data) {
    console.log('[PromptLabPage] Waiting for data...')
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Loader2 className="h-12 w-12 mx-auto mb-4 text-blue-400 animate-spin" />
          <p className="text-slate-400">
            {language === 'zh' ? '加载中...' : 'Loading...'}
          </p>
          <p className="text-slate-500 text-xs mt-2">
            runID: {runID}
          </p>
        </div>
      </div>
    )
  }

  if (showVariants.length === 0 && !everHadVariants) {
    console.log('[PromptLabPage] No variants found (prompt optimization not enabled for this run, never had any)')
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <Sparkles className="h-12 w-12 mx-auto mb-4 text-slate-500" />
          <p className="text-slate-400">
            {data?.message ||
              (language === 'zh'
                ? '此回测未启用提示词优化'
                : 'Prompt optimization not enabled for this run')}
          </p>
          <p className="text-slate-500 text-xs mt-2">
            {language === 'zh'
              ? '请在启动回测时启用 "Enable Prompt Optimization" 来使用此功能'
              : 'Enable "Prompt Optimization" when starting a backtest to use this feature'}
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
                  {showVariants.length}
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

            {showVariants.map((variant) => (
              <motion.div
                key={(variant.id || variant.ID) || Math.random().toString(36).slice(2)}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                className={`
                  relative overflow-hidden rounded-lg border transition-all cursor-pointer
                  ${
                    (variant.isActive || variant.IsActive)
                      ? 'border-green-500/50 bg-green-500/5'
                      : (selectedVariant?.id || selectedVariant?.ID) === (variant.id || variant.ID)
                        ? 'border-blue-500/50 bg-blue-500/5'
                        : 'border-slate-700/50 bg-slate-800/50 hover:border-slate-600/50'
                  }
                `}
                onClick={() => setSelectedVariant(variant)}
              >
                {(variant.isActive || variant.IsActive) && (
                  <div className="absolute top-0 right-0 px-3 py-1 bg-green-500 text-white text-xs font-bold">
                    {language === 'zh' ? '激活中' : 'ACTIVE'}
                  </div>
                )}

                <div className="p-4">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-3">
                      {(variant.isActive || variant.IsActive) ? (
                        <CheckCircle2 className="h-5 w-5 text-green-400 flex-shrink-0" />
                      ) : (
                        <Circle className="h-5 w-5 text-slate-500 flex-shrink-0" />
                      )}
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-sm text-slate-400">
                            {((variant.id || variant.ID) || '').substring(0, 8) || '—'}
                          </span>
                          <span
                            className={`text-xs font-bold ${getGenerationColor(variant.generation || variant.Generation || 0)}`}
                          >
                            Gen {variant.generation || variant.Generation}
                          </span>
                        </div>
                        <div className="text-xs text-slate-500 mt-1">
                          {new Date(variant.createdAt || variant.CreatedAt || new Date().toISOString()).toLocaleString()}
                        </div>
                      </div>
                    </div>

                    {!(variant.isActive || variant.IsActive) && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation()
                          handleActivate(variant.id || variant.ID || '')
                        }}
                        disabled={!!activating}
                        className={`
                          flex items-center gap-1 px-3 py-1 rounded-lg text-xs font-medium
                          transition-colors
                          ${
                            activating === (variant.id || variant.ID)
                              ? 'bg-blue-500/20 text-blue-400 cursor-not-allowed'
                              : 'bg-blue-500/10 text-blue-400 hover:bg-blue-500/20'
                          }
                        `}
                      >
                        <PlayCircle className="h-3 w-3" />
                        {activating === (variant.id || variant.ID)
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
                        {variant.totalDecisions || variant.TotalDecisions || 0}
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '总收益' : 'Return'}
                      </div>
                      <div
                        className={`font-mono text-sm font-bold ${
                          (variant.totalReturn || variant.TotalReturn || 0) >= 0
                            ? 'text-green-400'
                            : 'text-red-400'
                        }`}
                      >
                        {(variant.totalReturn || variant.TotalReturn || 0) >= 0 ? '+' : ''}
                        {((variant.totalReturn || variant.TotalReturn || 0) as number).toFixed(2)}%
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '胜率' : 'Win Rate'}
                      </div>
                      <div className="font-mono text-sm font-bold text-blue-400">
                        {(variant.winRate || variant.WinRate) !== undefined ? (variant.winRate || variant.WinRate || 0)?.toFixed?.(1) : '--'}%
                      </div>
                    </div>

                    <div className="text-center">
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '适应度' : 'Fitness'}
                      </div>
                      <div
                        className={`font-mono text-sm font-bold ${getFitnessColor(variant.fitnessScore || variant.FitnessScore || 0)}`}
                      >
                        {(variant.fitnessScore || variant.FitnessScore || 0)?.toFixed?.(3)}
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
                      {selectedVariant.id || selectedVariant.ID}
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '夏普比率' : 'Sharpe Ratio'}
                      </div>
                      <div className="font-mono text-sm font-bold text-purple-400">
                        {(selectedVariant.sharpeRatio || selectedVariant.SharpeRatio || 0)?.toFixed?.(3)}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '最大回撤' : 'Max DD'}
                      </div>
                      <div className="font-mono text-sm font-bold text-red-400">
                        {(selectedVariant.maxDrawdown || selectedVariant.MaxDrawdown) !== undefined ? (selectedVariant.maxDrawdown || selectedVariant.MaxDrawdown)?.toFixed?.(2) : '--'}%
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '盈亏比' : 'Profit Factor'}
                      </div>
                      <div className="font-mono text-sm font-bold text-green-400">
                        {(selectedVariant.profitFactor || selectedVariant.ProfitFactor) !== undefined ? (selectedVariant.profitFactor || selectedVariant.ProfitFactor)?.toFixed?.(2) : '--'}
                      </div>
                    </div>

                    <div>
                      <div className="text-xs text-slate-500 mb-1">
                        {language === 'zh' ? '胜率' : 'Win Rate'}
                      </div>
                      <div className="font-mono text-sm font-bold text-blue-400">
                        {((selectedVariant.winRate || selectedVariant.WinRate) !== undefined ? (selectedVariant.winRate || selectedVariant.WinRate)?.toFixed?.(1) : '--') + '%'}
                      </div>
                    </div>
                  </div>

                  <div>
                    <div className="text-xs text-slate-500 mb-2">
                      {language === 'zh' ? '提示词内容' : 'Prompt Content'}
                    </div>
                    <div className="max-h-96 overflow-y-auto rounded bg-slate-900/50 p-3 text-xs font-mono whitespace-pre-wrap border border-slate-700/30 space-y-4">
                      <div>
                        <div className="font-bold text-blue-300 mb-1">{language === 'zh' ? '角色定义' : 'Role Definition'}</div>
                        <div>{(selectedVariant.promptRoleDefinition || selectedVariant.PromptRoleDefinition) || <span className="text-slate-500">—</span>}</div>
                      </div>
                      <div>
                        <div className="font-bold text-blue-300 mb-1">{language === 'zh' ? '交易频率' : 'Trading Frequency'}</div>
                        <div>{(selectedVariant.promptTradingFrequency || selectedVariant.PromptTradingFrequency) || <span className="text-slate-500">—</span>}</div>
                      </div>
                      <div>
                        <div className="font-bold text-blue-300 mb-1">{language === 'zh' ? '入场标准' : 'Entry Standards'}</div>
                        <div>{(selectedVariant.promptEntryStandards || selectedVariant.PromptEntryStandards) || <span className="text-slate-500">—</span>}</div>
                      </div>
                      <div>
                        <div className="font-bold text-blue-300 mb-1">{language === 'zh' ? '决策流程' : 'Decision Process'}</div>
                        <div>{(selectedVariant.promptDecisionProcess || selectedVariant.PromptDecisionProcess) || <span className="text-slate-500">—</span>}</div>
                      </div>
                    </div>
                  </div>

                  <div className="pt-3 border-t border-slate-700/50">
                    <div className="text-xs text-slate-500">
                      {language === 'zh' ? '创建时间' : 'Created At'}
                    </div>
                    <div className="text-sm mt-1">
                      {new Date(selectedVariant.createdAt || selectedVariant.CreatedAt || new Date().toISOString()).toLocaleString()}
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