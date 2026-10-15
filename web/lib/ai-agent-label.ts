/**
 * Builds the label used by selectors that list a product and its reception
 * agent. Names arrive already rendered in the interface language. Generated
 * product agents carry the product name, so the product is not repeated.
 */
export function aiAgentOptionLabel(productName: string | undefined, agentName: string | undefined) {
  const product = productName?.trim() || ""
  const agent = agentName?.trim() || ""
  if (!product) return agent
  if (!agent || agent === product) return product
  if (agent.includes(product)) return agent
  return `${product} · ${agent}`
}
