import crypto from 'k6/crypto';
import encoding from 'k6/encoding';

const b64url = (value) => encoding.b64encode(value, 'rawurl');

/**
 * Mints a local HS256 access token identical to the ones the app issues via
 * JwtService. This lets load tests hit authenticated endpoints without going
 * through Firebase. The subject must be a seeded user id.
 *
 * The JWT_SECRET must match the running app's secret (compose default matches
 * docker-compose.yml).
 */
export function mintAccessToken(secret, userId) {
	const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
	const now = Math.floor(Date.now() / 1000);
	const payload = b64url(JSON.stringify({
		sub: userId,
		iat: now,
		exp: now + 15 * 60,
	}));
	const hmac = crypto.createHMAC('sha256', secret);
	hmac.update(`${header}.${payload}`);
	return `${header}.${payload}.${hmac.digest('base64rawurl')}`;
}

/**
 * Resolves the shared test configuration from k6 env vars, falling back to the
 * fixed ids seeded by seed.sql and the docker-compose default JWT secret.
 */
export function loadConfig() {
	return {
		baseUrl: __ENV.BASE_URL || 'http://localhost:8080',
		jwtSecret: __ENV.JWT_SECRET || 'replace-this-with-a-long-random-production-secret-at-least-32-bytes',
		outletId: __ENV.OUTLET_ID || 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
		ownerId: __ENV.OWNER_ID || '11111111-1111-1111-1111-111111111111',
		employeeIds: (__ENV.EMPLOYEE_IDS || '22222222-2222-2222-2222-222222222222,33333333-3333-3333-3333-333333333333,44444444-4444-4444-4444-444444444444,55555555-5555-5555-5555-555555555555')
			.split(',')
			.map((id) => id.trim())
			.filter(Boolean),
	};
}

export const attendanceListPath = (outletId) => `/api/v1/outlets/${outletId}/attendance`;
export const loginPath = '/api/v1/auth/login';
export const refreshPath = '/api/v1/auth/refresh';
