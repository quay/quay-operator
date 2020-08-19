package validation

import (
	"testing"

	"github.com/quay/config-tool/pkg/lib/shared"
	"github.com/stretchr/testify/assert"
)

var errorsForTests = []struct {
	name      string
	component string
	config    map[string]interface{}
	expected  []shared.ValidationError
}{
	{
		"NilComponent",
		"",
		map[string]interface{}{},
		[]shared.ValidationError{},
	},
	{
		"PostgresValid",
		"postgres",
		map[string]interface{}{},
		[]shared.ValidationError{},
	},
	{
		"PostgresInvalid",
		"postgres",
		map[string]interface{}{},
		[]shared.ValidationError{},
	},
}

func TestErrorsFor(t *testing.T) {
	t.Skip()
	assert := assert.New(t)

	for _, test := range errorsForTests {
		errors := ErrorsFor(test.component, test.config)

		assert.Equal(len(test.expected), len(errors))
	}
}
