export interface Product {
  id: number
  name: string
  description: string
  created_at: string
}

export interface TestStep {
  position: number
  action: string
  expected_result?: string
}

export interface Category {
  id: number
  name: string
}

export interface TestCase {
  id: number
  product_id: number
  category_id?: number
  author_id?: number
  default_tester_id?: number
  reviewer_id?: number
  summary: string
  preconditions?: string
  requirement?: string
  extra_link?: string
  notes?: string
  format: 'steps' | 'gherkin' | 'markdown' | 'checklist'
  gherkin_body?: string
  priority: string
  status: 'draft' | 'confirmed' | 'disabled'
  is_automated?: boolean
  script?: string
  arguments?: string
  steps: TestStep[]
  created_at: string
  updated_at: string
}

const TOKEN_KEY = 'kiwit_jwt'
const ROLE_KEY = 'kiwit_role'

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setAuthToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export function setUserRole(role: string | null) {
  if (role) localStorage.setItem(ROLE_KEY, role)
  else localStorage.removeItem(ROLE_KEY)
}

// getUserRole returns the authenticated user's role for UI decisions (e.g.
// hiding admin-only tabs). The role is stored at login; for sessions persisted
// before that, the JWT payload is decoded without verification (UI only —
// the backend always authorizes).
export function getUserRole(): string | null {
  const stored = localStorage.getItem(ROLE_KEY)
  if (stored) return stored
  const token = getAuthToken()
  if (!token) return null
  const parts = token.split('.')
  if (parts.length !== 3) return null
  try {
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')))
    return typeof payload.role === 'string' ? payload.role : null
  } catch {
    return null
  }
}

export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const token = getAuthToken()
  const res = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
    ...init,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }
  const text = await res.text()
  if (!text) {
    return undefined as T
  }
  return JSON.parse(text) as T
}

export async function downloadAuthenticated(url: string, filename: string, openInNewTab = false): Promise<void> {
  const token = getAuthToken()
  const res = await fetch(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }
  const blob = await res.blob()
  const blobUrl = URL.createObjectURL(blob)
  if (openInNewTab) {
    window.open(blobUrl, '_blank', 'noopener,noreferrer')
    setTimeout(() => URL.revokeObjectURL(blobUrl), 60_000)
    return
  }
  const a = document.createElement('a')
  a.href = blobUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(blobUrl)
}

export interface Version {
  id: number
  product_id: number
  value: string
}

export interface Build {
  id: number
  version_id: number
  name: string
  is_active: boolean
}

export interface TestPlan {
  id: number
  product_id: number
  version_id: number
  parent_id?: number
  name: string
  description?: string
  plan_type?: string
  extra_link?: string
  is_active: boolean
  created_at: string
}

export interface PlanCaseItem {
  test_case_id: number
  position: number
  summary: string
  priority: string
  status: string
}

export interface TestRun {
  id: number
  test_plan_id: number
  build_id: number
  environment_id?: number
  manager_id?: number
  default_tester_id?: number
  summary: string
  notes?: string
  planned_start?: string
  planned_stop?: string
  started_at: string
  stopped_at?: string
}

export interface TestExecution {
  id: number
  test_run_id: number
  test_case_id: number
  summary: string
  status: string
  comment?: string
  assignee_id?: number
  tested_by_id?: number
  environment_id?: number
  case_text_version?: number
  latest_case_version?: number
  started_at?: string
  stopped_at?: string
  dataset_id?: number
  dataset_name?: string
}

