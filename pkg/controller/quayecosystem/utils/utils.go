package utils

import (
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func IsZeroOfUnderlyingType(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func CheckValue(valueToCheck interface{}, defaultValue interface{}) interface{} {
	if IsZeroOfUnderlyingType(valueToCheck) {
		return defaultValue
	}

	return valueToCheck
}

func MergeEnvVars(baseEnvVars []corev1.EnvVar, overrideEnvVars []corev1.EnvVar) []corev1.EnvVar {

	for _, o := range overrideEnvVars {

		checkExistingKey, checkExistingKeyIdx := checkExistingKey(o, baseEnvVars)

		if checkExistingKey {
			baseEnvVars[checkExistingKeyIdx] = o
		} else {
			baseEnvVars = append(baseEnvVars, o)
		}

	}

	return baseEnvVars
}

func Retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func checkExistingKey(envVar corev1.EnvVar, envVars []corev1.EnvVar) (bool, int) {

	for bIdx, b := range envVars {

		if b.Name == envVar.Name {
			return true, bIdx
		}
	}

	return false, 0

}
