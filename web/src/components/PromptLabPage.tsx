import { PromptVariantLab } from './PromptVariantLab'

// Re-export types for backward compatibility
export type { PromptVariant, PromptVariantLabResponse as PromptVariantsResponse } from './PromptVariantLab'

interface PromptLabPageProps {
  runID?: string
  onBack?: () => void
}

export function PromptLabPage({ runID, onBack }: PromptLabPageProps) {
  return <PromptVariantLab type="backtest" resourceId={runID} onBack={onBack} />
}
