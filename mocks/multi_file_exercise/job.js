const PRIORITIES = { high: 0, normal: 1, low: 2 };

const createJob = (type, payload, priority = "normal") => ({
  id: crypto.randomUUID(),
  type,
  payload,
  priority,
  status: "pending",
  result: null,
  error: null,
  createdAt: Date.now(),
});

module.exports = { PRIORITIES, createJob };
