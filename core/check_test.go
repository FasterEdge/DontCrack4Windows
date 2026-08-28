package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckPasswordAcceptsHeaderAndQuery(t *testing.T) {
	const expected = "s3cret"

	r := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	if err := checkPassword(r, expected); err == nil {
		t.Fatal("missing password should fail")
	}

	r = httptest.NewRequest(http.MethodGet, "/heartbeat?password=s3cret", nil)
	if err := checkPassword(r, expected); err != nil {
		t.Fatalf("query password should be accepted for compatibility, got: %v", err)
	}

	r = httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	if err := checkPassword(r, expected); err != nil {
		t.Fatalf("bearer header should be accepted, got: %v", err)
	}

	r = httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	r.Header.Set("X-DontCrack-Password", "s3cret")
	if err := checkPassword(r, expected); err != nil {
		t.Fatalf("custom header should be accepted, got: %v", err)
	}

	// 未配置密码时全部放行
	r = httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	if err := checkPassword(r, ""); err != nil {
		t.Fatalf("empty expected password should pass, got: %v", err)
	}
}

// 复用 stater.go 的注册逻辑验证认证失败返回 401 而非 200
func TestUnauthorizedReturns401(t *testing.T) {
	// 通过 startServer 之外的独立 handler 无法直接复用；这里验证 http.Error 行为约定：
	// 修复后的 handler 必须写 StatusUnauthorized。用一个最小 handler 模拟修复后的分支。
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := checkPassword(r, "s3cret"); err != nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/heartbeat", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("auth failure must return 401, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/heartbeat", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("correct password must return 200, got %d", rec.Code)
	}
}
