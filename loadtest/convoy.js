import http from "k6/http";
import { check, sleep } from "k6";
import { randomItem } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const baseUrl = `${__ENV.BASE_URL}/api/v1`
const apiKey = __ENV.API_KEY
const appId = __ENV.APP_ID;
const params = {
    headers : {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey}`
    }
}

const eventFailRate = new Rate('failed events')

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
    { duration: '60s', target: 100 }, // simulate ramp-up of traffic from 1 to 100 users over 60s
    { duration: '60s', target: 100 }, // stay at 100 users for 60s
  ],
  thresholds: {
    'failed events': ['rate<0.1'],
    'http_req_duration': ['p(99)<6000'], // 99% of requests must complete below 2s
  },
};

export default function () {
  let eventBody = JSON.stringify(generateEventPayload(appId))
    const eventResponse = http.post(`${baseUrl}/events`, eventBody, params)

    check(eventResponse, {
        "create event status is 201": (r) => r.status === 201,
    })

    eventFailRate.add(eventResponse.status !== 201);
    sleep(1);
}
