import http from 'k6/http';
import { check } from 'k6';

import { mintAccessToken, loadConfig, attendanceListPath } from './config.js';

const cfg = loadConfig();

export const options = {
	vus: 2,
	iterations: 10,
	thresholds: {
		http_req_failed: ['rate<0.01'],
		http_req_duration: ['p(95)<500'],
	},
};

export default function () {
	const token = mintAccessToken(cfg.jwtSecret, cfg.ownerId);
	const res = http.get(`${cfg.baseUrl}${attendanceListPath(cfg.outletId)}`, {
		headers: { Authorization: `Bearer ${token}` },
	});
	check(res, {
		'attendance list returns 200': (r) => r.status === 200,
	});
}
