package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coderGtm/delta/attendance"
	"github.com/coderGtm/delta/audit"
	"github.com/coderGtm/delta/auth"
	"github.com/coderGtm/delta/config"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/httpapi"
	"github.com/coderGtm/delta/metrics"
	"github.com/coderGtm/delta/outlet"
	"github.com/coderGtm/delta/report"
	"github.com/coderGtm/delta/user"
	"github.com/google/go-cmp/cmp"
)

// BuildTestServer wires the full application router against a fresh
// testcontainers PostgreSQL store and returns the running test server plus the
// store. The server and the store are cleaned up when the test completes.
func BuildTestServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()
	server, _, store := buildTestServer(t)
	return server, store
}

// buildTestServer is BuildTestServer with the stub Firebase exposed so tests
// can assert on the accounts it was asked to delete.
func buildTestServer(t *testing.T) (*httptest.Server, *StubFirebase, *db.Store) {
	t.Helper()
	store := Setup(t)
	registry := metrics.NewRegistry()
	recorder := audit.NewRecorder(store)
	stubFB := newStubFirebase()
	jwtSvc := auth.NewJWTService("test-secret-for-contract-tests", 15*time.Minute)
	refreshSvc := auth.NewRefreshTokenService(store, 30*24*time.Hour, 7*24*time.Hour)
	authSvc := auth.NewService(store, stubFB, jwtSvc, refreshSvc, recorder, registry)
	authHandlers := &auth.Handlers{Svc: authSvc, TrustProxy: false}

	apiMux := http.NewServeMux()
	apiMux.Handle("POST /api/v1/auth/login", http.HandlerFunc(authHandlers.Login))
	apiMux.Handle("POST /api/v1/auth/refresh", http.HandlerFunc(authHandlers.Refresh))
	apiMux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(authHandlers.Logout))
	apiMux.Handle("POST /api/v1/auth/logout-all", auth.Require(http.HandlerFunc(authHandlers.LogoutAll)))

	userHandlers := user.NewHandler(authSvc, store, false)
	apiMux.Handle("DELETE /api/v1/users/me", auth.Require(http.HandlerFunc(userHandlers.DeleteMe)))

	outletSvc := outlet.NewService(store, recorder, registry)
	outletHandlers := &outlet.Handlers{Svc: outletSvc, TrustProxy: false}
	apiMux.Handle("POST /api/v1/outlets", auth.Require(http.HandlerFunc(outletHandlers.CreateOutlet)))
	apiMux.Handle("GET /api/v1/outlets/{outletId}", auth.Require(http.HandlerFunc(outletHandlers.GetOutlet)))
	apiMux.Handle("PUT /api/v1/outlets/{outletId}", auth.Require(http.HandlerFunc(outletHandlers.UpdateOutlet)))
	apiMux.Handle("PUT /api/v1/outlets/{outletId}/geofence", auth.Require(http.HandlerFunc(outletHandlers.UpdateGeofence)))
	apiMux.Handle("GET /api/v1/outlets/mine", auth.Require(http.HandlerFunc(outletHandlers.GetMyOutlets)))
	apiMux.Handle("GET /api/v1/outlets/invites", auth.Require(http.HandlerFunc(outletHandlers.GetMyInvites)))
	apiMux.Handle("GET /api/v1/outlets/{outletId}/memberships", auth.Require(http.HandlerFunc(outletHandlers.GetOutletMemberships)))
	apiMux.Handle("DELETE /api/v1/outlets/{outletId}", auth.Require(http.HandlerFunc(outletHandlers.DeleteOutlet)))
	apiMux.Handle("POST /api/v1/outlets/{outletId}/leave", auth.Require(http.HandlerFunc(outletHandlers.LeaveOutlet)))
	apiMux.Handle("POST /api/v1/outlets/{outletId}/memberships/invite", auth.Require(http.HandlerFunc(outletHandlers.InviteMember)))
	apiMux.Handle("DELETE /api/v1/outlets/{outletId}/memberships/{membershipId}", auth.Require(http.HandlerFunc(outletHandlers.RemoveMembership)))
	apiMux.Handle("PUT /api/v1/outlets/{outletId}/memberships/{membershipId}/display-name", auth.Require(http.HandlerFunc(outletHandlers.UpdateDisplayName)))
	apiMux.Handle("POST /api/v1/outlets/memberships/{membershipId}/accept", auth.Require(http.HandlerFunc(outletHandlers.AcceptInvite)))
	apiMux.Handle("POST /api/v1/outlets/memberships/{membershipId}/reject", auth.Require(http.HandlerFunc(outletHandlers.RejectInvite)))

	attSvc := attendance.NewService(store, nil, recorder, registry)
	attHandlers := &attendance.Handlers{Svc: attSvc, TrustProxy: false}
	apiMux.Handle("POST /api/v1/outlets/{outletId}/attendance", auth.Require(http.HandlerFunc(attHandlers.CreateOwn)))
	apiMux.Handle("POST /api/v1/outlets/{outletId}/attendance/manage", auth.Require(http.HandlerFunc(attHandlers.CreateManaged)))
	apiMux.Handle("GET /api/v1/outlets/{outletId}/attendance", auth.Require(http.HandlerFunc(attHandlers.List)))
	apiMux.Handle("GET /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(http.HandlerFunc(attHandlers.Get)))
	apiMux.Handle("PUT /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(http.HandlerFunc(attHandlers.Update)))
	apiMux.Handle("DELETE /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(http.HandlerFunc(attHandlers.Delete)))

	reportSvc := report.NewService(store, recorder, registry)
	reportHandlers := &report.Handlers{Svc: reportSvc, TrustProxy: false}
	apiMux.Handle("GET /api/v1/outlets/{outletId}/reports/salary", auth.Require(http.HandlerFunc(reportHandlers.Salary)))
	apiMux.Handle("GET /api/v1/outlets/{outletId}/reports/salary.xlsx", auth.Require(http.HandlerFunc(reportHandlers.SalaryXLSX)))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{TrustProxyHeaders: false, PrometheusBearerToken: ""}
	ready := func(ctx context.Context) error { return store.Pool().Ping(ctx) }
	rateLimiter := httpapi.NewRateLimiter(false)
	router := httpapi.NewRouter(logger, cfg, ready, registry.Handler(), auth.AttachUser(jwtSvc, store), rateLimiter.Middleware, apiMux)

	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, stubFB, store
}

// login performs a Firebase token login and returns the Authorization header
// value carrying the issued access token.
func login(t *testing.T, client *http.Client, serverURL, token string) string {
	t.Helper()
	status, _, body := doJSON(t, client, http.MethodPost, serverURL+"/api/v1/auth/login", "", map[string]any{"firebaseIdToken": token})
	if status != http.StatusOK {
		t.Fatalf("login with %q: status %d, body %s", token, status, body)
	}
	var resp struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatalf("login response missing accessToken")
	}
	return "Bearer " + resp.AccessToken
}

// doJSON sends a request with an optional JSON body and optional bearer token,
// returning the status code, response headers, and response body.
func doJSON(t *testing.T, client *http.Client, method, url, bearer string, body any) (int, http.Header, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header, data
}

// decodeMap decodes body as a JSON object.
func decodeMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode JSON: %v (body %s)", err, body)
	}
	return m
}

