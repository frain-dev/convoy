import http from "k6/http";
import { check, sleep } from "k6";
import { Trend, Rate } from "k6/metrics";
import { randomItem } from "https://jslib.k6.io/k6-utils/1.2.0/index.js";

const baseUrl = `${__ENV.BASE_URL}/api/v1`;
const apiKey = __ENV.API_KEY;
const appId = __ENV.APP_ID;
const params = {
    headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiKey}`,
    },
};

const listEventsErrorRate = new Rate("List_Events_errors");
const createEventErrorRate = new Rate("Create_Event_error");
const ListEventsTrend = new Trend("List_Events");
const createEventsTrend = new Trend("Create_Events");

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
        { duration: "60s", target: 20 }, // simulate ramp-up of traffic from 1 to 20 users over 60s
        { duration: "60s", target: 20 }, // stay at 20 users for 60s
    ],
    thresholds: {
        "List_Events": ["p(95) < 3000"], //95% of requests must complete below 3s
        "Create_Events": ["p(99) < 3000"], //99% of requests must complete below 3s
        "List_Events_errors": ["rate<0.1"], // error rate must be less than 10%
        "Create_Event_error": ["rate<0.1"], // error rate must be less than 10%
        http_req_duration: ["p(99)<6000"], // 99% of requests must complete below 6s
    },
};

export default function () {
    let eventBody = JSON.stringify(generateEventPayload(appId));
    const listEventsUrl = `${baseUrl}/events?appId=${appId}`;
    const createEventUrl = `${baseUrl}/events`;
  
    const requests = {
        "List_Events": {
            method: "GET",
            url: listEventsUrl,
            params: params,
        },
        "Create_Events": {
            method: "POST",
            url: createEventUrl,
            params: params,
            body: eventBody,
        },
    };

    const responses = http.batch(requests)
    const listResp = responses['List_Events'];
    const createResp = responses['Create_Events'];

    check(listResp, {
      'list_events': (r) => r.status === 200,
    }) || listEventsErrorRate.add(1);

    ListEventsTrend.add(listResp.timings.duration)

    check(createResp, {
      'event_created': (r) => r.status === 201,
    }) || createEventErrorRate.add(1)

    createEventsTrend.add(createResp.timings.duration)
    
    sleep(1);
}
