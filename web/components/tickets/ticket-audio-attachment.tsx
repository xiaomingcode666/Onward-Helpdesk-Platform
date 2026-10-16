"use client"

import type { AssetRefDTO } from "@/lib/api/types"
import { buildImMediaURL } from "@/lib/api/im"

export function isAudioAttachment(asset: Pick<AssetRefDTO, "file_type" | "asset_id">) {
  return Boolean(asset.asset_id) && asset.file_type.toLowerCase().startsWith("audio/")
}

// Plays a ticket's audio attachment (for example a customer's voice ticket
// recording) through the authenticated media endpoint.
export function TicketAudioAttachment({ assetId }: { assetId: string }) {
  return (
    <audio
      controls
      preload="none"
      className="mt-1 h-9 w-full"
      data-testid="ticket-audio-attachment"
      src={buildImMediaURL(assetId)}
    />
  )
}
