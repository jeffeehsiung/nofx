import { PromptVariantLab } from './PromptVariantLab'

// Re-export types for backward compatibility
export type { PromptVariant } from './PromptVariantLab'
export type { PromptVariantLabResponse as TraderPromptVariantsResponse } from './PromptVariantLab'
export type { PromptVariantPerformanceResponse as TraderPromptVariantResponse } from './PromptVariantLab'

interface LiveTraderPromptLabProps {
  traderId?: string
}

export function LiveTraderPromptLab({ traderId }: LiveTraderPromptLabProps) {
  return <PromptVariantLab type="trader" resourceId={traderId} />
}
