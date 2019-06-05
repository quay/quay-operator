package utils

import "reflect"

func IsZeroOfUnderlyingType(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func CheckValue(valueToCheck interface{}, defaultValue interface{}) interface{} {
	if IsZeroOfUnderlyingType(valueToCheck) {
		return defaultValue
	}

	return valueToCheck
}
