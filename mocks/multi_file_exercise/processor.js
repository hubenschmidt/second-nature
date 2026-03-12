const { createQueue } = require("./queue");

const createProcessor = () => {
  const queue = createQueue();
  const handlers = {};
  const completed = [];
  const failed = [];

  const register = (jobType, handlerFn) => {
    // Register a handler function for a given job type
  };

  const submit = (job) => {
    // Add a job to the queue
  };

  const processNext = () => {
    // Dequeue the next job, route to its handler, update status
    // Push to completed or failed accordingly
  };

  const drain = () => {
    // Process all remaining jobs in the queue
  };

  return { register, submit, processNext, drain, completed, failed, queue };
};

module.exports = { createProcessor };