// assertStatus fails the test unless the status matches.
func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
}

// assertErrorCode fails the test unless body is an error envelope with the
// given code.
func assertErrorCode(t *testing.T, body []byte, wantCode string) {
	t.Helper()
	m := decodeMap(t, body)
	if got, _ := m["code"].(string); got != wantCode {
		t.Fatalf("error code = %q, want %q (body %s)", got, wantCode, body)
	}
}

// assertError fails the test unless body is an error envelope with the given
// code and exact message.
func assertError(t *testing.T, body []byte, wantCode, wantMsg string) {
	t.Helper()
	m := decodeMap(t, body)
	if got, _ := m["code"].(string); got != wantCode {
		t.Fatalf("error code = %q, want %q", got, wantCode)
	}
	if got, _ := m["message"].(string); got != wantMsg {
		t.Fatalf("error message = %q, want %q", got, wantMsg)
	}
}

// assertJSONKeys fails the test unless the top-level object keys of body equal
// want (ignoring order).
func assertJSONKeys(t *testing.T, body []byte, want ...string) {
	t.Helper()
	got := make([]string, 0, len(want))
	for k := range decodeMap(t, body) {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JSON keys mismatch (-want +got):\n%s", diff)
	}
}

// assertJSONKeyOrder fails the test unless the top-level object keys of body
// appear in exactly the given order.
func assertJSONKeyOrder(t *testing.T, body []byte, want ...string) {
	t.Helper()
	re := regexp.MustCompile(`"([A-Za-z0-9_]+)"\s*:`)
	var got []string
	for _, m := range re.FindAllStringSubmatch(string(body), -1) {
		got = append(got, m[1])
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JSON key order mismatch (-want +got):\n%s", diff)
	}
}

