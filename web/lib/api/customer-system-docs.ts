"use client"

import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import type { CustomerPortalSystemIntroDoc } from "@/lib/api/customer-portal-types"

/** 获取已上架的系统介绍文档（访客可见） */
export async function fetchCustomerSystemIntroDocs(): Promise<CustomerPortalSystemIntroDoc[]> {
  return customerPortalRequest<CustomerPortalSystemIntroDoc[]>("/system-intro-docs")
}