export interface AIServer {
  id: number
  name: string
  provider: string
  base_url: string
  model: string
  api_key?: string
  temperature: number
  max_tokens: number
  timeout_sec: number
  retries: number
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface AIServerInput {
  name: string
  provider?: string
  base_url: string
  model: string
  api_key?: string
  temperature?: number
  max_tokens?: number
  timeout_sec?: number
  retries?: number
  is_default?: boolean
}

export interface Campaign {
  id: number
  product_id: number
  name: string
  description?: string
  status: 'active' | 'completed' | 'archived'
  start_date?: string
  end_date?: string
  created_at: string
  updated_at: string
}

export interface Iteration {
  id: number
  campaign_id: number
  name: string
  status: 'planned' | 'in_progress' | 'done'
  start_date?: string
  end_date?: string
  created_at: string
  updated_at: string
}

export interface CampaignPlanNode {
  id: number
  name: string
  is_active: boolean
  iteration_id?: number
}

export interface CampaignTree extends Campaign {
  iterations: (Iteration & { plans: CampaignPlanNode[] })[]
  plans: CampaignPlanNode[]
}

export interface VisualBaseline {
  id: number
  test_case_id: number
  attachment_id: number
  label?: string
  created_at: string
}

export interface VisualComparison {
  id: number
  execution_id: number
  baseline_id: number
  attachment_id: number
  diff_percent: number
  verdict: 'match' | 'mismatch' | 'no-baseline'
  ai_verdict?: string
  created_at: string
}

export interface ExploratorySession {
  id: number
  product_id: number
  test_run_id?: number
  charter: string
  timebox_minutes: number
  tester_id?: number
  notes?: string
  findings: string
  created_at: string
  updated_at: string
}

export interface UXMetricRow {
  product_id: number
  product_name: string
  sessions: number
  avg_sus: number
  avg_task_time_sec: number
  avg_errors: number
}

export interface CustomFieldDef {
  id: number
  entity_type: string
  name: string
  field_type: 'text' | 'number' | 'date' | 'list' | 'multiselect' | 'boolean'
  options: string
  required: boolean
  created_at: string
}

export interface CustomFieldValue {
  id: number
  field_id: number
  entity_type: string
  entity_id: number
  value: string
  updated_at: string
}

export interface CustomFieldWithValue extends CustomFieldDef {
  value?: string
}

export interface Dataset {
  id: number
  test_case_id: number
  name: string
  params: string
  created_at: string
}

export interface LinkRefItem {
  id: number
  entity_type: string
  entity_id: number
  name: string
  url: string
}

export interface ExecutionLinkItem {
  id: number
  test_execution_id: number
  name: string
  url: string
  created_at: string
}

export interface User {
  id: number
  email: string
  username: string
  role: string
  is_active?: boolean
  created_at?: string
}

export interface Classification {
  id: number
  name: string
}

export interface PlanType {
  id: number
  name: string
}

export interface UserGroup {
  id: number
  name: string
  created_at?: string
  members?: number[]
}

export interface Requirement {
  id: number
  product_id: number
  key: string
  name: string
  description?: string
  version: string
  status: 'draft' | 'approved' | 'obsolete'
  priority: string
  category?: string
  ai_quality?: string
  author_id?: number
  created_at: string
  updated_at: string
}

export interface CoverageEntry {
  test_case_id: number
  summary: string
  status: string
  latest_execution_id?: number
  latest_execution_status?: string
  latest_run_id?: number
  linked_at: string
}

export interface Milestone {
  id: number
  version_id: number
  name: string
  description?: string
  due_date?: string
  status: 'planned' | 'in_progress' | 'done'
  signoff_decision?: 'go' | 'no-go'
  signoff_user_id?: number
  signoff_note?: string
  signoff_at?: string
  created_at: string
  updated_at: string
}

export interface MilestoneProgress {
  milestone_id: number
  total: number
  passed: number
  failed: number
  blocked: number
  skipped: number
  running: number
  idle: number
  percent_done: string
}

export interface ApiTokenInfo {
  id: number
  name: string
  created_at: string
  last_used_at?: string
}

export interface AttachmentItem {
  id: number
  entity_type: 'test_case' | 'test_execution'
  entity_id: number
  filename: string
  content_type: string
  size_bytes: number
}

export interface TelemetrySummary {
  products: number
  test_cases: number
  test_plans: number
  test_runs: number
  executions: {
    total: number
    passed: number
    failed: number
    blocked: number
    skipped: number
    idle: number
  }
}

export interface TrendPoint {
  date: string
  status: string
  count: number
}

export interface MatrixRow {
  test_case_id: number
  summary: string
  status: string
  count: number
}

export interface CaseHealth {
  test_case_id: number
  summary: string
  total: number
  failed: number
  fail_rate: number
}

export interface NamedCount {
  name: string
  count: number
}

export interface TelemetryBreakdown {
  automated: number
  manual: number
  by_priority: NamedCount[]
  by_status: NamedCount[]
  by_category: NamedCount[]
}

export interface RunProgress {
  test_run_id: number
  test_plan_id: number
  plan_name: string
  summary: string
  started_at: string
  stopped: boolean
  total: number
  passed: number
  failed: number
  blocked: number
  skipped: number
  idle: number
  progress: number
}

export interface RecentExecution {
  execution_id: number
  test_case_id: number
  test_run_id: number
  case_summary: string
  run_summary: string
  status: string
  tester: string
  updated_at: string
}

export interface TesterLoad {
  user_id: number
  username: string
  total: number
  passed: number
  failed: number
  pass_rate: number
}

export interface ExecutionDashboard {
  runs: RunProgress[]
  recent_executions: RecentExecution[]
  testers: TesterLoad[]
}

export interface DashboardMyRun {
  id: number
  summary: string
  plan_id: number
  stopped_at?: string
  started_at: string
}

export interface DashboardActivity {
  id: number
  entity_type: string
  entity_id: number
  action: string
  actor_name: string
  created_at: string
}

export interface DashboardSnapshot {
  my_runs: DashboardMyRun[]
  recent_activity: DashboardActivity[]
}

export interface WebhookItem {
  id: number
  name: string
  url: string
  events: string[]
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface WebhookTestResult {
  delivered: boolean
  status_code?: number
  error?: string
}

export interface TelemetryFilterParams {
  product_id?: number
  version_id?: number
  build_id?: number
  test_plan_id?: number
  from?: string
  to?: string
}

function buildTelemetryQuery(params?: TelemetryFilterParams): string {
  if (!params) return ''
  const qs = new URLSearchParams()
  if (params.product_id) qs.set('product_id', String(params.product_id))
  if (params.version_id) qs.set('version_id', String(params.version_id))
  if (params.build_id) qs.set('build_id', String(params.build_id))
  if (params.test_plan_id) qs.set('test_plan_id', String(params.test_plan_id))
  if (params.from) qs.set('from', params.from)
  if (params.to) qs.set('to', params.to)
  return qs.toString() ? `?${qs.toString()}` : ''
}

export interface BugItem {
  id: number
  test_execution_id: number
  tracker_id?: number
  external_key: string
  url: string
  title?: string
  status: string
  last_synced_at?: string
}

export interface InternalBug {
  id: number
  product_id: number
  build_id?: number
  test_execution_id?: number
  summary: string
  description?: string
  status: 'open' | 'in_progress' | 'resolved' | 'closed'
  severity: 'low' | 'medium' | 'high' | 'critical'
  reporter_id?: number
  assignee_id?: number
  created_at: string
  updated_at: string
}

export interface CommentItem {
  id: number
  entity_type: 'test_case' | 'test_execution'
  entity_id: number
  body: string
  created_at: string
}

export interface RevisionItem {
  id: number
  version: number
  summary: string
  priority: string
  status: string
  created_at: string
}

export interface ComponentItem {
  id: number
  product_id: number
  name: string
  description?: string
}

export interface TagItem {
  id: number
  name: string
}

export interface IssueTrackerItem {
  id: number
  name: string
  provider: 'github' | 'gitlab' | 'jira' | 'bugzilla' | 'azure'
  base_url: string
  api_username?: string
  created_at: string
}

export interface EnvironmentItem {
  id: number
  name: string
  description?: string
  properties: Record<string, string>
  created_at: string
}

export interface TemplateItem {
  id: number
  name: string
  body: string
  created_at: string
}

export interface PriorityItem {
  id: number
  value: string
  is_active: boolean
}

export interface ExecutionStatusItem {
  id: number
  name: string
  color: string
  weight: number
}

export interface EmailSettingsItem {
  entity_type: 'test_case' | 'test_plan'
  entity_id: number
  notify_on_update: boolean
  cc_list: string[]
}

export interface NotificationItem {
  id: number
  event_type: string
  entity_type?: string
  entity_id?: number
  title: string
  body?: string
  read_at?: string
  created_at: string
}

export interface NotificationEventPreference {
  event_type: string
  in_app: boolean
  email: boolean
}

export const api = {
  register: (body: { email: string; username: string; password: string }) =>
    request<User>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify(body) }),
  login: (body: { email: string; password: string }) =>
    request<{ access_token: string; token_type: string; user: User }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  createApiToken: (name: string) =>
    request<{ token: string; name: string }>('/api/v1/auth/tokens', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),
  listApiTokens: () => request<{ items: ApiTokenInfo[] }>('/api/v1/auth/tokens'),
  revokeApiToken: (id: number) => request<void>(`/api/v1/auth/tokens/${id}`, { method: 'DELETE' }),
  changePassword: (body: { current_password: string; new_password: string }) =>
    request<{ ok: boolean }>('/api/v1/auth/change-password', { method: 'POST', body: JSON.stringify(body) }),
  requestPasswordReset: (body: { email: string }) =>
    request<{ ok: boolean }>('/api/v1/auth/password-reset', { method: 'POST', body: JSON.stringify(body) }),
  confirmPasswordReset: (body: { token: string; new_password: string }) =>
    request<{ ok: boolean }>('/api/v1/auth/password-reset/confirm', { method: 'POST', body: JSON.stringify(body) }),
  listUsers: () => request<{ items: User[] }>('/api/v1/admin/users'),
  updateUser: (id: number, body: { role: 'viewer' | 'tester' | 'admin'; is_active: boolean }) =>
    request<User>(`/api/v1/admin/users/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
  listGroups: () => request<{ items: UserGroup[] }>('/api/v1/admin/groups'),
  createGroup: (name: string) =>
    request<UserGroup>('/api/v1/admin/groups', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteGroup: (id: number) => request<void>(`/api/v1/admin/groups/${id}`, { method: 'DELETE' }),
  addGroupMember: (groupId: number, userId: number) =>
    request<{ ok: boolean }>(`/api/v1/admin/groups/${groupId}/members`, { method: 'POST', body: JSON.stringify({ user_id: userId }) }),
  removeGroupMember: (groupId: number, userId: number) =>
    request<{ ok: boolean }>(`/api/v1/admin/groups/${groupId}/members/${userId}`, { method: 'DELETE' }),
  authMethods: () => request<{ sso: boolean; ldap: boolean }>('/api/v1/auth/methods'),
  ldapLogin: (body: { username: string; password: string }) =>
    request<{ access_token: string; token_type: string; user: User }>('/api/v1/auth/ldap-login', { method: 'POST', body: JSON.stringify(body) }),
  listClassifications: () => request<{ items: Classification[] }>('/api/v1/classifications'),
  createClassification: (name: string) =>
    request<Classification>('/api/v1/classifications', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteClassification: (id: number) => request<void>(`/api/v1/classifications/${id}`, { method: 'DELETE' }),
  listPlanTypes: () => request<{ items: PlanType[] }>('/api/v1/plantypes'),
  createPlanType: (name: string) =>
    request<PlanType>('/api/v1/plantypes', { method: 'POST', body: JSON.stringify({ name }) }),
  deletePlanType: (id: number) => request<void>(`/api/v1/plantypes/${id}`, { method: 'DELETE' }),
  listProducts: () => request<{ items: Product[] }>('/api/v1/products'),
  createProduct: (body: { name: string; description?: string }) =>
    request<Product>('/api/v1/products', { method: 'POST', body: JSON.stringify(body) }),
  listVersions: (productId: number) => request<{ items: Version[] }>(`/api/v1/products/${productId}/versions`),
  createVersion: (productId: number, value: string) =>
    request<Version>(`/api/v1/products/${productId}/versions`, { method: 'POST', body: JSON.stringify({ value }) }),
  listBuilds: (versionId: number) => request<{ items: Build[] }>(`/api/v1/versions/${versionId}/builds`),
  createBuild: (versionId: number, name: string) =>
    request<Build>(`/api/v1/versions/${versionId}/builds`, { method: 'POST', body: JSON.stringify({ name }) }),
  listCategories: () => request<{ items: Category[] }>('/api/v1/categories'),
  createCategory: (name: string) =>
    request<Category>('/api/v1/categories', { method: 'POST', body: JSON.stringify({ name }) }),

  listRequirements: (params?: { q?: string; product_id?: number; status?: string; priority?: string; category?: string; limit?: number; offset?: number }) => {
    const qs = new URLSearchParams()
    if (params?.q) qs.set('q', params.q)
    if (params?.product_id) qs.set('product_id', String(params.product_id))
    if (params?.status) qs.set('status', params.status)
    if (params?.priority) qs.set('priority', params.priority)
    if (params?.category) qs.set('category', params.category)
    if (params?.limit) qs.set('limit', String(params.limit))
    if (params?.offset) qs.set('offset', String(params.offset))
    const s = qs.toString()
    return request<{ items: Requirement[]; total: number }>(`/api/v1/requirements${s ? `?${s}` : ''}`)
  },
  createRequirement: (body: Omit<Requirement, 'id' | 'created_at' | 'updated_at'>) =>
    request<Requirement>('/api/v1/requirements', { method: 'POST', body: JSON.stringify(body) }),
  updateRequirement: (id: number, body: Omit<Requirement, 'id' | 'created_at' | 'updated_at'>) =>
    request<Requirement>(`/api/v1/requirements/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteRequirement: (id: number) => request<void>(`/api/v1/requirements/${id}`, { method: 'DELETE' }),
  linkRequirementCase: (id: number, testCaseId: number) =>
    request<void>(`/api/v1/requirements/${id}/link-case`, { method: 'POST', body: JSON.stringify({ test_case_id: testCaseId }) }),
  unlinkRequirementCase: (id: number, testCaseId: number) =>
    request<void>(`/api/v1/requirements/${id}/unlink-case`, { method: 'POST', body: JSON.stringify({ test_case_id: testCaseId }) }),
  requirementCoverage: (id: number) => request<{ items: CoverageEntry[] }>(`/api/v1/requirements/${id}/coverage`),
  testCaseRequirements: (id: number) => request<{ items: Requirement[] }>(`/api/v1/testcases/${id}/requirements`),
  listMilestones: (versionId?: number) =>
    request<{ items: Milestone[] }>(`/api/v1/milestones${versionId ? `?version_id=${versionId}` : ''}`),
  createMilestone: (body: { version_id: number; name: string; description?: string; due_date?: string; status?: string }) =>
    request<Milestone>('/api/v1/milestones', { method: 'POST', body: JSON.stringify(body) }),
  updateMilestone: (id: number, body: { version_id: number; name: string; description?: string; due_date?: string; status?: string }) =>
    request<Milestone>(`/api/v1/milestones/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteMilestone: (id: number) => request<void>(`/api/v1/milestones/${id}`, { method: 'DELETE' }),
  milestoneSignoff: (id: number, body: { decision: 'go' | 'no-go'; note?: string }) =>
    request<Milestone>(`/api/v1/milestones/${id}/signoff`, { method: 'POST', body: JSON.stringify(body) }),
  milestoneProgress: (id: number) => request<MilestoneProgress>(`/api/v1/milestones/${id}/progress`),
  milestoneLinkRun: (id: number, testRunId: number) =>
    request<void>(`/api/v1/milestones/${id}/link-run`, { method: 'POST', body: JSON.stringify({ test_run_id: testRunId }) }),
  milestoneUnlinkRun: (id: number, testRunId: number) =>
    request<void>(`/api/v1/milestones/${id}/unlink-run`, { method: 'POST', body: JSON.stringify({ test_run_id: testRunId }) }),
  milestoneRuns: (id: number) => request<{ items: number[] }>(`/api/v1/milestones/${id}/runs`),
  renderMarkdown: (md: string) =>
    request<{ html: string }>('/api/v1/markdown/render', { method: 'POST', body: JSON.stringify({ markdown: md }) }),
  listTestCases: (params?: {
    q?: string
    product_id?: number
    category_id?: number
    component_id?: number
    tag_id?: number
    priority?: string
    status?: string
    is_automated?: 'true' | 'false'
    author_id?: number
    limit?: number
    offset?: number
  }) => {
    const qs = new URLSearchParams()
    if (params?.q) qs.set('q', params.q)
    if (params?.product_id) qs.set('product_id', String(params.product_id))
    if (params?.category_id) qs.set('category_id', String(params.category_id))
    if (params?.component_id) qs.set('component_id', String(params.component_id))
    if (params?.tag_id) qs.set('tag_id', String(params.tag_id))
    if (params?.priority) qs.set('priority', params.priority)
    if (params?.status) qs.set('status', params.status)
    if (params?.is_automated) qs.set('is_automated', params.is_automated)
    if (params?.author_id) qs.set('author_id', String(params.author_id))
    if (params?.limit !== undefined) qs.set('limit', String(params.limit))
    if (params?.offset !== undefined) qs.set('offset', String(params.offset))
    const suffix = qs.toString() ? `?${qs.toString()}` : ''
    return request<{ items: TestCase[]; total?: number }>(`/api/v1/testcases${suffix}`)
  },
  getTestCase: (id: number) => request<TestCase>(`/api/v1/testcases/${id}`),
  createTestCase: (body: Omit<TestCase, 'id' | 'created_at' | 'updated_at'>) =>
    request<TestCase>('/api/v1/testcases', { method: 'POST', body: JSON.stringify(body) }),
  updateTestCase: (id: number, body: Omit<TestCase, 'id' | 'created_at' | 'updated_at'>) =>
    request<TestCase>(`/api/v1/testcases/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  cloneTestCase: (id: number, summary?: string) =>
    request<TestCase>(`/api/v1/testcases/${id}/clone`, { method: 'POST', body: JSON.stringify({ summary: summary || '' }) }),
  fetchReportHtml: async (milestoneIds: number[]): Promise<string> => {
    const token = getAuthToken()
    const res = await fetch(`/api/v1/reports/multi/html?milestone_ids=${milestoneIds.join(',')}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    return URL.createObjectURL(blob)
  },
  listCampaigns: (productId?: number) =>
    request<{ items: Campaign[] }>(`/api/v1/campaigns${productId ? `?product_id=${productId}` : ''}`),
  createCampaign: (body: { product_id: number; name: string; description?: string; status?: string; start_date?: string; end_date?: string }) =>
    request<Campaign>('/api/v1/campaigns', { method: 'POST', body: JSON.stringify(body) }),
  deleteCampaign: (id: number) => request<void>(`/api/v1/campaigns/${id}`, { method: 'DELETE' }),
  campaignTree: (id: number) => request<CampaignTree>(`/api/v1/campaigns/${id}/tree`),
  listIterations: (campaignId: number) => request<{ items: Iteration[] }>(`/api/v1/campaigns/${campaignId}/iterations`),
  createIteration: (campaignId: number, body: { name: string; status?: string }) =>
    request<Iteration>(`/api/v1/campaigns/${campaignId}/iterations`, { method: 'POST', body: JSON.stringify(body) }),
  deleteIteration: (id: number) => request<void>(`/api/v1/iterations/${id}`, { method: 'DELETE' }),
  assignPlanToCampaign: (campaignId: number, testPlanId: number, iterationId?: number) =>
    request<void>(`/api/v1/campaigns/${campaignId}/assign-plan`, { method: 'POST', body: JSON.stringify({ test_plan_id: testPlanId, iteration_id: iterationId }) }),
  aiSummarizeRun: (testRunId: number) =>
    request<{ summary: string; model: string }>('/api/v1/ai/summarize-run', { method: 'POST', body: JSON.stringify({ test_run_id: testRunId }) }),
  aiSuggestBug: (body: { summary: string; description?: string }) =>
    request<{ severity: string; title: string; model: string }>('/api/v1/ai/suggest-bug', { method: 'POST', body: JSON.stringify(body) }),
  aiGenerateSteps: (gherkin: string) =>
    request<{ code: string; model: string }>('/api/v1/ai/generate-steps', { method: 'POST', body: JSON.stringify({ gherkin }) }),
  listAIServers: () => request<{ items: AIServer[] }>('/api/v1/admin/ai-servers'),
  createAIServer: (body: AIServerInput) =>
    request<AIServer>('/api/v1/admin/ai-servers', { method: 'POST', body: JSON.stringify(body) }),
  updateAIServer: (id: number, body: AIServerInput) =>
    request<AIServer>(`/api/v1/admin/ai-servers/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteAIServer: (id: number) => request<void>(`/api/v1/admin/ai-servers/${id}`, { method: 'DELETE' }),
  aiGenerateTests: (body: { product_id: number; requirement_id?: number; text?: string; max_scenarios?: number }) =>
    request<{ created: { id: number; summary: string }[]; gherkin: string; model: string; scenarios: number }>('/api/v1/ai/generate-tests', { method: 'POST', body: JSON.stringify(body) }),
  aiSearch: (query: string) =>
    request<{ model: string; filters: Record<string, unknown>; explanation: string; items: { type: string; title: string; row: Record<string, unknown> }[]; llm_model: string }>('/api/v1/ai/search', { method: 'POST', body: JSON.stringify({ query }) }),
  aiVisualVerdict: (comparisonId: number) =>
    request<{ verdict: 'real-defect' | 'acceptable-change' | 'unclear'; confidence: number; explanation: string; diff_percent: number; pixel_verdict: string; model: string }>('/api/v1/ai/visual-verdict', { method: 'POST', body: JSON.stringify({ comparison_id: comparisonId }) }),
  aiReportSummary: (milestoneIds: number[]) =>
    request<{ summary: string; model: string }>('/api/v1/ai/report-summary', { method: 'POST', body: JSON.stringify({ milestone_ids: milestoneIds }) }),
  aiTriageBug: (bugId: number) =>
    request<{ duplicates: { bug_id: number; confidence: number; reason: string }[]; suggested_severity: string; suggested_status: string; triage_note: string; model: string }>('/api/v1/ai/triage-bug', { method: 'POST', body: JSON.stringify({ bug_id: bugId }) }),
  aiAnalyzeRequirement: (requirementId: number) =>
    request<{ quality_score: number; verdict: 'good' | 'needs-work' | 'poor'; summary: string; issues: { type: string; severity: string; description: string }[]; suggestions: string[]; model: string }>('/api/v1/ai/analyze-requirement', { method: 'POST', body: JSON.stringify({ requirement_id: requirementId }) }),
  aiRegressionScope: (requirementId: number) =>
    request<{ summary: string; retest: { case_id: number; summary: string; reason: string; priority: string }[]; risks: string[]; gaps: string[]; model: string }>('/api/v1/ai/regression-scope', { method: 'POST', body: JSON.stringify({ requirement_id: requirementId }) }),
  aiGenerateCharter: (objective: string) =>
    request<{ charter: string; model: string }>('/api/v1/ai/generate-charter', { method: 'POST', body: JSON.stringify({ objective }) }),
  aiNotesToFindings: (sessionId: number) =>
    request<{ findings: string[]; model: string }>('/api/v1/ai/notes-to-findings', { method: 'POST', body: JSON.stringify({ session_id: sessionId }) }),
  aiGenerateDatasets: (caseId: number, count = 5) =>
    request<{ datasets: { name: string; params: string; rationale: string }[]; model: string }>('/api/v1/ai/generate-datasets', { method: 'POST', body: JSON.stringify({ test_case_id: caseId, count }) }),
  listVisualBaselines: (caseId: number) =>
    request<{ items: VisualBaseline[] }>(`/api/v1/testcases/${caseId}/visual-baselines`),
  addVisualBaseline: (caseId: number, attachmentId: number, label?: string) =>
    request<VisualBaseline>(`/api/v1/testcases/${caseId}/visual-baselines`, { method: 'POST', body: JSON.stringify({ attachment_id: attachmentId, label }) }),
  deleteVisualBaseline: (id: number) => request<void>(`/api/v1/visual-baselines/${id}`, { method: 'DELETE' }),
  visualCompare: (executionId: number, attachmentId: number, baselineId = 0) =>
    request<VisualComparison>(`/api/v1/executions/${executionId}/visual-compare`, { method: 'POST', body: JSON.stringify({ attachment_id: attachmentId, baseline_id: baselineId }) }),
  listVisualComparisons: (executionId: number) =>
    request<{ items: VisualComparison[] }>(`/api/v1/executions/${executionId}/visual-comparisons`),
  listExploratory: (productId?: number) =>
    request<{ items: ExploratorySession[] }>(`/api/v1/exploratory${productId ? `?product_id=${productId}` : ''}`),
  createExploratory: (body: { product_id: number; test_run_id?: number; charter: string; timebox_minutes?: number; notes?: string; findings?: string }) =>
    request<ExploratorySession>('/api/v1/exploratory', { method: 'POST', body: JSON.stringify(body) }),
  updateExploratory: (id: number, body: { product_id: number; test_run_id?: number; charter: string; timebox_minutes?: number; tester_id?: number; notes?: string; findings?: string }) =>
    request<ExploratorySession>(`/api/v1/exploratory/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteExploratory: (id: number) => request<void>(`/api/v1/exploratory/${id}`, { method: 'DELETE' }),
  getUXMetrics: (params?: TelemetryFilterParams) =>
    request<UXMetricRow[]>(`/api/v1/telemetry/ux-metrics${buildTelemetryQuery(params)}`),
  bulkUpdateTestCases: (body: { ids: number[]; status?: string; priority?: string }) =>
    request<{ updated: number }>('/api/v1/testcases/bulk', { method: 'PATCH', body: JSON.stringify(body) }),

  listTestPlans: (params?: { product_id?: number; version_id?: number; is_active?: 'true' | 'false'; tag_id?: number; q?: string }) => {
    const qs = new URLSearchParams()
    if (params?.product_id) qs.set('product_id', String(params.product_id))
    if (params?.version_id) qs.set('version_id', String(params.version_id))
    if (params?.is_active) qs.set('is_active', params.is_active)
    if (params?.tag_id) qs.set('tag_id', String(params.tag_id))
    if (params?.q) qs.set('q', params.q)
    const suffix = qs.toString() ? `?${qs.toString()}` : ''
    return request<{ items: TestPlan[] }>(`/api/v1/testplans${suffix}`)
  },
  listTestPlanCases: (planId: number) => request<{ items: PlanCaseItem[] }>(`/api/v1/testplans/${planId}/cases`),
  createTestPlan: (body: { product_id: number; version_id: number; parent_id?: number; name: string; description?: string; plan_type?: string; extra_link?: string }) =>
    request<TestPlan>('/api/v1/testplans', { method: 'POST', body: JSON.stringify(body) }),
  updateTestPlan: (id: number, body: { name: string; description?: string; version_id?: number; parent_id?: number; plan_type?: string; extra_link?: string; is_active?: boolean }) =>
    request<TestPlan>(`/api/v1/testplans/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteTestPlan: (id: number) => request<void>(`/api/v1/testplans/${id}`, { method: 'DELETE' }),
  addTestPlanCases: (planId: number, caseIds: number[]) =>
    request<{ plan_id: number; added: number }>(`/api/v1/testplans/${planId}/cases`, {
      method: 'POST',
      body: JSON.stringify({ case_ids: caseIds }),
    }),
  removeTestPlanCase: (planId: number, caseId: number) =>
    request<void>(`/api/v1/testplans/${planId}/cases/${caseId}`, { method: 'DELETE' }),
  reorderTestPlanCases: (planId: number, caseIds: number[]) =>
    request<{ ok: boolean }>(`/api/v1/testplans/${planId}/cases/order`, {
      method: 'PUT',
      body: JSON.stringify({ case_ids: caseIds }),
    }),
  cloneTestPlan: (planId: number, name: string) =>
    request<TestPlan>(`/api/v1/testplans/${planId}/clone`, {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  listTestRuns: (params?: {
    product_id?: number
    test_plan_id?: number
    build_id?: number
    status?: 'running' | 'stopped'
    q?: string
    limit?: number
    offset?: number
  }) => {
    const qs = new URLSearchParams()
    if (params?.product_id) qs.set('product_id', String(params.product_id))
    if (params?.test_plan_id) qs.set('test_plan_id', String(params.test_plan_id))
    if (params?.build_id) qs.set('build_id', String(params.build_id))
    if (params?.status) qs.set('status', params.status)
    if (params?.q) qs.set('q', params.q)
    if (params?.limit !== undefined) qs.set('limit', String(params.limit))
    if (params?.offset !== undefined) qs.set('offset', String(params.offset))
    const suffix = qs.toString() ? `?${qs.toString()}` : ''
    return request<{ items: TestRun[]; total?: number }>(`/api/v1/testruns${suffix}`)
  },
  createTestRun: (body: { test_plan_id: number; build_id: number; environment_id?: number; default_tester_id?: number; summary: string; notes?: string }) =>
    request<TestRun>('/api/v1/testruns', { method: 'POST', body: JSON.stringify(body) }),
  updateTestRun: (id: number, body: { summary: string; notes?: string; build_id?: number; environment_id?: number; default_tester_id?: number }) =>
    request<TestRun>(`/api/v1/testruns/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  addTestRunCases: (runId: number, caseIds: number[], environmentId?: number) =>
    request<{ added: number }>(`/api/v1/testruns/${runId}/cases`, {
      method: 'POST',
      body: JSON.stringify({ case_ids: caseIds, environment_id: environmentId }),
    }),
  deleteTestRunExecution: (runId: number, execId: number) =>
    request<void>(`/api/v1/testruns/${runId}/executions/${execId}`, { method: 'DELETE' }),
  stopTestRun: (runId: number) =>
    request<{ stopped: boolean }>(`/api/v1/testruns/${runId}/stop`, { method: 'POST' }),
  deleteTestRun: (runId: number) =>
    request<void>(`/api/v1/testruns/${runId}`, { method: 'DELETE' }),
  downloadRunExport: (runId: number, format: 'csv' | 'html') =>
    downloadAuthenticated(
      `/api/v1/testruns/${runId}/export?format=${format}`,
      `testrun-${runId}.${format}`,
      format === 'html',
    ),
  listExecutions: (runId: number) => request<{ items: TestExecution[] }>(`/api/v1/testruns/${runId}/executions`),
  updateExecution: (id: number, body: { status?: string; comment?: string; assignee_id?: number; environment_id?: number }) =>
    request<{ id: number; status: string; comment?: string; assignee_id?: number; tested_by_id?: number; environment_id?: number }>(`/api/v1/executions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  bulkUpdateExecutions: (body: { ids: number[]; status?: string; assignee_id?: number }) =>
    request<{ updated: number }>('/api/v1/executions/bulk', { method: 'PATCH', body: JSON.stringify(body) }),
  syncExecutionCase: (id: number) =>
    request<TestExecution>(`/api/v1/executions/${id}/sync-case`, { method: 'POST' }),
  listLinks: (entityType: 'test_run' | 'test_execution', entityId: number) =>
    request<{ items: LinkRefItem[] }>(`/api/v1/links?entity_type=${entityType}&entity_id=${entityId}`),
  addLink: (entityType: 'test_run' | 'test_execution', entityId: number, body: { name: string; url: string }) =>
    request<LinkRefItem>(`/api/v1/links`, { method: 'POST', body: JSON.stringify({ entity_type: entityType, entity_id: entityId, ...body }) }),
  deleteLink: (id: number) => request<void>(`/api/v1/links/${id}`, { method: 'DELETE' }),
  listExecutionLinks: (id: number) =>
    request<{ items: ExecutionLinkItem[] }>(`/api/v1/executions/${id}/links`),
  addExecutionLink: (id: number, body: { name: string; url: string }) =>
    request<ExecutionLinkItem>(`/api/v1/executions/${id}/links`, { method: 'POST', body: JSON.stringify(body) }),
  deleteExecutionLink: (id: number, linkId: number) =>
    request<void>(`/api/v1/executions/${id}/links/${linkId}`, { method: 'DELETE' }),

  listAttachments: (entityType: 'test_case' | 'test_execution', entityId: number) =>
    request<{ items: AttachmentItem[] }>(`/api/v1/attachments?entity_type=${entityType}&entity_id=${entityId}`),
  downloadAttachment: (id: number, filename: string) =>
    downloadAuthenticated(`/api/v1/attachments/${id}/download`, filename),
  deleteAttachment: (id: number) =>
    request<void>(`/api/v1/attachments/${id}`, { method: 'DELETE' }),
  listCustomFieldDefs: (entityType?: string) =>
    request<{ items: CustomFieldDef[] }>(`/api/v1/custom-fields${entityType ? `?entity_type=${entityType}` : ''}`),
  createCustomFieldDef: (body: { entity_type: string; name: string; field_type?: string; options?: string; required?: boolean }) =>
    request<CustomFieldDef>('/api/v1/custom-fields', { method: 'POST', body: JSON.stringify(body) }),
  updateCustomFieldDef: (id: number, body: { entity_type: string; name: string; field_type: string; options: string; required: boolean }) =>
    request<CustomFieldDef>(`/api/v1/custom-fields/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteCustomFieldDef: (id: number) => request<void>(`/api/v1/custom-fields/${id}`, { method: 'DELETE' }),
  getEntityCustomFields: (entityType: string, entityId: number) =>
    request<{ items: CustomFieldWithValue[] }>(`/api/v1/${entityType}/${entityId}/custom-fields`),
  setEntityCustomField: (entityType: string, entityId: number, fieldId: number, value: string) =>
    request<CustomFieldValue>(`/api/v1/${entityType}/${entityId}/custom-fields/${fieldId}`, { method: 'PUT', body: JSON.stringify({ value }) }),
  listCaseDatasets: (caseId: number) =>
    request<{ items: Dataset[] }>(`/api/v1/testcases/${caseId}/datasets`),
  createDataset: (caseId: number, body: { name: string; params?: string }) =>
    request<Dataset>(`/api/v1/testcases/${caseId}/datasets`, { method: 'POST', body: JSON.stringify(body) }),
  updateDataset: (id: number, body: { name: string; params?: string }) =>
    request<Dataset>(`/api/v1/datasets/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteDataset: (id: number) => request<void>(`/api/v1/datasets/${id}`, { method: 'DELETE' }),
  expandDatasets: (runId: number, caseId: number) =>
    request<{ created: number }>(`/api/v1/testruns/${runId}/cases/${caseId}/expand-datasets`, { method: 'POST' }),
  listExecutionScreenshots: (executionId: number) =>
    request<{ items: AttachmentItem[] }>(`/api/v1/executions/${executionId}/screenshots`),
  fetchAttachmentBlob: async (id: number): Promise<string> => {
    const token = getAuthToken()
    const res = await fetch(`/api/v1/attachments/${id}/download`, {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const blob = await res.blob()
    return URL.createObjectURL(blob)
  },
  getTelemetrySummary: (params?: TelemetryFilterParams) =>
    request<TelemetrySummary>(`/api/v1/telemetry/summary${buildTelemetryQuery(params)}`),
  getExecutionTrends: (params?: TelemetryFilterParams) =>
    request<{ items: TrendPoint[] }>(`/api/v1/telemetry/execution-trends${buildTelemetryQuery(params)}`),
  getStatusMatrix: (params?: TelemetryFilterParams) =>
    request<{ items: MatrixRow[] }>(`/api/v1/telemetry/status-matrix${buildTelemetryQuery(params)}`),
  getTestCaseHealth: (params?: TelemetryFilterParams) =>
    request<{ items: CaseHealth[] }>(`/api/v1/telemetry/test-case-health${buildTelemetryQuery(params)}`),
  getTelemetryBreakdown: (params?: TelemetryFilterParams) =>
    request<TelemetryBreakdown>(`/api/v1/telemetry/breakdown${buildTelemetryQuery(params)}`),
  getExecutionDashboard: (params?: TelemetryFilterParams) =>
    request<ExecutionDashboard>(`/api/v1/telemetry/execution-dashboard${buildTelemetryQuery(params)}`),
  getDashboard: () => request<DashboardSnapshot>('/api/v1/dashboard'),
  uploadAttachment: async (entityType: 'test_case' | 'test_execution', entityId: number, file: File) => {
    const form = new FormData()
    form.append('entity_type', entityType)
    form.append('entity_id', String(entityId))
    form.append('file', file)
    const token = getAuthToken()
    const res = await fetch('/api/v1/attachments', {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
      body: form,
    })
    if (!res.ok) {
      throw new Error((await res.text()) || `HTTP ${res.status}`)
    }
    return res.json() as Promise<AttachmentItem>
  },

  listExecutionBugs: (executionId: number) =>
    request<{ items: BugItem[] }>(`/api/v1/executions/${executionId}/bugs`),
  linkExecutionBug: (executionId: number, body: { external_key: string; url: string; title?: string }) =>
    request<BugItem>(`/api/v1/executions/${executionId}/bugs`, { method: 'POST', body: JSON.stringify(body) }),
  deleteTestCase: (id: number) =>
    request<void>(`/api/v1/testcases/${id}`, { method: 'DELETE' }),
  listTestCaseRevisions: (id: number) =>
    request<{ items: RevisionItem[] }>(`/api/v1/testcases/${id}/revisions`),
  listComments: (entityType: 'test_case' | 'test_execution', entityId: number) =>
    request<{ items: CommentItem[] }>(`/api/v1/comments?entity_type=${entityType}&entity_id=${entityId}`),
  createComment: (entityType: 'test_case' | 'test_execution', entityId: number, body: string) =>
    request<CommentItem>('/api/v1/comments', {
      method: 'POST',
      body: JSON.stringify({ entity_type: entityType, entity_id: entityId, body }),
    }),
  importJUnitXML: (body: { test_plan_id: number; build_id: number; summary: string; xml: string; auto_create_cases?: boolean }) =>
    request<{ test_run: TestRun; results: { name: string; status: string }[]; created_cases: number }>('/api/v1/testruns/import/junit', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  listComponents: (productId: number) =>
    request<{ items: ComponentItem[] }>(`/api/v1/products/${productId}/components`),
  createComponent: (productId: number, name: string, description?: string) =>
    request<ComponentItem>(`/api/v1/products/${productId}/components`, {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    }),
  addProductMember: (productId: number, body: { user_id: number; role: 'viewer' | 'tester' | 'maintainer' }) =>
    request<{ ok: boolean }>(`/api/v1/products/${productId}/members`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  listTags: () => request<{ items: TagItem[] }>('/api/v1/tags'),
  createTag: (name: string) => request<TagItem>('/api/v1/tags', { method: 'POST', body: JSON.stringify({ name }) }),
  listEntityTags: (entity: 'testcases' | 'testplans' | 'testruns', id: number) =>
    request<{ items: TagItem[] }>(`/api/v1/${entity}/${id}/tags`),
  attachEntityTag: (entity: 'testcases' | 'testplans' | 'testruns', id: number, tagId: number) =>
    request<{ ok: boolean }>(`/api/v1/${entity}/${id}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag_id: tagId }),
    }),
  listTestCaseComponents: (caseId: number) =>
    request<{ items: ComponentItem[] }>(`/api/v1/testcases/${caseId}/components`),
  attachTestCaseComponent: (caseId: number, componentId: number) =>
    request<{ ok: boolean }>(`/api/v1/testcases/${caseId}/components`, {
      method: 'POST',
      body: JSON.stringify({ component_id: componentId }),
    }),

  listIssueTrackers: () => request<{ items: IssueTrackerItem[] }>('/api/v1/issue-trackers'),
  createIssueTracker: (body: { name: string; provider: 'github' | 'gitlab' | 'jira' | 'bugzilla' | 'azure'; base_url: string; api_username?: string }) =>
    request<IssueTrackerItem>('/api/v1/issue-trackers', { method: 'POST', body: JSON.stringify(body) }),
  listWebhooks: () => request<{ items: WebhookItem[] }>('/api/v1/webhooks'),
  createWebhook: (body: { name: string; url: string; secret?: string; events: string[]; is_active?: boolean }) =>
    request<WebhookItem>('/api/v1/webhooks', { method: 'POST', body: JSON.stringify(body) }),
  updateWebhook: (id: number, body: { name: string; url: string; secret?: string; secret_clear?: boolean; events: string[]; is_active?: boolean }) =>
    request<WebhookItem>(`/api/v1/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteWebhook: (id: number) =>
    request<{ deleted: boolean }>(`/api/v1/webhooks/${id}`, { method: 'DELETE' }),
  testWebhook: (id: number) =>
    request<WebhookTestResult>(`/api/v1/webhooks/${id}/test`, { method: 'POST' }),
  reportExecutionBug: (
    executionId: number,
    body: { tracker_id: number; api_token?: string; project_key?: string; title: string; body?: string },
  ) => request<BugItem>(`/api/v1/executions/${executionId}/bugs/report`, { method: 'POST', body: JSON.stringify(body) }),
  syncExecutionBug: (executionId: number, bugId: number, apiToken?: string) =>
    request<BugItem>(`/api/v1/executions/${executionId}/bugs/${bugId}/sync`, {
      method: 'POST',
      body: JSON.stringify({ api_token: apiToken || '' }),
    }),
  commentExecutionBug: (executionId: number, bugId: number, body: { body: string; api_token?: string }) =>
    request<{ ok: boolean }>(`/api/v1/executions/${executionId}/bugs/${bugId}/comment`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  listInternalBugs: (params?: { product_id?: number; status?: string; severity?: string; test_execution_id?: number; q?: string }) => {
    const qs = new URLSearchParams()
    if (params?.product_id) qs.set('product_id', String(params.product_id))
    if (params?.status) qs.set('status', params.status)
    if (params?.severity) qs.set('severity', params.severity)
    if (params?.test_execution_id) qs.set('test_execution_id', String(params.test_execution_id))
    if (params?.q) qs.set('q', params.q)
    const suffix = qs.toString() ? `?${qs.toString()}` : ''
    return request<{ items: InternalBug[] }>(`/api/v1/bugs${suffix}`)
  },
  createInternalBug: (body: {
    product_id: number
    build_id?: number
    test_execution_id?: number
    summary: string
    description?: string
    severity?: InternalBug['severity']
    assignee_id?: number
  }) => request<InternalBug>('/api/v1/bugs', { method: 'POST', body: JSON.stringify(body) }),
  updateInternalBug: (
    id: number,
    body: { summary: string; description?: string; status?: InternalBug['status']; severity?: InternalBug['severity']; assignee_id?: number },
  ) => request<InternalBug>(`/api/v1/bugs/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
  deleteInternalBug: (id: number) => request<void>(`/api/v1/bugs/${id}`, { method: 'DELETE' }),

  listEnvironments: () => request<{ items: EnvironmentItem[] }>('/api/v1/environments'),
  createEnvironment: (body: { name: string; description?: string; properties?: Record<string, string> }) =>
    request<EnvironmentItem>('/api/v1/environments', { method: 'POST', body: JSON.stringify(body) }),

  listTemplates: () => request<{ items: TemplateItem[] }>('/api/v1/templates'),
  createTemplate: (body: { name: string; body: string }) =>
    request<TemplateItem>('/api/v1/templates', { method: 'POST', body: JSON.stringify(body) }),

  listPriorities: () => request<{ items: PriorityItem[] }>('/api/v1/priorities'),
  createPriority: (value: string) =>
    request<PriorityItem>('/api/v1/priorities', { method: 'POST', body: JSON.stringify({ value }) }),

  listExecutionStatuses: () => request<{ items: ExecutionStatusItem[] }>('/api/v1/execution-statuses'),
  createExecutionStatus: (body: { name: string; color?: string; weight?: number }) =>
    request<ExecutionStatusItem>('/api/v1/execution-statuses', { method: 'POST', body: JSON.stringify(body) }),

  getEmailSettings: (entityType: 'test_case' | 'test_plan', entityId: number) =>
    request<EmailSettingsItem>(`/api/v1/email-settings?entity_type=${entityType}&entity_id=${entityId}`),
  upsertEmailSettings: (body: EmailSettingsItem) =>
    request<EmailSettingsItem>('/api/v1/email-settings', { method: 'PUT', body: JSON.stringify(body) }),
  sendTestWebhook: (message: string) =>
    request<{ sent: boolean }>('/api/v1/notifications/test', { method: 'POST', body: JSON.stringify({ message }) }),
  sendTestEmail: (body: { to: string[]; subject: string; body: string }) =>
    request<{ sent: boolean }>('/api/v1/notifications/email-test', { method: 'POST', body: JSON.stringify(body) }),
  listNotifications: (params?: { unread?: boolean; limit?: number; offset?: number }) => {
    const q = new URLSearchParams()
    if (params?.unread) q.set('unread', 'true')
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.offset) q.set('offset', String(params.offset))
    const qs = q.toString()
    return request<{ items: NotificationItem[]; total: number; unread_count: number }>(
      `/api/v1/notifications${qs ? `?${qs}` : ''}`
    )
  },
  readNotification: (id: number) =>
    request<{ read: boolean }>(`/api/v1/notifications/${id}/read`, { method: 'POST' }),
  readAllNotifications: () =>
    request<{ updated: number }>('/api/v1/notifications/read-all', { method: 'POST' }),
  getMyNotificationSettings: () =>
    request<{ events: NotificationEventPreference[] }>('/api/v1/me/notification-settings'),
  putMyNotificationSettings: (events: NotificationEventPreference[]) =>
    request<{ events: NotificationEventPreference[] }>('/api/v1/me/notification-settings', {
      method: 'PUT',
      body: JSON.stringify({ events }),
    }),
}

// --- Configurable menu (MENU_ITEMS / HELP_MENU_ITEMS) ---

export interface MenuItem {
  label: string
  url?: string
  icon?: string
}

export interface MenuGroup {
  label: string
  items: (MenuItem | MenuGroup)[]
}

export interface MenuResponse {
  main: MenuGroup[]
  help: MenuItem[]
}

export const menuApi = {
  get: () => request<MenuResponse>('/api/v1/menu'),
}
