import type { ProjectConfiguration, ProjectRuntime } from "./api/project-configuration"

function runtimeSecretRefs(runtime?: ProjectRuntime): string[] {
  if (!runtime) return []
  return [...new Set([
    runtime.mail.password_ref,
    runtime.mail.imap?.password_ref ?? "",
    ...runtime.integrations.flatMap(integration => [integration.secret_ref, integration.key_ref]),
  ].filter(Boolean))]
}

// Normalize a separate editable copy; historical documents and their digests
// must remain unchanged, including old drafts with null arrays.
export function editableProjectConfiguration(document: ProjectConfiguration): ProjectConfiguration {
  const copy = structuredClone(document)
  // 受理规则已移除：历史版本里仍带着 intake 字段，载入时丢掉，避免新草稿被旧字段卡住。
  delete (copy as { intake?: unknown }).intake
  copy.projects ??= []
  copy.secret_refs ??= []
  return copy
}

export function restoreProjectConfiguration(document: ProjectConfiguration, current?: ProjectConfiguration): ProjectConfiguration {
  const copy = editableProjectConfiguration(document)
  if (!copy.runtime && current?.runtime) {
    copy.schema_version = 2
    copy.runtime = structuredClone(current.runtime)
    // Carry only dependencies of the retained settings, not unrelated current
    // declarations that may no longer be available when restoring history.
    copy.secret_refs = [...new Set([...copy.secret_refs, ...runtimeSecretRefs(copy.runtime)])]
  }
  return copy
}

export function replaceProjectRuntime(document: ProjectConfiguration, runtime: ProjectRuntime): ProjectConfiguration {
  const previousRefs = new Set(runtimeSecretRefs(document.runtime))
  const nextRefs = runtimeSecretRefs(runtime)
  const nextRefSet = new Set(nextRefs)
  // A reference can be shared by mail and multiple integrations. Remove an old
  // dependency only after every consumer has stopped using it; retain explicit
  // declarations unrelated to the edited runtime.
  const secret_refs = [...new Set([
    ...document.secret_refs.filter(ref => !previousRefs.has(ref) || nextRefSet.has(ref)),
    ...nextRefs,
  ])]
  return { ...document, runtime, secret_refs }
}
