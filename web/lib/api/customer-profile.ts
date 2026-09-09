"use client"

import type {
  CustomerAccountDeletion,
  CustomerPortalProfile,
  CustomerPortalProfileUpdatePayload,
} from "@/lib/api/customer-portal-types"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"

export function fetchCustomerProfile() {
  return customerPortalRequest<CustomerPortalProfile>("/me")
}

export function updateCustomerProfile(payload: CustomerPortalProfileUpdatePayload) {
  return customerPortalRequest<CustomerPortalProfile>("/me/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchCustomerAccountDeletionStatus() {
  return customerPortalRequest<CustomerAccountDeletion>("/account-deletion")
}

export function deleteCustomerAccount(payload: { current_password: string; confirmation: "DELETE" }) {
  return customerPortalRequest<CustomerAccountDeletion>("/account-deletion", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}
