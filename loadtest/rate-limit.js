import http from 'k6/http';
import { check } from 'k6';

import { mintAccessToken, loadConfig, loginPath, refreshPath, attendanceListPath } from './config.js';

const cfg = loadConfig();

// Rate-limit boundary tests against RateLimitingFilter policies.
//
// Budgets are per policy window (1 minute) and key (IP or user id):
//   POST /api/v1/auth/login                  -> 10 per minute per IP
//   POST /api/v1/auth/refresh                -> 30 per minute per IP
//   POST /api/v1/outlets/*/attendance        -> 20 per minute per user
//
// NOTE: counters are in-memory in the app and reset after 1 minute, so wait
// at least a minute between runs of this script against the same app instance.
export const options = {
	scenarios: {
		login_boundary: {
			executor: 'per-vu-iterations',
			vus: 1,
			iterations: 14,
			maxDuration: '60s',
			exec: 'loginBoundary',
		},
		refresh_boundary: {
			executor: 'per-vu-iterations',
			vus: 1,
			iterations: 35,
			maxDuration: '60s',
			exec: 'refreshBoundary',
		},
		attendance_boundary: {
			executor: 'per-vu-iterations',
			vus: 1,
			iterations: 25,
			maxDuration: '60s',
			exec: 'attendanceBoundary',
		},
		login_ip_isolation: {
			executor: 'per-vu-iterations',
			vus: 2,
			iterations: 12,
			maxDuration: '60s',
			exec: 'loginIpIsolation',
		},
	},
	thresholds: {
		// 429s are the expected outcome here, so http_req_failed is not a useful
		// signal. The meaningful assertion is that every response matched its
		// rate-limit budget, which is validated by the per-request checks.
		checks: ['rate==1.0'],
	},
};

const fakeFirebaseToken = 'k6-fake-firebase-token';
const postJson = { headers: { 'Content-Type': 'application/json' } };

function expectBudgetCheck(expectedStatus) {
	return {
		'status matches rate-limit budget': (r) => {
			if (expectedStatus === 'underLimit') {
				return r.status !== 429;
			}
			return r.status === 429 && r.headers['Retry-After'] !== undefined;
		},
	};
}

// Iteration i must be allowed for i < budget and rejected with 429 afterwards.
export function loginBoundary() {
	const withinBudget = __ITER < 10;
	const res = http.post(`${cfg.baseUrl}${loginPath}`, JSON.stringify({ firebaseIdToken: fakeFirebaseToken }), postJson);
	check(res, expectBudgetCheck(withinBudget ? 'underLimit' : 'limited'));
}

export function refreshBoundary() {
	const withinBudget = __ITER < 30;
	const res = http.post(`${cfg.baseUrl}${refreshPath}`, JSON.stringify({ refreshToken: 'k6-fake-refresh-token' }), postJson);
	check(res, expectBudgetCheck(withinBudget ? 'underLimit' : 'limited'));
}

export function attendanceBoundary() {
	const withinBudget = __ITER < 20;
	const token = mintAccessToken(cfg.jwtSecret, cfg.employeeIds[0]);
	const res = http.post(
		`${cfg.baseUrl}${attendanceListPath(cfg.outletId)}`,
		JSON.stringify({ type: 'CLOCK_IN', latitude: 12.9715987, longitude: 77.5945627 }),
		{ headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` } },
	);
	check(res, expectBudgetCheck(withinBudget ? 'underLimit' : 'limited'));
}

// Two VUs with different X-Forwarded-For IPs must each get their own 10/min
// budget. If IP keys were shared, the second VU would start seeing 429s early.
export function loginIpIsolation() {
	const withinBudget = __ITER < 10;
	const ip = `198.51.100.${10 + __VU}`;
	const res = http.post(`${cfg.baseUrl}${loginPath}`, JSON.stringify({ firebaseIdToken: fakeFirebaseToken }), {
		headers: {
			'Content-Type': 'application/json',
			'X-Forwarded-For': ip,
		},
	});
	check(res, expectBudgetCheck(withinBudget ? 'underLimit' : 'limited'));
}
