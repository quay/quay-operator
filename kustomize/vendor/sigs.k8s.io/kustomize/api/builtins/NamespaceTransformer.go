// Code generated by pluginator on NamespaceTransformer; DO NOT EDIT.
// pluginator {unknown  1970-01-01T00:00:00Z  }

package builtins

import (
	"fmt"

	"sigs.k8s.io/kustomize/api/filters/namespace"
	"sigs.k8s.io/kustomize/api/resid"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/transform"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filtersutil"
	"sigs.k8s.io/yaml"
)

// Change or set the namespace of non-cluster level resources.
type NamespaceTransformerPlugin struct {
	types.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	FieldSpecs       []types.FieldSpec `json:"fieldSpecs,omitempty" yaml:"fieldSpecs,omitempty"`

	// YAMLSupport can be set to true to use the kyaml filter instead of the
	// kunstruct transformer.
	// TODO: change the default to use kyaml when it is stable
	YAMLSupport bool `json:"yamlSupport,omitempty" yaml:"yamlSupport,omitempty"`
}

func (p *NamespaceTransformerPlugin) Config(
	_ *resmap.PluginHelpers, c []byte) (err error) {
	p.Namespace = ""
	p.FieldSpecs = nil
	return yaml.Unmarshal(c, p)
}

func (p *NamespaceTransformerPlugin) Transform(m resmap.ResMap) error {
	if len(p.Namespace) == 0 {
		return nil
	}
	for _, r := range m.Resources() {
		if len(r.Map()) == 0 {
			// Don't mutate empty objects?
			continue
		}

		id := r.OrgId()

		if p.YAMLSupport {
			// use the new style transform
			err := filtersutil.ApplyToJSON(namespace.Filter{
				Namespace: p.Namespace,
				FsSlice:   p.FieldSpecs,
			}, r.Kunstructured)
			if err != nil {
				return err
			}
		} else {
			// use the old style transform
			applicableFs := p.applicableFieldSpecs(id)
			for _, fs := range applicableFs {
				err := transform.MutateField(
					r.Map(), fs.PathSlice(), fs.CreateIfNotPresent,
					p.changeNamespace(r))
				if err != nil {
					return err
				}
			}
		}

		matches := m.GetMatchingResourcesByCurrentId(r.CurId().Equals)
		if len(matches) != 1 {
			return fmt.Errorf(
				"namespace transformation produces ID conflict: %+v", matches)
		}
	}
	return nil
}

// Special casing metadata.namespace since
// all objects have it, even "ClusterKind" objects
// that don't exist in a namespace (the Namespace
// object itself doesn't live in a namespace).
func (p *NamespaceTransformerPlugin) applicableFieldSpecs(id resid.ResId) []types.FieldSpec {
	var res []types.FieldSpec
	for _, fs := range p.FieldSpecs {
		if id.IsSelected(&fs.Gvk) &&
			(fs.Path != types.MetadataNamespacePath ||
				(fs.Path == types.MetadataNamespacePath && id.IsNamespaceableKind())) {
			res = append(res, fs)
		}
	}
	return res
}

func (p *NamespaceTransformerPlugin) changeNamespace(
	_ *resource.Resource) func(in interface{}) (interface{}, error) {
	return func(in interface{}) (interface{}, error) {
		switch in.(type) {
		case string:
			// will happen when the metadata/namespace
			// value is replaced
			return p.Namespace, nil
		case []interface{}:
			l, _ := in.([]interface{})
			for idx, item := range l {
				switch item.(type) {
				case map[string]interface{}:
					// Will happen when mutating the subjects
					// field of ClusterRoleBinding and RoleBinding
					inMap, _ := item.(map[string]interface{})
					if _, ok := inMap["name"]; !ok {
						continue
					}
					name, ok := inMap["name"].(string)
					if !ok {
						continue
					}
					// The only case we need to force the namespace
					// if for the "service account". "default" is
					// kind of hardcoded here for right now.
					if name != "default" {
						continue
					}
					inMap["namespace"] = p.Namespace
					l[idx] = inMap
				default:
					// nothing to do for right now
				}
			}
			return in, nil
		case map[string]interface{}:
			// Will happen if the createField=true
			// when the namespace is added to the
			// object
			inMap := in.(map[string]interface{})
			if len(inMap) == 0 {
				return p.Namespace, nil
			} else {
				return in, nil
			}
		default:
			return in, nil
		}
	}
}

func NewNamespaceTransformerPlugin() resmap.TransformerPlugin {
	return &NamespaceTransformerPlugin{}
}