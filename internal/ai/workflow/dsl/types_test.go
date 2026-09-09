package dsl

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestModelPolicyDecodesLegacyProductScopeAsFallbackChain(t *testing.T) {
	var definition Definition
	if err := json.Unmarshal([]byte(`{"modelPolicy":{"credentialScope":"product"}}`), &definition); err != nil {
		t.Fatal(err)
	}
	want := []string{ModelCredentialScopeProduct, ModelCredentialScopeTenantDefault}
	if definition.ModelPolicy == nil || !reflect.DeepEqual(definition.ModelPolicy.CredentialChain, want) {
		t.Fatalf("credential chain = %#v, want %#v", definition.ModelPolicy, want)
	}

	raw, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "credentialScope") || !strings.Contains(string(raw), "credentialChain") {
		t.Fatalf("legacy policy was not normalized: %s", raw)
	}
}

func TestDefinitionUsesDeclaredCredentialOrder(t *testing.T) {
	definition := Definition{ModelPolicy: &ModelPolicy{CredentialChain: []string{
		ModelCredentialScopeTenantDefault,
		ModelCredentialScopeProduct,
	}}}
	want := []string{ModelCredentialScopeTenantDefault, ModelCredentialScopeProduct}
	if got := definition.EffectiveModelCredentialChain(42); !reflect.DeepEqual(got, want) {
		t.Fatalf("credential chain = %#v, want %#v", got, want)
	}
}

func TestDefinitionDefaultsProductContextToProductThenTenant(t *testing.T) {
	want := []string{ModelCredentialScopeProduct, ModelCredentialScopeTenantDefault}
	if got := (Definition{}).EffectiveModelCredentialChain(42); !reflect.DeepEqual(got, want) {
		t.Fatalf("credential chain = %#v, want %#v", got, want)
	}
}
