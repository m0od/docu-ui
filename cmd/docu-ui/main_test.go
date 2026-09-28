package main

import (
	"reflect"
	"testing"
)

func environment(variables map[string]string) func(string) string {
	return func(key string) string { return variables[key] }
}

// The Secure flag keeps the session cookie off plain HTTP, where anyone on the network could copy
// it. Only the exact documented value turns it off: a "1", "yes" or "false" must never do it by accident.
func TestOnlyExactTrueDropsTheSecureCookie(tester *testing.T) {
	for _, value := range []string{"", "1", "TRUE", "True", "yes", "false", " true"} {
		if loadConfig(environment(map[string]string{"DOCU_INSECURE_COOKIE": value})).insecureCookie {
			tester.Errorf("%q dropped the Secure flag", value)
		}
	}
	if !loadConfig(environment(map[string]string{"DOCU_INSECURE_COOKIE": "true"})).insecureCookie {
		tester.Error(`"true" kept the Secure flag`)
	}
}

// README defaults: the image listens on 8080 and keeps its database in the /data volume.
func TestDefaultsAndOverrides(tester *testing.T) {
	defaults := loadConfig(environment(nil))
	if defaults.listenAddress != ":8080" || defaults.dataDirectory != "/data" || defaults.basePath != "" {
		tester.Fatalf("defaults: %+v", defaults)
	}
	overridden := loadConfig(environment(map[string]string{
		"DOCU_ADDR": ":9090", "DOCU_DATA_DIR": "/srv/docu", "DOCU_BASE_PATH": "/docu-ui",
		"DOCU_TLS_CERT": "/certs/cert.pem", "DOCU_TLS_KEY": "/certs/key.pem", "DOCU_ALLOWED_HOSTS": "docu.lan,docu.home",
	}))
	expected := startupConfig{
		listenAddress: ":9090", dataDirectory: "/srv/docu", basePath: "/docu-ui",
		tlsCertFile: "/certs/cert.pem", tlsKeyFile: "/certs/key.pem", allowedHosts: []string{"docu.lan", "docu.home"},
	}
	if !reflect.DeepEqual(overridden, expected) {
		tester.Fatalf("got %+v, want %+v", overridden, expected)
	}
}

// Without a cert a gateway terminates TLS. Half a TLS setup must not quietly serve plain HTTP:
// it picks HTTPS, where the missing file stops the start with an error.
func TestServesTLSWithCertOrKey(tester *testing.T) {
	testCases := map[string]struct {
		variables map[string]string
		servesTLS bool
	}{
		"neither":   {nil, false},
		"both":      {map[string]string{"DOCU_TLS_CERT": "cert.pem", "DOCU_TLS_KEY": "key.pem"}, true},
		"cert only": {map[string]string{"DOCU_TLS_CERT": "cert.pem"}, true},
		"key only":  {map[string]string{"DOCU_TLS_KEY": "key.pem"}, true},
	}
	for name, testCase := range testCases {
		if got := loadConfig(environment(testCase.variables)).servesTLS(); got != testCase.servesTLS {
			tester.Errorf("%s: servesTLS %v", name, got)
		}
	}
}
