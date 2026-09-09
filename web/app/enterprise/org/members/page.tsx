"use client"

import Link from "next/link"
import { useSearchParams } from "next/navigation"
import {
  ArrowLeftIcon,
  PackageIcon,
  RefreshCwIcon,
  UserMinusIcon,
  UserPlusIcon,
  UsersRoundIcon,
  WrenchIcon,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"
import type { TableColumnsType } from "antd"
import {
  ContentModule,
  DataTable,
  IconButton,
  PageShell,
  RailopsButton,
  StatusTag,
  type StatusTagTone,
} from "@railops/ui"

import { OptionCombobox } from "@/components/option-combobox"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useConfirm } from "@/components/confirm-provider"
import {
  fetchAgentProfilesAll,
  fetchAgentTeamsAll,
  removeAgentTeamMember,
  upsertAgentTeamMember,
  type AdminAgentProfile,
  type AdminAgentTeam,
} from "@/lib/api/admin"
import { getRequestTenantId } from "@/lib/api/client"
import {
  fetchEnterpriseIAMMembers,
  type EnterpriseIAMMember,
} from "@/lib/api/platform-iam"
import { ServiceStatus } from "@/lib/generated/enums"
import { formatDateTime } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"
import { agentMemberName } from "../member-name"

const PRODUCT_TEAM_TYPE = "product_repair"
const TECHNICAL_TEAM_TYPE = "technical_repair"
const ENTERPRISE_MEMBER_OPTION_PAGE_SIZE = 50

const orgMembersI18nPrefix = "orgExtract.orgMembers."
type OrgMembersT = ReturnType<typeof useI18n>
const om = (t: OrgMembersT, key: string, values?: Record<string, string | number>) =>
  t(`${orgMembersI18nPrefix}${key}`, values)

function serviceStatusLabel(value: number, t: OrgMembersT) {
  return value === ServiceStatus.Busy ? om(t, "serviceStatus.busy") : om(t, "serviceStatus.idle")
}

function serviceStatusTone(value: number): StatusTagTone {
  return value === ServiceStatus.Busy ? "warning" : "success"
}

function memberScopeText(item: AdminAgentProfile, t: OrgMembersT, isTechnicalTeam: boolean) {
  const count = item.teamIds?.length || 1
  if (isTechnicalTeam) {
    return item.teamName || om(t, "scope.currentMaintenance")
  }
  return count > 1 ? om(t, "scope.joined", { count }) : item.teamName || om(t, "scope.current")
}

function enterpriseMemberLabel(member: EnterpriseIAMMember) {
  return `${member.display_name || member.username} · ${member.job_title || member.member_no || member.username}`
}