func TestContractEndpoints(t *testing.T) {
	t.Run("login", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "",
			map[string]any{"firebaseIdToken": "token-owner"})
		assertStatus(t, status, http.StatusOK)
		assertJSONKeys(t, body, "id", "name", "email", "phone", "accessToken", "refreshToken", "createdAt", "updatedAt")
		assertJSONKeyOrder(t, body, "id", "name", "email", "phone", "accessToken", "refreshToken", "createdAt", "updatedAt")
		m := decodeMap(t, body)
		if m["name"] != "Owner User" || m["email"] != "owner@example.com" || m["phone"] != "" {
			t.Errorf("login profile = %v, want name/email/phone from owner token", m)
		}
		if m["accessToken"] == "" || m["refreshToken"] == "" {
			t.Errorf("login response missing tokens: %v", m)
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "",
			map[string]any{"firebaseIdToken": "bogus-token"})
		assertStatus(t, status, http.StatusUnauthorized)
		assertError(t, body, "INVALID_TOKEN", "Invalid Firebase ID Token")
	})

	t.Run("refresh rotation logout", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()

		_, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "",
			map[string]any{"firebaseIdToken": "token-owner"})
		var first struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.Unmarshal(body, &first); err != nil {
			t.Fatal(err)
		}

		status, _, rotated := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "",
			map[string]any{"refreshToken": first.RefreshToken})
		assertStatus(t, status, http.StatusOK)
		assertJSONKeys(t, rotated, "accessToken", "refreshToken")
		var second struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.Unmarshal(rotated, &second); err != nil {
			t.Fatal(err)
		}
		if second.RefreshToken == "" || second.RefreshToken == first.RefreshToken {
			t.Errorf("rotated refresh token = %q, want a fresh value", second.RefreshToken)
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "",
			map[string]any{"refreshToken": first.RefreshToken})
		assertStatus(t, status, http.StatusUnauthorized)
		assertErrorCode(t, body, "INVALID_TOKEN")

		status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/logout", "",
			map[string]any{"refreshToken": second.RefreshToken})
		assertStatus(t, status, http.StatusNoContent)
		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "",
			map[string]any{"refreshToken": second.RefreshToken})
		assertStatus(t, status, http.StatusUnauthorized)
		assertErrorCode(t, body, "INVALID_TOKEN")

		var third, fourth struct {
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.Unmarshal(mustLogin(t, client, server, "token-owner"), &third); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(mustLogin(t, client, server, "token-owner"), &fourth); err != nil {
			t.Fatal(err)
		}
		ownerBearer := login(t, client, server.URL, "token-owner")

		status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/logout-all", ownerBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		for _, tok := range []string{third.RefreshToken, fourth.RefreshToken} {
			status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "",
				map[string]any{"refreshToken": tok})
			assertStatus(t, status, http.StatusUnauthorized)
			assertErrorCode(t, body, "INVALID_TOKEN")
		}
	})

	t.Run("outlet create and owner checks", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")
		empBearer := login(t, client, server.URL, "token-emp")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusCreated)
		assertJSONKeys(t, body, "id", "name", "latitude", "longitude", "radiusMeters", "geofenceEnabled", "createdAt", "updatedAt")
		outletMap := decodeMap(t, body)
		if outletMap["name"] != "HQ" || outletMap["geofenceEnabled"] != false {
			t.Errorf("created outlet = %v", outletMap)
		}
		outletID, _ := outletMap["id"].(string)

		status, _, mine := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/mine", ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		assertJSONKeys(t, mine, "content", "page", "size", "totalElements", "totalPages", "first", "last", "empty")
		content := decodeMap(t, mine)["content"].([]any)
		if len(content) != 1 {
			t.Fatalf("owner mine = %d outlets, want 1", len(content))
		}
		mem := content[0].(map[string]any)
		if mem["role"] != "OWNER" || mem["status"] != "ACCEPTED" || mem["displayName"] != "Owner User" {
			t.Errorf("owner membership = %v", mem)
		}
		if mem["outlet"].(map[string]any)["id"] != outletID {
			t.Errorf("owner membership outlet id = %v, want %s", mem["outlet"].(map[string]any)["id"], outletID)
		}

		inviteAndAccept(t, client, server, outletID, ownerBearer, empBearer, "employee@example.com")

		status, _, body = doJSON(t, client, http.MethodPut, server.URL+"/api/v1/outlets/"+outletID, empBearer,
			map[string]any{"name": "HQ2", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusForbidden)
		assertError(t, body, "FORBIDDEN", "Only outlet owners can perform this action")

		status, _, body = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/outlets/"+outletID, empBearer, nil)
		assertStatus(t, status, http.StatusForbidden)
		assertError(t, body, "FORBIDDEN", "Only outlet owners can perform this action")

		status, _, body = doJSON(t, client, http.MethodPut, server.URL+"/api/v1/outlets/"+outletID+"/geofence", empBearer,
			map[string]any{"geofenceEnabled": true})
		assertStatus(t, status, http.StatusForbidden)
		assertError(t, body, "FORBIDDEN", "Only outlet owners can perform this action")

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "", "latitude": 999, "longitude": -74.0060, "radiusMeters": 0})
		assertStatus(t, status, http.StatusBadRequest)
		assertError(t, body, "VALIDATION_ERROR",
			"Outlet name is required, Latitude must be less than or equal to 90, Radius in meters must be greater than zero")

		missing := "00000000-0000-0000-0000-000000000000"
		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+missing, ownerBearer, nil)
		assertStatus(t, status, http.StatusNotFound)
		assertError(t, body, "NOT_FOUND", "Outlet not found: "+missing)
	})

	t.Run("outlet create accepts lenient JSON coercion", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")

		// A mobile client built against the previous API's JSON decoder may
		// send quoted coordinates and a fractional radius; both must be accepted.
		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ2", "latitude": "40.7128", "longitude": "-74.006", "radiusMeters": 500.0})
		assertStatus(t, status, http.StatusCreated)
		outletMap := decodeMap(t, body)
		if outletMap["radiusMeters"] != float64(500) {
			t.Errorf("lenient radiusMeters = %v, want 500", outletMap["radiusMeters"])
		}
		if outletMap["name"] != "HQ2" {
			t.Errorf("lenient name = %v", outletMap["name"])
		}
	})

	t.Run("membership lifecycle", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")
		empBearer := login(t, client, server.URL, "token-emp")
		secondBearer := login(t, client, server.URL, "token-second")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusCreated)
		outletID := decodeMap(t, body)["id"].(string)

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/memberships/invite", ownerBearer,
			map[string]any{"email": "employee@example.com"})
		assertStatus(t, status, http.StatusCreated)
		assertJSONKeys(t, body, "membershipId", "outlet", "userId", "userName", "userEmail", "displayName", "role", "status", "invitedByUserId", "invitedByUserName", "createdAt", "updatedAt")
		empMembership := decodeMap(t, body)
		if empMembership["status"] != "INVITED" || empMembership["role"] != "EMPLOYEE" {
			t.Errorf("invite = %v", empMembership)
		}
		if empMembership["invitedByUserName"] != "Owner User" {
			t.Errorf("invite invitedByUserName = %v, want Owner User", empMembership["invitedByUserName"])
		}
		empMembershipID := empMembership["membershipId"].(string)

		status, _, invites := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/invites", empBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, invites)["content"].([]any)); got != 1 {
			t.Errorf("employee invites = %d, want 1", got)
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/memberships/"+empMembershipID+"/accept", empBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if decodeMap(t, body)["status"] != "ACCEPTED" {
			t.Errorf("accept = %v", decodeMap(t, body))
		}

		status, _, mine := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/mine", empBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, mine)["content"].([]any)); got != 1 {
			t.Errorf("employee mine = %d, want 1", got)
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/memberships/invite", ownerBearer,
			map[string]any{"email": "second@example.com"})
		assertStatus(t, status, http.StatusCreated)
		secondMembershipID := decodeMap(t, body)["membershipId"].(string)

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/memberships/"+secondMembershipID+"/reject", secondBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if decodeMap(t, body)["status"] != "REJECTED" {
			t.Errorf("reject = %v", decodeMap(t, body))
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/memberships/invite", ownerBearer,
			map[string]any{"email": "second@example.com"})
		assertStatus(t, status, http.StatusCreated)
		if got := decodeMap(t, body)["membershipId"].(string); got != secondMembershipID {
			t.Errorf("re-invite membership id = %s, want %s (reopened)", got, secondMembershipID)
		}
		if decodeMap(t, body)["status"] != "INVITED" {
			t.Errorf("re-invite = %v, want INVITED", decodeMap(t, body))
		}
		status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/memberships/"+secondMembershipID+"/accept", secondBearer, nil)
		assertStatus(t, status, http.StatusOK)

		status, _, body = doJSON(t, client, http.MethodPut, server.URL+"/api/v1/outlets/"+outletID+"/memberships/"+empMembershipID+"/display-name", ownerBearer,
			map[string]any{"displayName": "Nick"})
		assertStatus(t, status, http.StatusOK)
		if decodeMap(t, body)["displayName"] != "Nick" {
			t.Errorf("display-name = %v", decodeMap(t, body))
		}

		status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/leave", empBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		status, _, mine = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/mine", empBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, mine)["content"].([]any)); got != 0 {
			t.Errorf("employee mine after leave = %d, want 0", got)
		}

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/memberships/invite", ownerBearer,
			map[string]any{"email": "employee@example.com"})
		assertStatus(t, status, http.StatusCreated)
		if got := decodeMap(t, body)["membershipId"].(string); got != empMembershipID {
			t.Errorf("re-invite after leave id = %s, want %s", got, empMembershipID)
		}
		status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/memberships/"+empMembershipID+"/accept", empBearer, nil)
		assertStatus(t, status, http.StatusOK)

		status, _, _ = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/outlets/"+outletID+"/memberships/"+empMembershipID, ownerBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID, empBearer, nil)
		assertStatus(t, status, http.StatusNotFound)

		status, _, mine = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/mine", ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		ownerMembershipID := decodeMap(t, mine)["content"].([]any)[0].(map[string]any)["membershipId"].(string)
		status, _, body = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/outlets/"+outletID+"/memberships/"+ownerMembershipID, ownerBearer, nil)
		assertStatus(t, status, http.StatusBadRequest)
		assertError(t, body, "BAD_REQUEST", "Owner memberships cannot be removed through this endpoint")
	})

	t.Run("attendance", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")
		empBearer := login(t, client, server.URL, "token-emp")
		secondBearer := login(t, client, server.URL, "token-second")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusCreated)
		outletID := decodeMap(t, body)["id"].(string)

		inviteAndAccept(t, client, server, outletID, ownerBearer, empBearer, "employee@example.com")
		inviteAndAccept(t, client, server, outletID, ownerBearer, secondBearer, "second@example.com")
		secondEmpID := decodeMap(t, mustLogin(t, client, server, "token-second"))["id"].(string)

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance", empBearer,
			map[string]any{"type": "CLOCK_IN", "latitude": 40.7128, "longitude": -74.0060})
		assertStatus(t, status, http.StatusCreated)
		assertJSONKeys(t, body, "id", "outletId", "userId", "userName", "userEmail", "displayName", "type", "entryTime", "latitude", "longitude", "createdByUserId", "updatedByUserId", "createdAt", "updatedAt")
		ownEntry := decodeMap(t, body)
		if ownEntry["type"] != "CLOCK_IN" || ownEntry["userName"] != "Employee User" || ownEntry["displayName"] != "Employee User" {
			t.Errorf("own entry = %v", ownEntry)
		}
		if ownEntry["createdByUserId"] != ownEntry["userId"] {
			t.Errorf("own entry createdByUserId = %v, want own userId", ownEntry["createdByUserId"])
		}
		entryTime, err := time.Parse(time.RFC3339Nano, ownEntry["entryTime"].(string))
		if err != nil {
			t.Fatalf("entryTime parse: %v", err)
		}
		if d := time.Since(entryTime); d < -time.Minute || d > 5*time.Minute {
			t.Errorf("own entry entryTime = %s, not near server now", ownEntry["entryTime"])
		}
		ownEntryID := ownEntry["id"].(string)
		empID := ownEntry["userId"].(string)

		managedTime := time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC)
		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance/manage", ownerBearer,
			map[string]any{"userId": empID, "type": "CLOCK_OUT", "entryTime": managedTime.Format(time.RFC3339Nano), "latitude": 40.7128, "longitude": -74.0060})
		assertStatus(t, status, http.StatusCreated)
		managedEntry := decodeMap(t, body)
		if managedEntry["type"] != "CLOCK_OUT" || managedEntry["createdByUserId"] != decodeMap(t, mustLogin(t, client, server, "token-owner"))["id"] {
			t.Errorf("managed entry = %v", managedEntry)
		}

		status, _, page := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance", empBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, page)["content"].([]any)); got != 2 {
			t.Errorf("employee list = %d, want 2", got)
		}

		status, _, page = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance", secondBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, page)["content"].([]any)); got != 0 {
			t.Errorf("second employee list = %d, want 0", got)
		}

		status, _, page = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance", ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if got := len(decodeMap(t, page)["content"].([]any)); got != 2 {
			t.Errorf("owner list = %d, want 2", got)
		}

		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance?userId="+secondEmpID, empBearer, nil)
		assertStatus(t, status, http.StatusForbidden)
		assertError(t, body, "FORBIDDEN", "Employees can only view their own attendance entries")

		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance/"+ownEntryID, ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if decodeMap(t, body)["id"] != ownEntryID {
			t.Errorf("get = %v, want id %s", decodeMap(t, body), ownEntryID)
		}

		updateTime := time.Date(2026, 8, 10, 18, 0, 0, 0, time.UTC)
		status, _, body = doJSON(t, client, http.MethodPut, server.URL+"/api/v1/outlets/"+outletID+"/attendance/"+ownEntryID, ownerBearer,
			map[string]any{"type": "CLOCK_OUT", "entryTime": updateTime.Format(time.RFC3339Nano), "latitude": 40.7128, "longitude": -74.0060})
		assertStatus(t, status, http.StatusOK)
		updated := decodeMap(t, body)
		if updated["type"] != "CLOCK_OUT" || updated["updatedByUserId"] != decodeMap(t, mustLogin(t, client, server, "token-owner"))["id"] {
			t.Errorf("updated entry = %v", updated)
		}

		status, _, _ = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/outlets/"+outletID+"/attendance/"+ownEntryID, ownerBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/attendance/"+ownEntryID, ownerBearer, nil)
		assertStatus(t, status, http.StatusNotFound)
	})

	t.Run("attendance geofence", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")
		empBearer := login(t, client, server.URL, "token-emp")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "Fenced", "latitude": 0, "longitude": 0, "radiusMeters": 1000})
		assertStatus(t, status, http.StatusCreated)
		outletID := decodeMap(t, body)["id"].(string)

		status, _, body = doJSON(t, client, http.MethodPut, server.URL+"/api/v1/outlets/"+outletID+"/geofence", ownerBearer,
			map[string]any{"geofenceEnabled": true})
		assertStatus(t, status, http.StatusOK)
		if decodeMap(t, body)["geofenceEnabled"] != true {
			t.Errorf("geofence = %v", decodeMap(t, body))
		}

		inviteAndAccept(t, client, server, outletID, ownerBearer, empBearer, "employee@example.com")

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance", empBearer,
			map[string]any{"type": "CLOCK_IN", "latitude": 10, "longitude": 10})
		assertStatus(t, status, http.StatusForbidden)
		assertError(t, body, "FORBIDDEN", "Attendance location is outside the outlet geofence")

		status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance", empBearer,
			map[string]any{"type": "CLOCK_IN", "latitude": 0, "longitude": 0})
		assertStatus(t, status, http.StatusCreated)
	})

	t.Run("salary report", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")
		empBearer := login(t, client, server.URL, "token-emp")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusCreated)
		outletID := decodeMap(t, body)["id"].(string)

		inviteAndAccept(t, client, server, outletID, ownerBearer, empBearer, "employee@example.com")

		empID := decodeMap(t, mustLogin(t, client, server, "token-emp"))["id"].(string)
		seedPair := func(t1, t2 time.Time) {
			status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance/manage", ownerBearer,
				map[string]any{"userId": empID, "type": "CLOCK_IN", "entryTime": t1.Format(time.RFC3339Nano), "latitude": 40.7128, "longitude": -74.0060})
			assertStatus(t, status, http.StatusCreated)
			status, _, body = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/attendance/manage", ownerBearer,
				map[string]any{"userId": empID, "type": "CLOCK_OUT", "entryTime": t2.Format(time.RFC3339Nano), "latitude": 40.7128, "longitude": -74.0060})
			assertStatus(t, status, http.StatusCreated)
			_ = body
		}
		seedPair(time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC))
		seedPair(time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 11, 13, 0, 0, 0, time.UTC))

		start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2026, 8, 11, 23, 59, 59, 999999999, time.UTC)
		query := "?userId=" + empID + "&startTime=" + urlQueryTime(start) + "&endTime=" + urlQueryTime(end) + "&timezone=UTC&hourlyRate=20"
		status, _, body = doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/reports/salary"+query, ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		assertJSONKeys(t, body, "outletId", "outletName", "userId", "userName", "userEmail", "displayName", "startTime", "endTime", "timezone", "hourlyRate", "totalHours", "totalSalary", "days")

		var rep report.SalaryReport
		if err := json.Unmarshal(body, &rep); err != nil {
			t.Fatalf("decode report: %v", err)
		}
		if rep.OutletName != "HQ" || rep.Timezone != "UTC" || rep.DisplayName != "Employee User" || rep.UserName != "Employee User" {
			t.Errorf("report header = %+v", rep)
		}
		if rep.TotalHours.Format(2) != "12.00" || rep.TotalSalary.Format(2) != "240.00" {
			t.Errorf("report totals = %s / %s, want 12.00 / 240.00", rep.TotalHours.Format(2), rep.TotalSalary.Format(2))
		}
		if len(rep.Days) != 2 {
			t.Fatalf("report days = %d, want 2", len(rep.Days))
		}
		if rep.Days[0].Date != "2026-08-10" || len(rep.Days[0].Pairs) != 1 || rep.Days[0].TotalHours.Format(2) != "8.00" || rep.Days[0].Salary.Format(2) != "160.00" {
			t.Errorf("day 1 = %+v", rep.Days[0])
		}
		if rep.Days[1].Date != "2026-08-11" || len(rep.Days[1].Pairs) != 1 || rep.Days[1].TotalHours.Format(2) != "4.00" || rep.Days[1].Salary.Format(2) != "80.00" {
			t.Errorf("day 2 = %+v", rep.Days[1])
		}

		status, hdr, xlsx := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/"+outletID+"/reports/salary.xlsx"+query, ownerBearer, nil)
		assertStatus(t, status, http.StatusOK)
		if ct := hdr.Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			t.Errorf("xlsx Content-Type = %q", ct)
		}
		if cd := hdr.Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="salary-report-`) || !strings.HasSuffix(cd, `.xlsx"`) {
			t.Errorf("xlsx Content-Disposition = %q", cd)
		}
		if len(xlsx) == 0 {
			t.Errorf("xlsx body empty")
		}
	})

	t.Run("delete account", func(t *testing.T) {
		server, stubFB, _ := buildTestServer(t)
		client := server.Client()
		secondBearer := login(t, client, server.URL, "token-second")

		status, _, _ := doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/users/me", secondBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		if len(stubFB.Deleted) != 1 || stubFB.Deleted[0] != "second-uid" {
			t.Errorf("stubFB.Deleted = %v, want [second-uid]", stubFB.Deleted)
		}
	})

	t.Run("delete account blocked while owning active outlets", func(t *testing.T) {
		server, stubFB, _ := buildTestServer(t)
		client := server.Client()
		ownerBearer := login(t, client, server.URL, "token-owner")

		status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets", ownerBearer,
			map[string]any{"name": "HQ", "latitude": 40.7128, "longitude": -74.0060, "radiusMeters": 100})
		assertStatus(t, status, http.StatusCreated)
		outletID := decodeMap(t, body)["id"].(string)

		status, _, body = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/users/me", ownerBearer, nil)
		assertStatus(t, status, http.StatusConflict)
		assertError(t, body, "CONFLICT", "Delete your active outlets before deleting your account: HQ")
		if len(stubFB.Deleted) != 0 {
			t.Errorf("stubFB.Deleted = %v, want no deletions while outlets exist", stubFB.Deleted)
		}

		status, _, _ = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/outlets/"+outletID, ownerBearer, nil)
		assertStatus(t, status, http.StatusNoContent)

		status, _, _ = doJSON(t, client, http.MethodDelete, server.URL+"/api/v1/users/me", ownerBearer, nil)
		assertStatus(t, status, http.StatusNoContent)
		if len(stubFB.Deleted) != 1 || stubFB.Deleted[0] != "owner-uid" {
			t.Errorf("stubFB.Deleted = %v, want [owner-uid]", stubFB.Deleted)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()

		status, hdr, body := doJSON(t, client, http.MethodGet, server.URL+"/api/v1/outlets/mine", "", nil)
		assertStatus(t, status, http.StatusUnauthorized)
		if len(body) != 0 {
			t.Errorf("unauthorized body = %q, want empty", body)
		}
		if got := hdr.Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("WWW-Authenticate = %q, want Bearer", got)
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		server, _ := BuildTestServer(t)
		client := server.Client()

		var saw429 int
		for i := 0; i < 12; i++ {
			req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/login",
				strings.NewReader(`{"firebaseIdToken":"token-owner"}`))
			if err != nil {
				t.Fatal(err)
			}
			// The client-side RemoteAddr is never transmitted to the server, so
			// all requests here share the server's TCP peer IP, which is what
			// the login rate limit is keyed on.
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("login request %d: %v", i, err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusTooManyRequests {
				saw429++
				assertErrorCode(t, body, "RATE_LIMIT_EXCEEDED")
				if resp.Header.Get("Retry-After") == "" {
					t.Errorf("429 response missing Retry-After header")
				}
			}
		}
		if saw429 == 0 {
			t.Errorf("no request was rate limited after 12 logins")
		}
	})
}

// inviteAndAccept invites the user behind employeeBearer to outletID and
// accepts the invitation, returning the membership id.
func inviteAndAccept(t *testing.T, client *http.Client, server *httptest.Server, outletID, ownerBearer, employeeBearer, email string) string {
	t.Helper()
	status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/"+outletID+"/memberships/invite", ownerBearer,
		map[string]any{"email": email})
	assertStatus(t, status, http.StatusCreated)
	membershipID := decodeMap(t, body)["membershipId"].(string)
	status, _, _ = doJSON(t, client, http.MethodPost, server.URL+"/api/v1/outlets/memberships/"+membershipID+"/accept", employeeBearer, nil)
	assertStatus(t, status, http.StatusOK)
	return membershipID
}

// mustLogin returns the raw login response for the given token.
func mustLogin(t *testing.T, client *http.Client, server *httptest.Server, token string) []byte {
	t.Helper()
	status, _, body := doJSON(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", "",
		map[string]any{"firebaseIdToken": token})
	assertStatus(t, status, http.StatusOK)
	return body
}

// urlQueryTime renders t for a URL query parameter.
func urlQueryTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
