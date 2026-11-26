# Kiwit TCMS — XML-RPC / JSON-RPC API

Kiwit udostępnia pełne API zgodne z [Kiwi TCMS](https://kiwitcms.org) — klient `tcms-api` (Python) działa bez zmian.

## Endpointy

| Protokół | Endpoint | Content-Type |
|----------|----------|--------------|
| XML-RPC  | `POST /xml-rpc/`  | `text/xml` |
| JSON-RPC 2.0 | `POST /json-rpc/` | `application/json` |

## Autoryzacja

1. `Auth.login(username, password)` → zwraca token / ustawia cookie sesji
2. Kolejne wywołania z nagłówkiem `Authorization: Bearer <token>` lub cookie `kiwit_session`

## Dostępne metody (~90)

### Auth / Meta
- `Auth.login`, `Auth.logout`
- `KiwiTCMS.version`
- `Markdown.render`

### TestCase
`create`, `filter`, `update`, `remove`, `history`, `sortkeys`, `add_tag`, `remove_tag`, `add_component`, `remove_component`, `add_comment`, `remove_comment`, `comments`, `list_attachments`, `add_attachment`, `properties`, `add_property`, `remove_property`, `add_notification_cc`, `remove_notification_cc`, `get_notification_cc`

### TestPlan
`create`, `filter`, `update`, `remove`, `clone`, `tree`, `add_tag`, `remove_tag`, `add_case`, `remove_case`, `update_case_order`, `list_attachments`, `add_attachment`, `sortkeys`, `get_cases`

### TestRun
`create`, `filter`, `update`, `remove`, `add_case`, `remove_case`, `get_cases`, `add_tag`, `remove_tag`, `add_cc`, `remove_cc`, `get_cc`, `properties`, `add_property`, `list_attachments`, `add_attachment`, `add_link`, `get_links`

### TestExecution
`create`, `filter`, `update`, `remove`, `history`, `add_comment`, `remove_comment`, `get_comments`, `add_link`, `remove_link`, `get_links`, `properties`, `add_property`, `remove_property`, `add_tag`, `remove_tag`, `list_attachments`, `count_attachments`, `add_attachment`

### Słowniki (dictionaries)
- **Product**: `filter`, `create`
- **Version**: `filter`, `create`
- **Build**: `filter`, `create`, `update`
- **Category**: `filter`, `create`
- **Component**: `filter`, `create`, `update`
- **Tag**: `filter`, `create`
- **Priority**: `filter`, `create`
- **TestCaseStatus**: `filter`, `create`
- **TestExecutionStatus**: `filter`, `create`
- **PlanType**: `filter`, `create`
- **Classification**: `filter`, `create`
- **Template**: `filter`, `create`
- **Environment**: `filter`, `create`, `properties`, `add_property`, `remove_property`
- **BugTracker**: `filter`, `create`

### Bug
- `Bug.report`, `Bug.details`, `Bug.filter`

### User / Group
- `User.filter`, `User.update`, `User.deactivate`, `User.join_group`, `User.add_attachment`
- `Group.filter`, `Group.create`, `Group.users`, `Group.permissions`

### Inne
- `Attachment.remove_attachment` (alias `Attachment.remove`)
- `Search.query`
- `Telemetry.breakdown`, `Telemetry.status_matrix`, `Telemetry.execution_trends`, `Telemetry.test_case_health`

## Przykład użycia z tcms-api

```python
from tcms_api import TCMS

api = TCMS(url="http://localhost:8080", username="admin", password="admin")
print(api.exec.TestCase.filter({"plan": 1}))
```

## Field lookups (zaawansowane filtrowanie)

Wszystkie metody `.filter()` przyjmują słownik z Django-style field lookups:

| Lookup | Przykład | SQL |
|--------|----------|-----|
| exact (default) | `{"id": 5}` | `id = 5` |
| `__startswith` | `{"summary__startswith": "test"}` | `summary LIKE 'test%'` |
| `__endswith` | `{"name__endswith": "api"}` | `name LIKE '%api'` |
| `__contains` | `{"summary__contains": "login"}` | `summary LIKE '%login%'` |
| `__icontains` | `{"email__icontains": "@x.com"}` | `LOWER(email) LIKE LOWER('%@x.com%')` |
| `__gt` / `__gte` | `{"id__gte": 100}` | `id >= 100` |
| `__lt` / `__lte` | `{"id__lt": 50}` | `id < 50` |
| `__in` | `{"id__in": [1,2,3]}` | `id IN (1,2,3)` |
| `__isnull` | `{"stopped_at__isnull": True}` | `stopped_at IS NULL` |
| `__range` | `{"id__range": [10,20]}` | `id BETWEEN 10 AND 20` |
| relacje | `{"product__name__startswith": "My"}` | `JOIN products ... WHERE p.name LIKE 'My%'` |

```python
# Przykłady:
api.exec.TestCase.filter({"summary__startswith": "test", "priority": "high"})
api.exec.TestRun.filter({"id__gte": 100, "plan__product__name__icontains": "demo"})
```

## InitDB — pierwsza inicjalizacja bazy

| Endpoint | Metoda | Opis |
|----------|--------|------|
| `/init-db/` | GET | Jeśli baza ma użytkowników → 302 do `/`. Inaczej JSON `{"initialized": false}` |
| `/init-db/` | POST | Tworzy pierwszego admina + seeduje słowniki. Body opcjonalny: `{"admin_username":"...","admin_password":"...","admin_email":"..."}` lub env `KIWIT_ADMIN_*` |
| `/api/v1/admin/reset-db` | POST | Admin-only. Usuwa wszystkie dane (oprócz schematu), re-seeduje słowniki |
