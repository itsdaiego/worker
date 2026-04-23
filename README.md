# Go Backend Challenge: Concurrent Job Processor

## Overview

Build a REST API that accepts, processes, and persists background **jobs**.

Estimated time: **2–3 hours**

---

## The Task

Implement a job processing service with the following endpoints:

| Method | Path            | Description                                           |
|--------|-----------------|-------------------------------------------------------|
| POST   | /jobs           | Create a new job                                      |
| GET    | /jobs/{id}      | Get a job by ID                                       |
| GET    | /jobs           | List all jobs                                         |
| POST   | /jobs/batch     | Process all pending jobs and return a result summary  |

**Job shape (JSON):**
```json
{
  "id": "uuid",
  "type": "resize_image | send_email | generate_report",
  "payload": "any string",
  "status": "pending | processing | done | failed"
}
```

---

## Behavior

**POST /jobs**
- Accepts `type` and `payload`
- Returns the created job with a generated ID and `status: pending`
- Rejects invalid input with an appropriate error code and message

**POST /jobs/batch**
- Blocks until all pending jobs have finished processing
- Returns:
```json
{ "total": 10, "succeeded": 10, "failed": 0 }
```

**Job processing**:
- Each job takes ~300ms to complete
- Jobs transition through statuses as they are processed
- A job already being processed or finished must not be picked up again

---


## Getting Started

```bash
go mod init github.com/<you>/jobprocessor
go run .
```
