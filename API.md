# API Documentation

## Base URL
```
http://localhost:8080/api/v1
```

## Common Headers
```
Content-Type: application/json
Authorization: Bearer <JWT_TOKEN>  # For protected endpoints
```

## Rate Limiting
- **Limit**: 100 requests per minute per IP address
- **Response when exceeded**: HTTP 429 Too Many Requests

---

## Authentication Service

### Register User
```http
POST /auth/register
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

**Response:**
```json
{
  "message": "user registered successfully"
}
```

---

### Login
```http
POST /auth/login
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "user-123",
    "email": "user@example.com"
  }
}
```

---

### Verify Token
```http
POST /auth/verify
```

**Headers:**
```
Authorization: <JWT_TOKEN>
```

**Response:**
```json
{
  "valid": true
}
```

---

## Calendar & Task Service

### List Events
```http
GET /calendar/events
```

**Response:**
```json
[
  {
    "id": "event-1",
    "user_id": "user-123",
    "title": "Team Meeting",
    "description": "Weekly sync meeting",
    "start_time": "2024-01-15T10:00:00Z",
    "end_time": "2024-01-15T11:00:00Z",
    "location": "Conference Room A",
    "created_at": "2024-01-10T08:00:00Z",
    "updated_at": "2024-01-10T08:00:00Z"
  }
]
```

---

### Create Event
```http
POST /calendar/events
```

**Request Body:**
```json
{
  "user_id": "user-123",
  "title": "Team Meeting",
  "description": "Weekly sync meeting",
  "start_time": "2024-01-15T10:00:00Z",
  "end_time": "2024-01-15T11:00:00Z",
  "location": "Conference Room A"
}
```

**Response:**
```json
{
  "id": "event-1",
  "user_id": "user-123",
  "title": "Team Meeting",
  "description": "Weekly sync meeting",
  "start_time": "2024-01-15T10:00:00Z",
  "end_time": "2024-01-15T11:00:00Z",
  "location": "Conference Room A",
  "created_at": "2024-01-10T08:00:00Z",
  "updated_at": "2024-01-10T08:00:00Z"
}
```

---

### Get Event
```http
GET /calendar/events/{id}
```

**Response:**
```json
{
  "id": "event-1"
}
```

---

### Update Event
```http
PUT /calendar/events/{id}
```

**Request Body:**
```json
{
  "title": "Updated Team Meeting",
  "description": "Updated description"
}
```

**Response:**
```json
{
  "id": "event-1",
  "title": "Updated Team Meeting",
  "description": "Updated description"
}
```

---

### Delete Event
```http
DELETE /calendar/events/{id}
```

**Response:**
```
HTTP 204 No Content
```

---

### List Tasks
```http
GET /calendar/tasks
```

**Response:**
```json
[
  {
    "id": "task-1",
    "user_id": "user-123",
    "title": "Complete project documentation",
    "description": "Write comprehensive docs",
    "due_date": "2024-01-20T17:00:00Z",
    "completed": false,
    "priority": "high",
    "created_at": "2024-01-10T08:00:00Z",
    "updated_at": "2024-01-10T08:00:00Z"
  }
]
```

---

### Create Task
```http
POST /calendar/tasks
```

**Request Body:**
```json
{
  "user_id": "user-123",
  "title": "Complete project documentation",
  "description": "Write comprehensive docs",
  "due_date": "2024-01-20T17:00:00Z",
  "priority": "high"
}
```

**Response:**
```json
{
  "id": "task-1",
  "user_id": "user-123",
  "title": "Complete project documentation",
  "description": "Write comprehensive docs",
  "due_date": "2024-01-20T17:00:00Z",
  "completed": false,
  "priority": "high",
  "created_at": "2024-01-10T08:00:00Z",
  "updated_at": "2024-01-10T08:00:00Z"
}
```

---

## Location Service

### Track Location
```http
POST /location/track
```

**Request Body:**
```json
{
  "user_id": "user-123",
  "latitude": 22.3193,
  "longitude": 114.1694,
  "address": "Hong Kong University of Science and Technology"
}
```

**Response:**
```json
{
  "message": "location tracked successfully"
}
```

---

### Get Location History
```http
GET /location/history?user_id=user-123
```

**Response:**
```json
[
  {
    "id": "loc-1",
    "user_id": "user-123",
    "latitude": 22.3193,
    "longitude": 114.1694,
    "address": "Hong Kong University of Science and Technology",
    "timestamp": "2024-01-10T10:00:00Z",
    "created_at": "2024-01-10T10:00:00Z"
  }
]
```

---

### Get Current Location
```http
GET /location/current?user_id=user-123
```

**Response:**
```json
{
  "user_id": "user-123",
  "latitude": 22.3193,
  "longitude": 114.1694
}
```

---

### Find Nearby
```http
GET /location/nearby?lat=22.3193&lng=114.1694
```

**Response:**
```json
[]
```

---

## Integration Service

### Sync Data
```http
POST /integration/sync
```

**Request Body:**
```json
{
  "source": "google-calendar",
  "target": "orbit-calendar",
  "data": {
    "events": []
  }
}
```

**Response:**
```json
{
  "message": "sync completed"
}
```

---

### Handle Webhook
```http
POST /integration/webhooks
```

**Request Body:**
```json
{
  "event": "calendar.updated",
  "data": {}
}
```

**Response:**
```json
{
  "message": "webhook processed"
}
```

---

### Connect External Service
```http
POST /integration/external/connect
```

**Request Body:**
```json
{
  "service": "google-calendar",
  "api_key": "your-api-key"
}
```

**Response:**
```json
{
  "message": "external service connected"
}
```

---

### Disconnect External Service
```http
POST /integration/external/disconnect
```

**Request Body:**
```json
{
  "service": "google-calendar"
}
```

**Response:**
```json
{
  "message": "external service disconnected"
}
```

---

### Get Integration Status
```http
GET /integration/external/status
```

**Response:**
```json
{
  "integrations": []
}
```

---

## Error Responses

### 400 Bad Request
```json
{
  "error": "invalid request"
}
```

### 401 Unauthorized
```json
{
  "error": "invalid token"
}
```

### 429 Too Many Requests
```json
{
  "error": "rate limit exceeded"
}
```

### 500 Internal Server Error
```json
{
  "error": "internal server error"
}
```
