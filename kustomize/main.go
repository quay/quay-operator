package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
)

func isConfigField(field string) bool {
	return !strings.Contains(field, ".")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

// generate uses Kustomize as a library to build the output files.
func generate(appDir string) {
	fSys := filesys.MakeFsOnDisk()
	opts := &krusty.Options{}
	k := krusty.MakeKustomizer(fSys, opts)
	m, err := k.Run(appDir)
	check(err)

	err = emitResources(fSys, m)
	check(err)
}

// Takes each `QUAY_CONFIG_FIELD` from the generated `Secret` and shoves it under a single `config.yaml` key
// (because that's what Quay wants).
//
// Usage: rm -rf ./output/* && go run main.go
func main() {
	appDir := os.Getenv("APP_DIR")
	if appDir == "" {
		appDir = "app"
	}
	generate(appDir)

	quayConfigSecretFile := ""

	files, err := ioutil.ReadDir("output")
	check(err)
	for _, f := range files {
		if strings.Contains(f.Name(), "quay-config-secret") {
			quayConfigSecretFile = f.Name()
			break
		}
	}

	yamlFile, err := ioutil.ReadFile(path.Join("output", quayConfigSecretFile))
	check(err)

	// TODO(alecmerdler): Use actual k8s `Secret` struct here...
	var quayConfigSecret map[string]interface{}
	err = yaml.Unmarshal(yamlFile, &quayConfigSecret)
	check(err)

	configYAML := quayConfigSecret["data"].(map[interface{}]interface{})["config.yaml"]
	decodedConfigYAML, err := base64.StdEncoding.DecodeString(configYAML.(string))
	check(err)

	var config map[string]interface{}
	err = yaml.Unmarshal(decodedConfigYAML, &config)
	check(err)

	for key, val := range quayConfigSecret["data"].(map[interface{}]interface{}) {
		if isConfigField(key.(string)) {
			decoded, err := base64.StdEncoding.DecodeString(val.(string))
			if err != nil {
				panic(err)
			}

			var decodedVal interface{}
			err = yaml.Unmarshal(decoded, &decodedVal)
			check(err)

			config[key.(string)] = decodedVal
			delete(quayConfigSecret["data"].(map[interface{}]interface{}), key)
		}
	}

	modifiedConfigYAML, err := yaml.Marshal(config)
	check(err)

	fmt.Println(string(modifiedConfigYAML))

	encodedConfigYAML := base64.StdEncoding.EncodeToString(modifiedConfigYAML)
	quayConfigSecret["data"].(map[interface{}]interface{})["config.yaml"] = encodedConfigYAML
	modifiedYAMLFile, err := yaml.Marshal(quayConfigSecret)
	check(err)

	err = ioutil.WriteFile(path.Join("output", quayConfigSecretFile), modifiedYAMLFile, 0644)
	check(err)

	fmt.Println("Successfully updated config secret.")
}

// NOTE: Functions below adapted from Kustomize (https://sourcegraph.com/github.com/kubernetes-sigs/kustomize/-/blob/kustomize/internal/commands/build/build.go)

func emitResources(fSys filesys.FileSystem, m resmap.ResMap) error {
	return writeIndividualFiles(fSys, "./output", m)
}

func writeIndividualFiles(fSys filesys.FileSystem, folderPath string, m resmap.ResMap) error {
	byNamespace := m.GroupedByCurrentNamespace()
	for namespace, resList := range byNamespace {
		for _, res := range resList {
			fName := fileName(res)
			if len(byNamespace) > 1 {
				fName = strings.ToLower(namespace) + "_" + fName
			}
			err := writeFile(fSys, folderPath, fName, res)
			if err != nil {
				return err
			}
		}
	}
	for _, res := range m.NonNamespaceable() {
		err := writeFile(fSys, folderPath, fileName(res), res)
		if err != nil {
			return err
		}
	}
	return nil
}

func fileName(res *resource.Resource) string {
	return strings.ToLower(res.GetGvk().String()) +
		"_" + strings.ToLower(res.GetName()) + ".yaml"
}

func writeFile(fSys filesys.FileSystem, path, fName string, res *resource.Resource) error {
	out, err := yaml.Marshal(res.Map())
	if err != nil {
		return err
	}
	return fSys.WriteFile(filepath.Join(path, fName), out)
}
