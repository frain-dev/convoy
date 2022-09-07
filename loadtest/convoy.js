import http from "k6/http";
import { check, sleep } from "k6";
import { randomItem } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const names = ["John", "Jane", "Bert", "Ed"];
const emails = [
  "John@gmail.com",
  "Jane@amazon.com",
  "Bert@yahoo.com",
  "Ed@hotmail.com",
];
export const generateEventPayload = (appId) => ({
  app_id: appId,
  data: {
    player_name: randomItem(names),
    email: randomItem(emails),
  },
  event_type: `${randomItem(names)}.${randomItem(names)}`.toLowerCase(),
});

export let options = {
  noConnectionReuse: true,
  stages: [
    { duration: "20s", target: 1 }, // simulate ramp-up of traffic from 1 to 1000 users over 1 minute.
    { duration: "20s", target: 10 }, // stay at 1000 users for 1 minute
    { duration: "20s", target: 0 }, // ramp-down to 0 users in 1 minute
  ],
  thresholds: {
    http_req_duration: ["p(99)<6000"], // 99% of requests must complete below 6.0s
    "successfully sent event": ["p(99)<6000"], // 99% of requests must complete below 6.0s
  },
};

export default function () {
  const headers = { "content-type": "application/json" };
  const baseUrl = "http://localhost:5005";
  const groupId = "12097532-bd40-4cd9-bdfe-0a2dfb08d86e";
  const appId = "d64eac7b-7d3f-464b-84d0-f3938d36b3af";

  // create endpoint
  const response = http.post(
    `${baseUrl}/api/v1/events?groupId=${groupId}`,
    JSON.stringify(generateEventPayload(appId)),
    { headers }
  );

  check(response, {
    "create endpoint status is 201": (r) => r.status === 201,
  });

  sleep(1);
}
