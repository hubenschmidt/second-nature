# Event-Driven Task Queue

You are building a lightweight task queue that processes jobs from multiple producers.
The system has three components that must work together:

1. **Job** — a unit of work with a type, payload, priority, and status
2. **Queue** — a priority queue that accepts jobs and yields them in priority order
3. **Processor** — consumes jobs from the queue, dispatches to the correct handler, and tracks results

## Requirements

- Jobs have three priority levels: `high`, `normal`, `low`
- The queue must yield highest-priority jobs first (FIFO within same priority)
- The processor must route jobs to the correct handler based on `job.type`
- If a handler throws, the job status should be set to `"failed"` with the error message
- Successfully processed jobs should be set to `"completed"` with the handler's return value

## Questions to consider

1. What happens if a job type has no registered handler?
2. How would you add retry logic for failed jobs?
3. How would you make this thread-safe for concurrent producers?
