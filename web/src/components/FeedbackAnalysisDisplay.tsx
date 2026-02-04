import { AlertTriangle, Target, TrendingDown, Eye, Loader2 } from 'lucide-react'
import { useLanguage } from '../contexts/LanguageContext'
import type { BacktestAnalysis } from '../types'

interface FeedbackAnalysisDisplayProps {
  analysis: BacktestAnalysis | null
  isLoading?: boolean
  error?: string | null
}

export function FeedbackAnalysisDisplay({
  analysis,
  isLoading = false,
  error = null,
}: FeedbackAnalysisDisplayProps) {
  const { language } = useLanguage()

  if (isLoading) {
    return (
      <div className="py-12 text-center text-sm" style={{ color: '#5E6673' }}>
        <Loader2 className="w-6 h-6 animate-spin mx-auto mb-2" />
        {language === 'zh' ? '加载分析中...' : 'Loading analysis...'}
      </div>
    )
  }

  if (error) {
    return (
      <div className="py-12 text-center text-sm" style={{ color: '#F6465D' }}>
        {error}
      </div>
    )
  }

  if (!analysis) {
    return (
      <div className="py-12 text-center" style={{ color: '#5E6673' }}>
        <AlertTriangle className="w-12 h-12 mx-auto mb-3" style={{ color: '#848E9C' }} />
        <div className="text-lg mb-2">
          {language === 'zh' ? '分析数据尚未生成' : 'Analysis Not Available Yet'}
        </div>
        <div className="text-sm">
          {language === 'zh' ? '等待交易积累后再查看' : 'Wait for more trades to accumulate'}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Failure Patterns */}
      {analysis.failure_patterns && analysis.failure_patterns.length > 0 && (
        <div>
          <h3 className="text-lg font-bold mb-3 flex items-center gap-2" style={{ color: '#F6465D' }}>
            <AlertTriangle className="w-5 h-5" />
            {language === 'zh' ? '失败模式分析' : 'Failure Patterns'}
          </h3>
          <div className="space-y-3">
            {analysis.failure_patterns.map((pattern, idx) => (
              <div
                key={idx}
                className="p-4 rounded-lg"
                style={{ background: 'rgba(246,70,93,0.1)', border: '1px solid rgba(246,70,93,0.3)' }}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="font-bold" style={{ color: '#F6465D' }}>
                    {pattern.pattern_type.replace(/_/g, ' ').toUpperCase()}
                  </div>
                  <div
                    className="text-xs px-2 py-1 rounded"
                    style={{ background: 'rgba(246,70,93,0.2)', color: '#F6465D' }}
                  >
                    {pattern.frequency} {language === 'zh' ? '次' : 'times'}
                  </div>
                </div>
                <div className="text-sm mb-2" style={{ color: '#EAECEF' }}>
                  {pattern.description}
                </div>
                <div className="grid grid-cols-2 gap-2 mb-2 text-xs">
                  <div className="p-2 rounded" style={{ background: 'rgba(246,70,93,0.15)' }}>
                    <span style={{ color: '#848E9C' }}>{language === 'zh' ? '平均损失:' : 'Avg Loss:'} </span>
                    <span className="font-mono font-bold" style={{ color: '#F6465D' }}>
                      ${pattern.avg_pnl.toFixed(2)}
                    </span>
                  </div>
                  <div className="p-2 rounded" style={{ background: 'rgba(246,70,93,0.15)' }}>
                    <span style={{ color: '#848E9C' }}>{language === 'zh' ? '损失率:' : 'Loss %:'} </span>
                    <span className="font-mono font-bold" style={{ color: '#F6465D' }}>
                      {pattern.avg_pnl_pct.toFixed(2)}%
                    </span>
                  </div>
                </div>
                {pattern.recommendation && (
                  <div
                    className="p-2 rounded text-xs"
                    style={{ background: 'rgba(240,185,11,0.1)', border: '1px solid rgba(240,185,11,0.2)', color: '#F0B90B' }}
                  >
                    💡 {pattern.recommendation}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recommended Actions */}
      {analysis.recommended_actions && analysis.recommended_actions.length > 0 && (
        <div>
          <h3 className="text-lg font-bold mb-3 flex items-center gap-2" style={{ color: '#F0B90B' }}>
            <Target className="w-5 h-5" />
            {language === 'zh' ? '建议改进措施' : 'Recommended Actions'}
          </h3>
          <div className="space-y-2">
            {analysis.recommended_actions.map((action, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg flex items-start gap-3"
                style={{ background: 'rgba(240,185,11,0.1)', border: '1px solid rgba(240,185,11,0.2)' }}
              >
                <div className="mt-0.5 font-bold" style={{ color: '#F0B90B' }}>{idx + 1}.</div>
                <div className="flex-1 text-sm" style={{ color: '#EAECEF' }}>{action}</div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Top Losing Trades */}
      {analysis.top_losing_trades && analysis.top_losing_trades.length > 0 && (
        <div>
          <h3 className="text-lg font-bold mb-3 flex items-center gap-2" style={{ color: '#F6465D' }}>
            <TrendingDown className="w-5 h-5" />
            {language === 'zh' ? '最大亏损交易' : 'Top Losing Trades'}
          </h3>
          <div className="space-y-2">
            {analysis.top_losing_trades.slice(0, 10).map((trade, idx) => (
              <div key={idx} className="p-3 rounded-lg" style={{ background: '#1E2329', border: '1px solid #2B3139' }}>
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span
                      className="text-xs px-2 py-0.5 rounded font-mono"
                      style={{ background: 'rgba(246,70,93,0.2)', color: '#F6465D' }}
                    >
                      #{idx + 1}
                    </span>
                    <span className="font-bold" style={{ color: '#EAECEF' }}>{trade.symbol}</span>
                    <span className="text-xs" style={{ color: '#848E9C' }}>
                      {new Date(trade.timestamp).toLocaleString()}
                    </span>
                  </div>
                  <div className="text-right">
                    <div className="font-mono font-bold" style={{ color: '#F6465D' }}>
                      ${trade.realized_pnl.toFixed(2)}
                    </div>
                    <div className="text-xs font-mono" style={{ color: '#F6465D' }}>
                      ({trade.realized_pnl_pct.toFixed(2)}%)
                    </div>
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-2 text-xs mb-2">
                  <div>
                    <span style={{ color: '#848E9C' }}>{language === 'zh' ? '入场:' : 'Entry:'} </span>
                    <span className="font-mono" style={{ color: '#EAECEF' }}>${trade.entry_price.toFixed(2)}</span>
                  </div>
                  <div>
                    <span style={{ color: '#848E9C' }}>{language === 'zh' ? '出场:' : 'Exit:'} </span>
                    <span className="font-mono" style={{ color: '#EAECEF' }}>${trade.exit_price.toFixed(2)}</span>
                  </div>
                  <div>
                    <span style={{ color: '#848E9C' }}>{language === 'zh' ? '持仓:' : 'Hold:'} </span>
                    <span className="font-mono" style={{ color: '#EAECEF' }}>{trade.hold_duration}</span>
                  </div>
                </div>
                {trade.analysis && (
                  <div className="text-xs p-2 rounded" style={{ background: 'rgba(246,70,93,0.1)', color: '#F6465D' }}>
                    {trade.analysis}
                  </div>
                )}
                {trade.reasoning && (
                  <div className="text-xs mt-2 p-2 rounded" style={{ background: '#0B0E11', color: '#848E9C' }}>
                    <div className="font-bold mb-1" style={{ color: '#EAECEF' }}>
                      {language === 'zh' ? '决策原因:' : 'Reasoning:'}
                    </div>
                    {trade.reasoning}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Key Insights */}
      {analysis.key_insights && analysis.key_insights.length > 0 && (
        <div>
          <h3 className="text-lg font-bold mb-3 flex items-center gap-2" style={{ color: '#0ECB81' }}>
            <Eye className="w-5 h-5" />
            {language === 'zh' ? '关键洞察' : 'Key Insights'}
          </h3>
          <div className="space-y-2">
            {analysis.key_insights.map((insight, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg flex items-start gap-3"
                style={{ background: 'rgba(14,203,129,0.1)', border: '1px solid rgba(14,203,129,0.2)' }}
              >
                <div className="text-lg">💡</div>
                <div className="flex-1 text-sm" style={{ color: '#EAECEF' }}>{insight}</div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
