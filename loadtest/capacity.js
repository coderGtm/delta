import http from 'k6/http';
import { check } from 'k6';

import { mintAccessToken, loadConfig, attendanceListPath } from './config.js';

const cfg = loadConfig();
const maxVus = __ENV.MAX_VUS ? Number(__ENV.MAX_VUS) : 60;

// Capacity test: ramps up virtual users against GET /outlets/{id}/attendance.
// This endpoint is deliberately chosen because it is NOT rate-limited, so the
// measured throughput reflects app + JVM + Hikari pool + Postgres capacity.
//
// Run with, e.g.:
//   k6 run loadtest/capacity.js
//   k6 run -e MAX_VUS=100 loadtest/capacity.js
// Watch memory during the run with: docker stats delta-app
export const options = {
	scenarios: {
		ramp_up: {
			executor: 'ramping-vus',
			startVUs: 1,
			stages: [
				{ duration: '60s', target: Math.max(1, Math.round(maxVus * 0.25)) },
				{ duration: '30s', target: Math.max(1, Math.round(maxVus * 0.25)) },
				{ duration: '60s', target: maxVus },
				{ duration: '60s', target: maxVus },
			],
			gracefulRampDown: '30s',
		},
	},
	thresholds: {
		http_req_failed: ['rate<0.02'],
		http_req_duration: ['p(95)<1000'],
	},
};

export default function () {
	const token = mintAccessToken(cfg.jwtSecret, cfg.ownerId);
	const res = http.get(`${cfg.baseUrl}${attendanceListPath(cfg.outletId)}?size=20`, {
		headers: { Authorization: `Bearer ${token}` },
	});
	check(res, {
		'attendance list returns 200': (r) => r.status === 200,
	});
}
