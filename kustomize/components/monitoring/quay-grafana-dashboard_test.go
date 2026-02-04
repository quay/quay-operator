package monitoring_test

import (
	"encoding/json"
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

type ConfigMap struct {
	Data map[string]string `json:"data"`
}

type VariableValue struct {
	Selected bool   `json:"selected"`
	Text     string `json:"text"`
	Value    string `json:"value"`
}

type GrafanaDashboard struct {
	Templating struct {
		List []struct {
			Name    string         `json:"name"`
			Type    string         `json:"type"`
			Query   string         `json:"query"`
			Hide    int            `json:"hide"`
			Current *VariableValue `json:"current"`
		} `json:"list"`
	} `json:"templating"`
}

func TestGrafanaDashboardVariables(t *testing.T) {
	// Read the ConfigMap YAML
	content, err := os.ReadFile("quay-grafana-dashboard.configmap.yaml")
	if err != nil {
		t.Fatalf("Failed to read dashboard configmap: %v", err)
	}

	var cm ConfigMap
	if err := yaml.Unmarshal(content, &cm); err != nil {
		t.Fatalf("Failed to unmarshal configmap YAML: %v", err)
	}

	dashboardJSON, ok := cm.Data["quay.json"]
	if !ok {
		t.Fatal("Dashboard JSON not found in configmap data")
	}

	var dashboard GrafanaDashboard
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		t.Fatalf("Failed to unmarshal dashboard JSON: %v", err)
	}

	// Validate variables
	expectedVars := map[string]struct {
		varType string
		hide    int
	}{
		"datasource": {varType: "datasource", hide: 0},
		"namespace":  {varType: "custom", hide: 1},
		"service":    {varType: "custom", hide: 1},
	}

	for _, v := range dashboard.Templating.List {
		expected, ok := expectedVars[v.Name]
		if !ok {
			t.Logf("Unknown variable %q found", v.Name)
			continue
		}

		if v.Type != expected.varType {
			t.Errorf("Variable %q has type %q, expected %q", v.Name, v.Type, expected.varType)
		}

		if v.Hide != expected.hide {
			t.Errorf("Variable %q has hide=%d, expected %d", v.Name, v.Hide, expected.hide)
		}

		// For custom type variables, validate configuration
		if v.Type == "custom" {
			if v.Query == "" {
				t.Errorf("Variable %q has empty query", v.Name)
			}
			// Check for trailing comma (the original bug)
			if len(v.Query) > 0 && v.Query[len(v.Query)-1] == ',' {
				t.Errorf("Variable %q query %q has trailing comma", v.Name, v.Query)
			}
			// Check that current field is set (required for Grafana to apply the variable)
			if v.Current == nil {
				t.Errorf("Variable %q is missing 'current' field - Grafana won't apply the variable value", v.Name)
			} else if v.Current.Value == "" {
				t.Errorf("Variable %q has empty current value", v.Name)
			}
		}
	}

	// Ensure all expected variables exist
	foundVars := make(map[string]bool)
	for _, v := range dashboard.Templating.List {
		foundVars[v.Name] = true
	}

	for name := range expectedVars {
		if !foundVars[name] {
			t.Errorf("Expected variable %q not found in dashboard", name)
		}
	}
}
