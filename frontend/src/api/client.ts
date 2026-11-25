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

export interface TestCase {
  id: number
  product_id: number
  summary: string
  preconditions?: string
  format: 'steps' | 'gherkin'
  gherkin_body?: string
  priority: 'low' | 'medium' | 'high' | 'critical'
  status: 'draft' | 'confirmed' | 'disabled'
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

async function request<T>(url: string, init?: RequestInit): Promise<T> {
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
  return res.json() as Promise<T>
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
  name: string
  description?: string
  is_active: boolean
  created_at: string
}

export interface TestRun {
  id: number
  test_plan_id: number
  build_id: number
  summary: string
  notes?: string
  started_at: string
}

export interface TestExecution {
  id: number
  test_run_id: number
  test_case_id: number
  summary: string
  status: 'idle' | 'running' | 'passed' | 'failed' | 'blocked' | 'skipped'
  comment?: string
}

export interface User {
  id: number
  email: string
  username: string
  role: string
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
  listProducts: () => request<{ items: Product[] }>('/api/v1/products'),
  createProduct: (body: { name: string; description?: string }) =>
    request<Product>('/api/v1/products', { method: 'POST', body: JSON.stringify(body) }),
  listVersions: (productId: number) => request<{ items: Version[] }>(`/api/v1/products/${productId}/versions`),
  createVersion: (productId: number, value: string) =>
    request<Version>(`/api/v1/products/${productId}/versions`, { method: 'POST', body: JSON.stringify({ value }) }),
  listBuilds: (versionId: number) => request<{ items: Build[] }>(`/api/v1/versions/${versionId}/builds`),
  createBuild: (versionId: number, name: string) =>
    request<Build>(`/api/v1/versions/${versionId}/builds`, { method: 'POST', body: JSON.stringify({ name }) }),

  listTestCases: () => request<{ items: TestCase[] }>('/api/v1/testcases'),
  createTestCase: (body: Omit<TestCase, 'id' | 'created_at' | 'updated_at'>) =>
    request<TestCase>('/api/v1/testcases', { method: 'POST', body: JSON.stringify(body) }),
  updateTestCase: (id: number, body: Omit<TestCase, 'id' | 'created_at' | 'updated_at'>) =>
    request<TestCase>(`/api/v1/testcases/${id}`, { method: 'PUT', body: JSON.stringify(body) }),

  listTestPlans: () => request<{ items: TestPlan[] }>('/api/v1/testplans'),
  createTestPlan: (body: { product_id: number; version_id: number; name: string; description?: string }) =>
    request<TestPlan>('/api/v1/testplans', { method: 'POST', body: JSON.stringify(body) }),
  addTestPlanCases: (planId: number, caseIds: number[]) =>
    request<{ plan_id: number; added: number }>(`/api/v1/testplans/${planId}/cases`, {
      method: 'POST',
      body: JSON.stringify({ case_ids: caseIds }),
    }),

  listTestRuns: () => request<{ items: TestRun[] }>('/api/v1/testruns'),
  createTestRun: (body: { test_plan_id: number; build_id: number; summary: string; notes?: string }) =>
    request<TestRun>('/api/v1/testruns', { method: 'POST', body: JSON.stringify(body) }),
  listExecutions: (runId: number) => request<{ items: TestExecution[] }>(`/api/v1/testruns/${runId}/executions`),
  updateExecution: (id: number, body: { status: TestExecution['status']; comment?: string }) =>
    request<{ id: number; status: string; comment?: string }>(`/api/v1/executions/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),

  listAttachments: (entityType: 'test_case' | 'test_execution', entityId: number) =>
    request<{ items: AttachmentItem[] }>(`/api/v1/attachments?entity_type=${entityType}&entity_id=${entityId}`),
  getTelemetrySummary: () => request<TelemetrySummary>('/api/v1/telemetry/summary'),
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
}
