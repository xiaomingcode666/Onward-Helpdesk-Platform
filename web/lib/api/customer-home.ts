"use client"

import type { CustomerPortalHome } from "@/lib/api/customer-portal-types"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"

export function fetchCustomerHome() {
  return customerPortalRequest<CustomerPortalHome>("/home")
}
