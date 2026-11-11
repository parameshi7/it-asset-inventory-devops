import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL =
    __ENV.BASE_URL || 'http://127.0.0.1:8081';

const ASSETS_ENDPOINT =
    `${BASE_URL}/api/v1/assets`;

export const options = {
    scenarios: {
        inventory_load: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '20s', target: 100 },
                { duration: '20s', target: 300 },
                { duration: '20s', target: 500 },
                { duration: '60s', target: 500 },
                { duration: '10s', target: 0 },
            ],
            gracefulRampDown: '30s',
        },
    },

    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.05'],
        checks: ['rate>0.95'],
    },
};

export default function () {
    const response = http.get(ASSETS_ENDPOINT, {
        timeout: '30s',
    });

    check(response, {
        'assets endpoint returns HTTP 200':
            (r) => r.status === 200,
    });

    sleep(1);
}
