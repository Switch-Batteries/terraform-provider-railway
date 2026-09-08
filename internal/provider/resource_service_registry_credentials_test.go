package provider

import (
	"encoding/json"
	"testing"
)

// The mutation must carry nothing but the credential: every other field of
// ServiceInstanceUpdateInput that reaches Railway as null clears a setting the
// Railway IaC file owns (cronSchedule cleared both watchdogs' schedules once).
func TestRegistryCredentialsMutationSendsOnlyTheCredential(t *testing.T) {
	input := __updateServiceInstanceRegistryCredentialsInput{
		EnvironmentId: "env",
		ServiceId:     "svc",
		Username:      "user",
		Password:      "secret",
	}

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var variables map[string]any
	if err := json.Unmarshal(raw, &variables); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"environmentId", "serviceId", "username", "password"} {
		if _, ok := variables[key]; !ok {
			t.Errorf("variable %q missing", key)
		}
	}

	if len(variables) != 4 {
		t.Errorf("expected exactly four variables, got %v", variables)
	}
}
