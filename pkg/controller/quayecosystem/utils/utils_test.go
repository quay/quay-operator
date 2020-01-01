package utils

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestMergeEnvVars(t *testing.T) {

	cases := []struct {
		name            string
		baseEnvVars     []corev1.EnvVar
		overrideEnvVars []corev1.EnvVar
		expected        []corev1.EnvVar
	}{{
		name: "basic-valid-test",
		baseEnvVars: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "foo",
				Value: "bar",
			},
		},
		overrideEnvVars: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "foo2",
				Value: "bar2",
			},
		},
		expected: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "foo",
				Value: "bar",
			},
			corev1.EnvVar{
				Name:  "foo2",
				Value: "bar2",
			},
		},
	},
		{
			name: "override-test",
			baseEnvVars: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "foo",
					Value: "bar",
				},
			},
			overrideEnvVars: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "foo",
					Value: "override bar",
				},
			},
			expected: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "foo",
					Value: "override bar",
				},
			},
		},
		{
			name:        "empty-base-test",
			baseEnvVars: []corev1.EnvVar{},
			overrideEnvVars: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "foo",
					Value: "bar",
				},
			},
			expected: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "foo",
					Value: "bar",
				},
			},
		},
	}

	for i, c := range cases {

		t.Run(c.name, func(t *testing.T) {

			result := MergeEnvVars(c.baseEnvVars, c.overrideEnvVars)

			if !reflect.DeepEqual(c.expected, result) {
				t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
			}
		})

	}

}

func TestHostFromostname(t *testing.T) {

	cases := []struct {
		name     string
		input    string
		expected string
	}{{
		name:     "hostname-plain",
		input:    "quay.example.com",
		expected: "quay.example.com",
	},
		{
			name:     "hostname-port",
			input:    "quay.example.com:5000",
			expected: "quay.example.com",
		},
		{
			name:     "hostname-ip",
			input:    "192.168.100.100",
			expected: "192.168.100.100",
		},
		{
			name:     "hostname-ip-port",
			input:    "192.168.100.100:5000",
			expected: "192.168.100.100",
		},
	}

	for i, c := range cases {

		t.Run(c.name, func(t *testing.T) {

			result := GetHostFromHostname(c.input)

			if !reflect.DeepEqual(c.expected, result) {
				t.Errorf("Test case %d did not match\nExpected: %#v\nActual: %#v", i, c.expected, result)
			}
		})

	}

}
