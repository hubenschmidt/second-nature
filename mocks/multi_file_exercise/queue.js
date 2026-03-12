const { PRIORITIES } = require("./job");

const createQueue = () => {
  const buckets = {
    high: [],
    normal: [],
    low: [],
  };

  const enqueue = (job) => {
    // Add job to the correct priority bucket
  };

  const dequeue = () => {
    // Return the next highest-priority job, or null if empty
  };

  const size = () => {
    // Return total number of pending jobs across all buckets
  };

  return { enqueue, dequeue, size };
};

module.exports = { createQueue };