export default function EnterpriseOrgMembersPage() {
  const t = useI18n()
  const confirm = useConfirm()
  const searchParams = useSearchParams()
  const requestedTeamId = Number(searchParams.get("teamId")) || 0
  const [teams, setTeams] = useState<AdminAgentTeam[]>([])
  const [selectedTeamId, setSelectedTeamId] = useState<string>(requestedTeamId > 0 ? String(requestedTeamId) : "")
  const [members, setMembers] = useState<AdminAgentProfile[]>([])
  const [enterpriseMembers, setEnterpriseMembers] = useState<EnterpriseIAMMember[]>([])
  const [selectedUserId, setSelectedUserId] = useState("")
  const [memberSearch, setMemberSearch] = useState("")
  const [loadingTeams, setLoadingTeams] = useState(true)
  const [loadingMembers, setLoadingMembers] = useState(false)
  const [loadingEnterpriseMembers, setLoadingEnterpriseMembers] = useState(false)
  const [joining, setJoining] = useState(false)
  const [removingUserId, setRemovingUserId] = useState<number | null>(null)
  const enterpriseMemberRequestRef = useRef(0)

  const productTeams = useMemo(
    () => teams.filter((item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0),
    [teams],
  )
  const technicalTeam = useMemo(
    () => teams.find((item) => item.teamType === TECHNICAL_TEAM_TYPE) ?? null,
    [teams],
  )
  const selectableTeams = useMemo(
    () => productTeams.length > 0 ? productTeams : technicalTeam ? [technicalTeam] : [],
    [productTeams, technicalTeam],
  )
  const selectedTeam = useMemo(
    () => selectableTeams.find((item) => String(item.id) === selectedTeamId) ?? null,
    [selectableTeams, selectedTeamId],
  )
  const isTechnicalTeam = selectedTeam?.teamType === TECHNICAL_TEAM_TYPE
  const joinedUserIds = useMemo(() => new Set(members.map((item) => item.userId)), [members])
  const candidateOptions = useMemo(
    () =>
      enterpriseMembers
        .filter((item) => item.status === 0 && item.user_id > 0 && !joinedUserIds.has(item.user_id))
        .map((item) => ({ value: String(item.user_id), label: enterpriseMemberLabel(item) })),
    [enterpriseMembers, joinedUserIds],
  )

  const loadTeams = useCallback(async () => {
    setLoadingTeams(true)
    try {
      const data = await fetchAgentTeamsAll()
      const productRows = data.filter((item) => item.teamType === PRODUCT_TEAM_TYPE && item.productId > 0)
      const technicalRow = data.find((item) => item.teamType === TECHNICAL_TEAM_TYPE)
      const selectableRows = productRows.length > 0 ? productRows : technicalRow ? [technicalRow] : []
      setTeams(data)
      setSelectedTeamId((current) => {
        if (requestedTeamId > 0 && selectableRows.some((item) => item.id === requestedTeamId)) {
          return String(requestedTeamId)
        }
        if (current && selectableRows.some((item) => String(item.id) === current)) {
          return current
        }
        return selectableRows[0] ? String(selectableRows[0].id) : ""
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : om(t, "toasts.loadSupportTeamsFailed"))
    } finally {
      setLoadingTeams(false)
    }
  }, [requestedTeamId, t])

  const loadMembers = useCallback(async () => {
    if (!selectedTeam) {
      setMembers([])
      return
    }
    setLoadingMembers(true)
    try {
      const data = await fetchAgentProfilesAll({ teamId: selectedTeam.id })
      setMembers(data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : om(t, selectedTeam.teamType === TECHNICAL_TEAM_TYPE ? "toasts.loadMaintenanceMembersFailed" : "toasts.loadMembersFailed"))
    } finally {
      setLoadingMembers(false)
    }
  }, [selectedTeam, t])

  const loadEnterpriseMembers = useCallback(async (search = "") => {
    const requestId = enterpriseMemberRequestRef.current + 1
    enterpriseMemberRequestRef.current = requestId
    setLoadingEnterpriseMembers(true)
    try {
      const keyword = search.trim()
      const page = await fetchEnterpriseIAMMembers(
        {
          page: 1,
          limit: ENTERPRISE_MEMBER_OPTION_PAGE_SIZE,
          status: "0",
          search: keyword || undefined,
        },
        getRequestTenantId()
      )
      if (enterpriseMemberRequestRef.current === requestId) {
        setEnterpriseMembers(page.results || [])
      }
    } catch (error) {
      if (enterpriseMemberRequestRef.current === requestId) {
        toast.error(error instanceof Error ? error.message : om(t, "toasts.loadEnterpriseMembersFailed"))
      }
    } finally {
      if (enterpriseMemberRequestRef.current === requestId) {
        setLoadingEnterpriseMembers(false)
      }
    }
  }, [t])

  useEffect(() => {
    void loadTeams()
  }, [loadTeams])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void loadEnterpriseMembers(memberSearch)
    }, memberSearch.trim() ? 250 : 0)
    return () => window.clearTimeout(timer)
  }, [loadEnterpriseMembers, memberSearch])

  useEffect(() => {
    setSelectedUserId("")
    void loadMembers()
  }, [loadMembers])

  async function handleJoinMember() {
    if (!selectedTeam) {
      toast.error(om(t, "toasts.selectSupportTeamFirst"))
      return
    }
    const userId = Number(selectedUserId)
    if (!Number.isFinite(userId) || userId <= 0) {
      toast.error(om(t, "toasts.selectUserToJoin"))
      return
    }
    setJoining(true)
    try {
      await upsertAgentTeamMember({
        teamId: selectedTeam.id,
        userId,
        dispatchEnabled: true,
        dispatchWeight: 1,
      })
      const member = enterpriseMembers.find((item) => item.user_id === userId)
      toast.success(
        om(t, "toasts.joinedMember", {
          member: member?.display_name || member?.username || om(t, "toasts.fallbackMember"),
          team: selectedTeam.productName || selectedTeam.name,
        }),
      )
      setSelectedUserId("")
      setMemberSearch("")
      await loadMembers()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : om(t, isTechnicalTeam ? "toasts.joinMaintenanceFailed" : "toasts.joinFailed"))
    } finally {
      setJoining(false)
    }
  }

  async function handleRemoveMember(member: AdminAgentProfile) {
    if (!selectedTeam) return
    const isLeader = !isTechnicalTeam && selectedTeam.leaderUserId === member.userId
    if (isLeader) {
      toast.error(om(t, "toasts.removeLeaderFirst"))
      return
    }
    const name = agentMemberName(member)
    const accepted = await confirm({
      title: om(t, isTechnicalTeam ? "confirm.removeMaintenanceTitle" : "confirm.removeTitle", { name }),
      confirmText: om(t, "confirm.removeConfirm"),
      cancelText: om(t, "confirm.cancel"),
      variant: "destructive",
    })
    if (!accepted) return
    setRemovingUserId(member.userId)
    try {
      const result = await removeAgentTeamMember({ teamId: selectedTeam.id, userId: member.userId })
      const recovered = result.pendingTicketsRecovered + result.pendingConversationsRecovered
      toast.success(
        result.alreadyRemoved
          ? om(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberMissing" : "toasts.removeMemberMissing", { name })
          : recovered > 0
            ? om(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberDoneRecovered" : "toasts.removeMemberDoneRecovered", { name, count: recovered })
            : om(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberDone" : "toasts.removeMemberDone", { name }),
      )
      await loadMembers()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : om(t, isTechnicalTeam ? "toasts.removeMaintenanceMemberFailed" : "toasts.removeMemberFailed"))
    } finally {
      setRemovingUserId(null)
    }
  }

  const memberColumns = (t: OrgMembersT): TableColumnsType<AdminAgentProfile> => [
    {
      title: om(t, "table.colMember"),
      key: "member",
      width: 200,
      render: (_, item) => (
        <div>
          <div className="font-medium">{agentMemberName(item)}</div>
          <div className="text-xs text-muted-foreground">{item.username || item.nickname || om(t, "table.userFallback", { id: item.userId })}</div>
        </div>
      ),
    },
    {
      title: om(t, isTechnicalTeam ? "table.colMaintenanceScope" : "table.colScope"),
      key: "scope",
      width: 200,
      render: (_, item) => (
        <div>
          <StatusTag tone="neutral">{memberScopeText(item, t, isTechnicalTeam)}</StatusTag>
          <div className="mt-1 text-xs text-muted-foreground">{item.teamName || selectedTeam?.name || "-"}</div>
        </div>
      ),
    },
    {
      title: om(t, "table.colService"),
      key: "service",
      width: 120,
      render: (_, item) => (
        <StatusTag tone={serviceStatusTone(item.serviceStatus)}>
          {serviceStatusLabel(item.serviceStatus, t)}
        </StatusTag>
      ),
    },
    {
      title: om(t, "table.colDispatch"),
      key: "dispatch",
      width: 200,
      render: (_, item) => (
        <div className="flex flex-wrap gap-1.5">
          <StatusTag tone={item.autoAssignEnabled ? "success" : "neutral"}>
            {item.autoAssignEnabled ? om(t, "table.autoDispatchEnabled") : om(t, "table.autoDispatchManual")}
          </StatusTag>
          <StatusTag tone={item.teamDispatchEnabled ? "blue" : "neutral"}>
            {item.teamDispatchEnabled ? om(t, "table.teamWeight", { weight: item.dispatchWeight || 1 }) : om(t, "table.teamAutoDispatchOff")}
          </StatusTag>
          <StatusTag tone="neutral">{om(t, "table.manualDispatchAvailable")}</StatusTag>
        </div>
      ),
    },
    {
      title: om(t, "table.colLastStatus"),
      key: "lastStatus",
      width: 240,
      render: (_, item) => (
        <div className="space-y-1 text-sm">
          <div>{om(t, "table.onlineAt", { time: formatDateTime(item.lastOnlineAt) })}</div>
          <div className="text-muted-foreground">{om(t, "table.statusAt", { time: formatDateTime(item.lastStatusAt) })}</div>
        </div>
      ),
    },
    {
      title: om(t, "table.colActions"),
      key: "actions",
      width: 80,
      align: "right",
      render: (_, item) => (
        <IconButton
          icon={<UserMinusIcon className="size-4" />}
          tooltip={!isTechnicalTeam && selectedTeam?.leaderUserId === item.userId
            ? om(t, "tooltip.changeLeaderFirst")
            : om(t, isTechnicalTeam ? "tooltip.removeMaintenance" : "tooltip.remove")}
          aria-label={om(t, isTechnicalTeam ? "tooltip.removeMaintenanceAria" : "tooltip.removeAria", { name: agentMemberName(item) })}
          danger
          disabled={removingUserId === item.userId}
          onClick={() => void handleRemoveMember(item)}
        />
      ),
    },
  ]

  return (
    <PageShell
      title={om(t, isTechnicalTeam ? "maintenancePageTitle" : "pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-org-members-page"
      actions={
        <>
          <Link href={selectedTeam ? `/enterprise/org?teamId=${selectedTeam.id}` : "/enterprise/org"}>
            <RailopsButton size="small">
              <ArrowLeftIcon />{om(t, "actions.backToOrg")}
            </RailopsButton>
          </Link>
          <div className="w-full sm:w-72">
            <OptionCombobox
              value={selectedTeamId}
              options={selectableTeams.map((team) => ({ value: String(team.id), label: team.productName || team.name }))}
              placeholder={loadingTeams ? om(t, "teamPicker.loadingSupport") : om(t, isTechnicalTeam ? "teamPicker.selectMaintenance" : "teamPicker.select")}
              searchPlaceholder={om(t, isTechnicalTeam ? "teamPicker.searchMaintenance" : "teamPicker.search")}
              emptyText={om(t, "teamPicker.emptySupport")}
              onChange={(value) => setSelectedTeamId(value)}
            />
          </div>
          <RailopsButton
            onClick={() => void Promise.all([loadTeams(), loadEnterpriseMembers(memberSearch), loadMembers()])}
            disabled={loadingTeams || loadingMembers || loadingEnterpriseMembers}
          >
            <RefreshCwIcon className={loadingTeams || loadingMembers || loadingEnterpriseMembers ? "animate-spin" : undefined} />
            {om(t, "actions.refresh")}
          </RailopsButton>
        </>
      }
    >

      {selectedTeam ? (
        <section className="rhd-railops-org-members-summary">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              {isTechnicalTeam ? <WrenchIcon className="size-4 text-muted-foreground" /> : <PackageIcon className="size-4 text-muted-foreground" />}
              <strong>{selectedTeam.productName || selectedTeam.name}</strong>
              <StatusTag tone="neutral">{om(t, isTechnicalTeam ? "header.technicalGroup" : "header.productGroup")}</StatusTag>
              <StatusTag tone="neutral">{om(t, "header.membersNameCount", { count: members.length })}</StatusTag>
            </div>
          </div>
          <div className="rhd-railops-org-members-join">
            <div className="text-sm font-medium">{om(t, "joinPanel.title")}</div>
            <div className="mt-2 flex gap-2">
              <div className="min-w-0 flex-1">
                <OptionCombobox
                  value={selectedUserId}
                  options={candidateOptions}
                  placeholder={loadingEnterpriseMembers ? om(t, "joinPanel.loading") : candidateOptions.length > 0 ? om(t, "joinPanel.selectMember") : om(t, "joinPanel.noJoinable")}
                  searchPlaceholder={om(t, "joinPanel.search")}
                  emptyText={loadingEnterpriseMembers ? om(t, "joinPanel.loading") : om(t, "joinPanel.noJoinable")}
                  searchValue={memberSearch}
                  onSearchChange={setMemberSearch}
                  shouldFilter={false}
                  onChange={(value) => setSelectedUserId(value)}
                />
              </div>
              <RailopsButton onClick={() => void handleJoinMember()} disabled={!selectedUserId || joining}>
                <UserPlusIcon />
                {om(t, "joinPanel.join")}
              </RailopsButton>
            </div>
          </div>
        </section>
      ) : null}

      <ContentModule
        className="rhd-railops-org-members-table-module"
        title={(
          <span className="rhd-railops-org-members-title">
            <UsersRoundIcon className="size-4" />
            {om(t, "tableModule.title")}
          </span>
        )}
        extra={<StatusTag tone="blue">{om(t, "tableModule.membersCount", { count: members.length })}</StatusTag>}
      >
        <div className="overflow-x-auto">
          <DataTable<AdminAgentProfile>
            className="rhd-railops-org-members-table"
            size="small"
            rowKey={(record) => record.id}
            dataSource={members}
            loading={loadingMembers}
            emptyDescription={om(t, "tableModule.empty")}
            columns={memberColumns(t)}
          />
        </div>
      </ContentModule>
    </PageShell>
  )
}
