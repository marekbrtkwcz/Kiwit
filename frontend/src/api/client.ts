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
  format: 'steps' | 'gherkin'
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

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setAuthToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
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
  external_key: string
  url: string
  title?: string
  status: string
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
  provider: 'github' | 'gitlab' | 'jira'
  base_url: string
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
  listUsers: () => request<{ items: User[] }>('/api/v1/admin/users'),
  updateUser: (id: number, body: { role: 'viewer' | 'tester' | 'admin'; is_active: boolean }) =>
    request<User>(`/api/v1/admin/users/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),
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
  importJUnitXML: (body: { test_plan_id: number; build_id: number; summary: string; xml: string }) =>
    request<{ test_run: TestRun; results: { name: string; status: string }[] }>('/api/v1/testruns/import/junit', {
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
  createIssueTracker: (body: { name: string; provider: 'github' | 'gitlab' | 'jira'; base_url: string }) =>
    request<IssueTrackerItem>('/api/v1/issue-trackers', { method: 'POST', body: JSON.stringify(body) }),
  reportExecutionBug: (
    executionId: number,
    body: { tracker_id: number; api_token?: string; project_key?: string; title: string; body?: string },
  ) => request<BugItem>(`/api/v1/executions/${executionId}/bugs/report`, { method: 'POST', body: JSON.stringify(body) }),

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
}
