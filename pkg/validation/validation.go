package validation

import (
	"github.com/quay/config-tool/pkg/lib/fieldgroups/database"
	"github.com/quay/config-tool/pkg/lib/shared"
)

// ErrorsFor returns any validation errors from comparing the given config with the given component.
// If valid, returns empty list.
func ErrorsFor(component string, config map[string]interface{}) []shared.ValidationError {
	var fieldGroup shared.FieldGroup

	switch component {
	case "postgres":
		fieldGroup, _ = database.NewDatabaseFieldGroup(config)
	// TODO(alecmerdler): All component kinds...
	default:
		return []shared.ValidationError{}
	}

	return fieldGroup.Validate()
}
