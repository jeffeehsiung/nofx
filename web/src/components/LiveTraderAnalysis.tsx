import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { FeedbackAnalysisDisplay } from './FeedbackAnalysisDisplay'
import type { BacktestAnalysis } from '../types'

interface LiveTraderAnalysisProps {
  traderId: string
}

export function LiveTraderAnalysis({ traderId }: LiveTraderAnalysisProps) {
  const { language } = useLanguage()

  const { data, error, isLoading } = useSWR(
    traderId ? `trader-analysis-${traderId}` : null,
    async () => await api.getTraderAnalysis(traderId),
    { refreshInterval: 5000 }
  )

  const analysis: BacktestAnalysis | undefined = data // API returns BacktestAnalysis directly

  return (
    <FeedbackAnalysisDisplay
      analysis={analysis || null}
      isLoading={isLoading}
      error={error ? `${language === 'zh' ? '获取分析失败' : 'Failed to fetch analysis'}` : null}
    />
  )
}
