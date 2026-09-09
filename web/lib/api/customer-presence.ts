"use client"

import { customerPortalRequest } from "@/lib/api/customer-portal-client"

export type CustomerPortalPresence = {
  online: boolean
  last_seen_at: string
}

export function heartbeatCustomerPortalPresence() {
  return customerPortalRequest<CustomerPortalPresence>("/presence/_heartbeat", {
    method: "POST",
  })
}
